package applications_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	stdtesting "testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/modules/applications"
)

type mockDeployEnqueuer struct {
	enqueuedJobs []models.DeploymentJob
}

func (m *mockDeployEnqueuer) Enqueue(job models.DeploymentJob) error {
	m.enqueuedJobs = append(m.enqueuedJobs, job)
	return nil
}

type mockDeploymentCreator struct {
	created []models.CreateDeploymentInput
}

func (m *mockDeploymentCreator) Create(input models.CreateDeploymentInput) (*models.Deployment, error) {
	return m.CreateWithContext(context.Background(), input)
}

func (m *mockDeploymentCreator) CreateWithContext(ctx context.Context, input models.CreateDeploymentInput) (*models.Deployment, error) {
	m.created = append(m.created, input)
	return &models.Deployment{
		ID:            "dep-test-123",
		ApplicationID: input.ApplicationID,
		CommitHash:    input.CommitHash,
		CommitMessage: input.CommitMessage,
		Status:        models.DeployStatusQueued,
	}, nil
}

func (m *mockDeploymentCreator) GetByApplicationIDWithContext(ctx context.Context, appID string) ([]*models.Deployment, error) {
	return nil, nil
}

func (m *mockDeploymentCreator) MarkStarted(id string) error {
	return nil
}

func (m *mockDeploymentCreator) UpdateStatus(id string, status models.DeployStatus) error {
	return nil
}

func TestUniversalWebhook(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, database.RunAllMigrations(db))

	appRepo := applications.NewRepo(db)
	envVarRepo := applications.NewEnvVarRepo(db)
	mockDeploy := &mockDeploymentCreator{}
	mockWorker := &mockDeployEnqueuer{}

	handler := applications.NewWebhooksHandler(appRepo, mockDeploy, envVarRepo)
	handler.SetDeployWorker(mockWorker)

	router := chi.NewRouter()
	router.Post("/api/webhooks/deploy/{application_id}", handler.Deploy)

	ctx := context.Background()

	// 1. Setup Docker Image app
	initialImg := "myorg/myapp:v1.0.0"
	app, err := appRepo.CreateWithContext(ctx, models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Docker App",
		Slug:          "docker-app",
		SourceType:    models.SourceTypeDockerImage,
		DockerImage:   &initialImg,
	})
	require.NoError(t, err)

	// 2. Send webhook with "commit" and "image_tag"
	payload := map[string]string{
		"branch":         "main",
		"commit":         "fedcba987654",
		"commit_message": "Release v1.1.0",
		"image_tag":      "myorg/myapp:v1.1.0",
	}
	payloadBytes, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/webhooks/deploy/"+app.ID, bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	if app.WebhookSecret != nil {
		req.Header.Set("Authorization", "Bearer "+*app.WebhookSecret)
	}
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify application's docker image was updated in database
	updatedApp, err := appRepo.GetByIDWithContext(ctx, app.ID)
	require.NoError(t, err)
	require.NotNil(t, updatedApp.DockerImage)
	assert.Equal(t, "myorg/myapp:v1.1.0", *updatedApp.DockerImage)

	// Verify deployment created with commit hash
	require.Len(t, mockDeploy.created, 1)
	require.NotNil(t, mockDeploy.created[0].CommitHash)
	assert.Equal(t, "fedcba987654", *mockDeploy.created[0].CommitHash)

	// Verify job enqueued with updated image tag and commit hash
	require.Len(t, mockWorker.enqueuedJobs, 1)
	assert.Equal(t, "myorg/myapp:v1.1.0", mockWorker.enqueuedJobs[0].DockerImage)
	assert.Equal(t, "fedcba987654", mockWorker.enqueuedJobs[0].CommitHash)

	// 3. Test branch mismatch
	mismatchPayload := map[string]string{
		"branch": "develop",
	}
	mismatchBytes, _ := json.Marshal(mismatchPayload)
	req2 := httptest.NewRequest(http.MethodPost, "/api/webhooks/deploy/"+app.ID, bytes.NewReader(mismatchBytes))
	req2.Header.Set("Content-Type", "application/json")
	if app.WebhookSecret != nil {
		req2.Header.Set("Authorization", "Bearer "+*app.WebhookSecret)
	}
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Contains(t, w2.Body.String(), "branch mismatch")

	// 4. Test missing authorization
	req3 := httptest.NewRequest(http.MethodPost, "/api/webhooks/deploy/"+app.ID, bytes.NewReader(payloadBytes))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusUnauthorized, w3.Code)
}
