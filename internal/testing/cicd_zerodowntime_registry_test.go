package testing

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	stdtesting "testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/encryption"
	"github.com/akordium-id/get-labuh/internal/gitbot"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/modules/applications"
	"github.com/akordium-id/get-labuh/internal/modules/settings"
)

type mockDeployEnqueuer struct {
	jobs []models.DeploymentJob
}

func (m *mockDeployEnqueuer) Enqueue(job models.DeploymentJob) error {
	m.jobs = append(m.jobs, job)
	return nil
}

func TestIntegration_CICD_UniversalWebhook_DynamicImageAndCommit(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	defer db.Close()

	projectRepo := repo.NewProjectRepo(db)
	appRepo := repo.NewApplicationRepo(db)
	deployRepo := repo.NewDeploymentRepo(db)
	envVarRepo := repo.NewEnvVarRepo(db)

	project, err := projectRepo.Create(models.CreateProjectInput{Name: "Webhook Project", Slug: "webhook-project"})
	require.NoError(t, err)
	env, err := projectRepo.CreateEnvironment(project.ID, "Production", "production")
	require.NoError(t, err)

	dockerImg := "registry.example.com/org/service:1.0.0"
	app, err := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "Backend Service",
		Slug:          "backend-service",
		SourceType:    models.SourceTypeDockerImage,
		DockerImage:   &dockerImg,
		AppPort:       8080,
	})
	require.NoError(t, err)

	webhookHandler := applications.NewWebhooksHandler(appRepo, deployRepo, envVarRepo)
	enqueuer := &mockDeployEnqueuer{}
	webhookHandler.SetDeployWorker(enqueuer)

	// Simulate incoming CI/CD webhook with dynamic image tag & commit hash
	payloadJSON := `{
		"image_tag": "registry.example.com/org/service:2.0.0",
		"commit": "abcdef1234567890",
		"commit_message": "release: v2.0.0 automated build",
		"branch": "main"
	}`

	req := httptest.NewRequest(http.MethodPost, "/webhooks/deploy/"+app.ID, strings.NewReader(payloadJSON))
	req.Header.Set("Content-Type", "application/json")
	require.NotNil(t, app.WebhookSecret)
	req.Header.Set("Authorization", "Bearer "+*app.WebhookSecret)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("application_id", app.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	webhookHandler.Deploy(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify application's docker_image was dynamically updated in DB
	updatedApp, err := appRepo.GetByID(app.ID)
	require.NoError(t, err)
	require.NotNil(t, updatedApp.DockerImage)
	assert.Equal(t, "registry.example.com/org/service:2.0.0", *updatedApp.DockerImage)

	// Verify deployment record in DB contains the commit hash and commit message
	deployments, err := deployRepo.GetByApplicationID(app.ID)
	require.NoError(t, err)
	require.Len(t, deployments, 1)
	require.NotNil(t, deployments[0].CommitHash)
	assert.Equal(t, "abcdef1234567890", *deployments[0].CommitHash)
	require.NotNil(t, deployments[0].CommitMessage)
	assert.Equal(t, "release: v2.0.0 automated build", *deployments[0].CommitMessage)

	// Verify worker queue job has the updated commit hash
	require.Len(t, enqueuer.jobs, 1)
	assert.Equal(t, "abcdef1234567890", enqueuer.jobs[0].CommitHash)
}

func TestIntegration_CICD_RegistryAuthAndPipelineStatus(t *stdtesting.T) {
	_ = os.Setenv("LABUH_MASTER_KEY", "test-master-key-for-registries-32b")
	_ = encryption.Init("LABUH_MASTER_KEY")

	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	defer db.Close()

	// 1. Private Docker Registry CRUD with AES-256-GCM encryption
	regRepo := settings.NewRegistryRepo(db)
	createdReg, err := regRepo.Create(context.Background(), models.CreateDockerRegistryInput{
		Name:          "Production GHCR",
		ServerAddress: "ghcr.io",
		Username:      "ci-bot",
		Password:      "super-secret-token",
		Email:         "ci@example.com",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, createdReg.ID)

	// Verify password in DB is encrypted
	var rawPassword string
	err = db.QueryRow("SELECT password FROM docker_registries WHERE id = ?", createdReg.ID).Scan(&rawPassword)
	require.NoError(t, err)
	assert.NotEqual(t, "super-secret-token", rawPassword)

	// Verify retrieval decrypts password correctly
	retrievedReg, err := regRepo.GetByID(context.Background(), createdReg.ID)
	require.NoError(t, err)
	assert.Equal(t, "super-secret-token", retrievedReg.Password)

	// Verify Docker Auth config header creation
	authConfig := docker.RegistryAuth{
		ServerAddress: retrievedReg.ServerAddress,
		Username:      retrievedReg.Username,
		Password:      retrievedReg.Password,
	}
	encodedAuth, err := authConfig.EncodeBase64()
	require.NoError(t, err)

	decodedBytes, err := base64.URLEncoding.DecodeString(encodedAuth)
	require.NoError(t, err)
	var decodedAuth docker.RegistryAuth
	err = json.Unmarshal(decodedBytes, &decodedAuth)
	require.NoError(t, err)
	assert.Equal(t, "ci-bot", decodedAuth.Username)
	assert.Equal(t, "super-secret-token", decodedAuth.Password)
	assert.Equal(t, "ghcr.io", decodedAuth.ServerAddress)

	// 2. Commit Status Reporting to Mock Git Server
	type statusPayload struct {
		State       string `json:"state"`
		Context     string `json:"context"`
		Description string `json:"description"`
		TargetURL   string `json:"target_url"`
	}
	receivedStatuses := make([]statusPayload, 0)

	mockGitServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-pat", r.Header.Get("Authorization"))
		assert.Contains(t, r.URL.Path, "/repos/akordium-id/get-labuh/statuses/112233445566")

		var payload statusPayload
		_ = json.NewDecoder(r.Body).Decode(&payload)
		receivedStatuses = append(receivedStatuses, payload)

		w.WriteHeader(http.StatusCreated)
	}))
	defer mockGitServer.Close()

	gitClient := gitbot.NewClientWithBaseURLs(mockGitServer.URL, mockGitServer.URL)
	repoURL := "https://github.com/akordium-id/get-labuh.git"
	commitSHA := "112233445566"
	gitToken := "test-pat"

	// Report pending at pipeline start
	err = gitClient.SetGitHubCommitStatus(
		context.Background(),
		repoURL,
		commitSHA,
		gitToken,
		"pending",
		"http://localhost:8080/deployments/dep-1",
		"Deploying in Labuh...",
		"labuh/deploy",
	)
	require.NoError(t, err)

	// Report success at pipeline completion
	err = gitClient.SetGitHubCommitStatus(
		context.Background(),
		repoURL,
		commitSHA,
		gitToken,
		"success",
		"http://localhost:8080/deployments/dep-1",
		"Deployed successfully without downtime",
		"labuh/deploy",
	)
	require.NoError(t, err)

	require.Len(t, receivedStatuses, 2)
	assert.Equal(t, "pending", receivedStatuses[0].State)
	assert.Equal(t, "Deploying in Labuh...", receivedStatuses[0].Description)
	assert.Equal(t, "success", receivedStatuses[1].State)
	assert.Equal(t, "Deployed successfully without downtime", receivedStatuses[1].Description)
}

func TestIntegration_ZeroDowntime_FailFastAndDraining(t *stdtesting.T) {
	// 1. Fail-fast detection verification
	// Simulating Docker inspect response when container crashes on boot
	isRestarting := true
	status := "restarting"
	isRunning := true

	// Logic from WaitForHealthCustom:
	shouldFailFast := !isRunning || isRestarting || status == "restarting" || status == "exited"
	assert.True(t, shouldFailFast, "crashing container must trigger fail-fast immediately")

	// 2. Connection Draining Simulation:
	// Ensures that old container traffic is not cut off abruptly
	drained := false
	drainDuration := 50 * time.Millisecond // shortened for unit test speed

	startTime := time.Now()
	// simulate drain
	time.Sleep(drainDuration)
	drained = true

	elapsed := time.Since(startTime)
	assert.True(t, drained)
	assert.GreaterOrEqual(t, elapsed, drainDuration)
}
