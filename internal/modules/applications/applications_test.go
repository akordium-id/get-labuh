package applications

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	stdtesting "testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/gitbot"
	"github.com/akordium-id/get-labuh/internal/models"
)

type mockEnvProvider struct{}

func (m *mockEnvProvider) GetEnvironmentByID(id string) (*models.Environment, error) {
	return &models.Environment{ID: id, Name: "Production", Slug: "production"}, nil
}

func (m *mockEnvProvider) GetByID(id string) (*models.Project, error) {
	return &models.Project{ID: id, Name: "Project 1", Slug: "project-1"}, nil
}

type mockProjectEnvProvider struct {
	projects []*models.Project
	envs     []*models.Environment
}

func (m *mockProjectEnvProvider) GetAll() ([]*models.Project, error) {
	return m.projects, nil
}

func (m *mockProjectEnvProvider) Create(input models.CreateProjectInput) (*models.Project, error) {
	p := &models.Project{ID: "proj-1", Name: input.Name, Slug: input.Slug}
	m.projects = append(m.projects, p)
	return p, nil
}

func (m *mockProjectEnvProvider) GetEnvironmentsByProjectID(projectID string) ([]*models.Environment, error) {
	return m.envs, nil
}

func (m *mockProjectEnvProvider) CreateEnvironment(projectID, name, slug string) (*models.Environment, error) {
	e := &models.Environment{ID: "env-1", ProjectID: projectID, Name: name, Slug: slug}
	m.envs = append(m.envs, e)
	return e, nil
}

type mockDeploymentCreator struct {
	created []*models.Deployment
}

func (m *mockDeploymentCreator) CreateWithContext(ctx context.Context, input models.CreateDeploymentInput) (*models.Deployment, error) {
	d := &models.Deployment{
		ID:            "dep-1",
		ApplicationID: input.ApplicationID,
		CommitHash:    input.CommitHash,
		CommitMessage: input.CommitMessage,
		Status:        models.DeployStatusQueued,
	}
	m.created = append(m.created, d)
	return d, nil
}

func (m *mockDeploymentCreator) Create(input models.CreateDeploymentInput) (*models.Deployment, error) {
	return m.CreateWithContext(context.Background(), input)
}

func (m *mockDeploymentCreator) MarkStarted(id string) error {
	return nil
}

func (m *mockDeploymentCreator) UpdateStatus(id string, status models.DeployStatus) error {
	return nil
}

func (m *mockDeploymentCreator) GetByApplicationIDWithContext(ctx context.Context, appID string) ([]*models.Deployment, error) {
	var result []*models.Deployment
	for _, d := range m.created {
		if d.ApplicationID == appID {
			result = append(result, d)
		}
	}
	return result, nil
}

type mockSettingProvider struct{}

func (m *mockSettingProvider) Get(ctx context.Context, key string) (string, error) {
	return "test", nil
}

type mockDeployWorker struct {
	enqueued []models.DeploymentJob
}

func (m *mockDeployWorker) Enqueue(job models.DeploymentJob) error {
	m.enqueued = append(m.enqueued, job)
	return nil
}

func setupTestModule(t *stdtesting.T) (*Module, *mockDeploymentCreator, *mockDeployWorker) {
	t.Helper()
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	t.Cleanup(func() { _ = db.Close() })

	depCreator := &mockDeploymentCreator{}
	workerMock := &mockDeployWorker{}

	mod := New(
		db,
		&mockEnvProvider{},
		&mockProjectEnvProvider{},
		depCreator,
		&mockSettingProvider{},
		nil,
		nil,
	)
	mod.SetDeployWorker(workerMock)

	return mod, depCreator, workerMock
}

func TestApplicationRepo_CRUD(t *stdtesting.T) {
	mod, _, _ := setupTestModule(t)

	// Create
	app, err := mod.Repo.CreateWithContext(context.Background(), models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Web Api",
		Slug:          "web-api",
		SourceType:    models.SourceTypeDockerImage,
		AppPort:       8080,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, app.ID)
	assert.Equal(t, "Web Api", app.Name)
	assert.Equal(t, models.AppStatusIdle, app.Status)

	// Get by ID
	found, err := mod.Repo.GetByIDWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	assert.Equal(t, app.ID, found.ID)

	// Update status
	err = mod.Repo.UpdateStatusWithContext(context.Background(), app.ID, models.AppStatusRunning)
	require.NoError(t, err)

	found, err = mod.Repo.GetByIDWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	assert.Equal(t, models.AppStatusRunning, found.Status)

	// Update limits
	cpu := 1.5
	mem := int64(1024)
	err = mod.Repo.UpdateResourceLimitsWithContext(context.Background(), app.ID, &cpu, &mem, nil)
	require.NoError(t, err)

	found, err = mod.Repo.GetByIDWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	assert.Equal(t, 1.5, *found.CPULimit)
	assert.Equal(t, int64(1024), *found.MemoryLimit)
}

func TestEnvVarRepo_CRUD(t *stdtesting.T) {
	mod, _, _ := setupTestModule(t)

	envVar, err := mod.EnvVarRepo.CreateWithContext(context.Background(), models.CreateAppEnvVarInput{
		ApplicationID: "app-1",
		Key:           "PORT",
		Value:         "8080",
		IsSecret:      false,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, envVar.ID)
	assert.Equal(t, "PORT", envVar.Key)

	vars, err := mod.EnvVarRepo.GetByApplicationIDWithContext(context.Background(), "app-1")
	require.NoError(t, err)
	assert.Len(t, vars, 1)
	assert.Equal(t, "8080", vars[0].Value)

	err = mod.EnvVarRepo.DeleteWithContext(context.Background(), envVar.ID)
	require.NoError(t, err)

	vars, err = mod.EnvVarRepo.GetByApplicationIDWithContext(context.Background(), "app-1")
	require.NoError(t, err)
	assert.Empty(t, vars)
}

func TestEnvVarRepo_UpsertBulk(t *stdtesting.T) {
	mod, _, _ := setupTestModule(t)
	ctx := context.Background()

	app, err := mod.Repo.CreateWithContext(ctx, models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Bulk Env App",
		Slug:          "bulk-env-app",
		SourceType:    models.SourceTypeDockerImage,
	})
	require.NoError(t, err)

	// Seed existing vars
	_, err = mod.EnvVarRepo.CreateWithContext(ctx, models.CreateAppEnvVarInput{
		ApplicationID: app.ID,
		Key:           "INITIAL_KEY",
		Value:         "val1",
	})
	require.NoError(t, err)

	_, err = mod.EnvVarRepo.CreateWithContext(ctx, models.CreateAppEnvVarInput{
		ApplicationID: app.ID,
		Key:           "TO_OVERWRITE",
		Value:         "old_val",
	})
	require.NoError(t, err)

	// Perform Bulk Upsert
	bulkInputs := []models.CreateAppEnvVarInput{
		{Key: "TO_OVERWRITE", Value: "new_val", IsSecret: false},
		{Key: "NEW_KEY", Value: "val2", IsSecret: true},
	}

	count, err := mod.EnvVarRepo.UpsertBulkWithContext(ctx, app.ID, bulkInputs)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	vars, err := mod.EnvVarRepo.GetByApplicationIDWithContext(ctx, app.ID)
	require.NoError(t, err)
	assert.Len(t, vars, 3)

	varMap := make(map[string]*models.AppEnvVar)
	for _, v := range vars {
		varMap[v.Key] = v
	}

	assert.Equal(t, "val1", varMap["INITIAL_KEY"].Value)
	assert.Equal(t, "new_val", varMap["TO_OVERWRITE"].Value)
	assert.Equal(t, "val2", varMap["NEW_KEY"].Value)
	assert.True(t, varMap["NEW_KEY"].IsSecret)
}

func TestApplicationsHandler_ResourceLimits(t *stdtesting.T) {
	mod, _, _ := setupTestModule(t)

	app, err := mod.Repo.CreateWithContext(context.Background(), models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Backend Service",
		Slug:          "backend-service",
		SourceType:    models.SourceTypeDockerImage,
	})
	require.NoError(t, err)

	form := url.Values{}
	form.Set("cpu_limit", "2.0")
	form.Set("memory_limit", "2048")
	form.Set("memory_reservation", "1024")

	req := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/resources", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", app.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	mod.Handler.UpdateResourceLimits(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/applications/"+app.ID, rec.Header().Get("Location"))

	updated, err := mod.Repo.GetByIDWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.CPULimit)
	assert.Equal(t, 2.0, *updated.CPULimit)
	require.NotNil(t, updated.MemoryLimit)
	assert.Equal(t, int64(2048), *updated.MemoryLimit)
}

func TestApplicationsHandler_Deploy(t *stdtesting.T) {
	mod, depCreator, workerMock := setupTestModule(t)

	app, err := mod.Repo.CreateWithContext(context.Background(), models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Worker App",
		Slug:          "worker-app",
		SourceType:    models.SourceTypeDockerImage,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/deploy", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", app.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	mod.Handler.Deploy(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/deployments/dep-1", rec.Header().Get("HX-Redirect"))
	assert.Len(t, depCreator.created, 1)
	assert.Len(t, workerMock.enqueued, 1)
}

func TestDetectAppStack(t *stdtesting.T) {
	tmpDir := t.TempDir()
	stack, port := DetectAppStack(tmpDir)
	assert.Equal(t, StackGeneric, stack)
	assert.Equal(t, 8080, port)
}

func TestApplicationsHandler_UpdateHealthcheck(t *stdtesting.T) {
	mod, _, _ := setupTestModule(t)

	app, err := mod.Repo.CreateWithContext(context.Background(), models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Healthcheck App",
		Slug:          "healthcheck-app",
		SourceType:    models.SourceTypeDockerImage,
	})
	require.NoError(t, err)

	form := url.Values{}
	form.Set("healthcheck_path", "/api/v1/healthz")
	form.Set("healthcheck_timeout", "45")

	req := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/healthcheck", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", app.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	mod.Handler.UpdateHealthcheck(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/applications/"+app.ID, rec.Header().Get("Location"))

	updated, err := mod.Repo.GetByIDWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.HealthcheckPath)
	assert.Equal(t, "/api/v1/healthz", *updated.HealthcheckPath)
	require.NotNil(t, updated.HealthcheckTimeout)
	assert.Equal(t, 45, *updated.HealthcheckTimeout)
}

func TestApplicationsHandler_UpdateBasicAuth(t *stdtesting.T) {
	mod, _, _ := setupTestModule(t)

	app, err := mod.Repo.CreateWithContext(context.Background(), models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Secure App",
		Slug:          "secure-app",
		SourceType:    models.SourceTypeDockerImage,
	})
	require.NoError(t, err)

	form := url.Values{}
	form.Set("basic_auth_user", "superadmin")
	form.Set("basic_auth_password", "strongpassword123")

	req := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/basic-auth", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", app.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	mod.Handler.UpdateBasicAuth(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/applications/"+app.ID, rec.Header().Get("Location"))

	updated, err := mod.Repo.GetByIDWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.BasicAuthUser)
	assert.Equal(t, "superadmin", *updated.BasicAuthUser)
	require.NotNil(t, updated.BasicAuthPassword)
	assert.Equal(t, "strongpassword123", *updated.BasicAuthPassword)
}

func TestApplications_CronJobs(t *stdtesting.T) {
	mod, _, _ := setupTestModule(t)

	app, err := mod.Repo.CreateWithContext(context.Background(), models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Cron App",
		Slug:          "cron-app",
		SourceType:    models.SourceTypeDockerImage,
	})
	require.NoError(t, err)

	// Test 1: Create Cron Job via Repo
	job, err := mod.CronRepo.CreateWithContext(context.Background(), models.CreateCronJobInput{
		ApplicationID:  app.ID,
		Name:           "Laravel Schedule",
		Command:        "php artisan schedule:run",
		CronExpression: "* * * * *",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, job.ID)
	assert.Equal(t, "Laravel Schedule", job.Name)
	assert.True(t, job.IsEnabled)

	// Test 2: List by App ID
	jobs, err := mod.CronRepo.GetByApplicationIDWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	require.Len(t, jobs, 1)

	// Test 3: Toggle Enabled
	newStatus, err := mod.CronRepo.ToggleEnabledWithContext(context.Background(), job.ID)
	require.NoError(t, err)
	assert.False(t, newStatus)

	activeJobs, err := mod.CronRepo.ListActiveJobsWithContext(context.Background())
	require.NoError(t, err)
	assert.Empty(t, activeJobs)

	// Test 4: Update Execution
	err = mod.CronRepo.UpdateExecutionWithContext(context.Background(), job.ID, models.CronJobStatusSuccess, "Schedule run successfully")
	require.NoError(t, err)

	updatedJob, err := mod.CronRepo.GetByIDWithContext(context.Background(), job.ID)
	require.NoError(t, err)
	require.NotNil(t, updatedJob.LastStatus)
	assert.Equal(t, models.CronJobStatusSuccess, *updatedJob.LastStatus)
	require.NotNil(t, updatedJob.LastOutput)
	assert.Equal(t, "Schedule run successfully", *updatedJob.LastOutput)

	// Test 5: HTTP CreateCronJob Handler
	form := url.Values{}
	form.Set("name", "Cleanup Logs")
	form.Set("command", "rm -rf /tmp/logs/*")
	form.Set("cron_expression", "0 2 * * *")

	req := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/cron-jobs", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", app.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	mod.Handler.CreateCronJob(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/applications/"+app.ID, rec.Header().Get("HX-Redirect"))

	jobsAfterHTTP, err := mod.CronRepo.GetByApplicationIDWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	assert.Len(t, jobsAfterHTTP, 2)

	// Test 6: Delete Cron Job
	err = mod.CronRepo.DeleteWithContext(context.Background(), job.ID)
	require.NoError(t, err)

	jobsAfterDelete, err := mod.CronRepo.GetByApplicationIDWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	assert.Len(t, jobsAfterDelete, 1)
}

func TestApplications_PRPreviews(t *stdtesting.T) {
	mod, _, _ := setupTestModule(t)

	// Buat parent app dengan PR previews enabled
	tmplDomain := "{{app}}-pr-{{pr}}.preview.test"
	app, err := mod.Repo.CreateWithContext(context.Background(), models.CreateApplicationInput{
		EnvironmentID:         "env-1",
		Name:                  "Core API",
		Slug:                  "core-api",
		SourceType:            models.SourceTypeGit,
		PRPreviewsEnabled:     true,
		PreviewTemplateDomain: &tmplDomain,
	})
	require.NoError(t, err)

	// Tambahkan env var di parent
	_, err = mod.EnvVarRepo.CreateWithContext(context.Background(), models.CreateAppEnvVarInput{
		ApplicationID: app.ID,
		Key:           "APP_KEY",
		Value:         "base64:secretkey123",
	})
	require.NoError(t, err)

	// Helper untuk HMAC GitHub signature
	signGH := func(body string) string {
		mac := hmac.New(sha256.New, []byte(*app.WebhookSecret))
		mac.Write([]byte(body))
		return "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}

	// Buat parent volume dan file mount untuk verifikasi kloning ke preview app
	_, err = mod.Repo.CreateVolume(context.Background(), models.CreateAppVolumeInput{
		ApplicationID: app.ID,
		Name:          "uploads",
		VolumeType:    models.VolumeTypeNamed,
		MountPath:     "/app/uploads",
	})
	require.NoError(t, err)

	_, err = mod.Repo.CreateFileMount(context.Background(), models.CreateAppFileMountInput{
		ApplicationID: app.ID,
		MountPath:     "/app/config.json",
		Content:       `{"env":"prod"}`,
		FileMode:      "0644",
	})
	require.NoError(t, err)

	// 1. Test Webhook GitHub: Pull Request Opened -> Spawn Preview App
	ghPayload := `{
		"action": "opened",
		"number": 42,
		"pull_request": {
			"title": "feat: add payment gateway",
			"head": {
				"ref": "feature/payments",
				"sha": "abc123commit"
			}
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github/"+app.ID, strings.NewReader(ghPayload))
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-Hub-Signature-256", signGH(ghPayload))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("application_id", app.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	mod.Webhooks.GitHub(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)

	// Verifikasi Preview App terbentuk di repo
	previews, err := mod.Repo.ListPreviewsWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	require.Len(t, previews, 1)
	preview := previews[0]

	assert.Equal(t, "Core API (PR #42)", preview.Name)
	assert.Equal(t, "core-api-pr-42", preview.Slug)
	assert.True(t, preview.IsPreview)
	require.NotNil(t, preview.ParentAppID)
	assert.Equal(t, app.ID, *preview.ParentAppID)
	require.NotNil(t, preview.PRNumber)
	assert.Equal(t, 42, *preview.PRNumber)
	require.NotNil(t, preview.Branch)
	assert.Equal(t, "feature/payments", *preview.Branch)
	require.NotNil(t, preview.CustomDomain)
	assert.Equal(t, "core-api-pr-42.preview.test", *preview.CustomDomain)

	// Verifikasi env vars ter-clone ke preview app
	previewVars, err := mod.EnvVarRepo.GetByApplicationIDWithContext(context.Background(), preview.ID)
	require.NoError(t, err)
	require.Len(t, previewVars, 1)
	assert.Equal(t, "APP_KEY", previewVars[0].Key)
	assert.Equal(t, "base64:secretkey123", previewVars[0].Value)

	// Verifikasi persistent volumes & file mounts ter-clone ke preview app
	previewVols, err := mod.Repo.GetVolumesByAppID(context.Background(), preview.ID)
	require.NoError(t, err)
	require.Len(t, previewVols, 1)
	assert.Equal(t, "uploads", previewVols[0].Name)
	assert.Equal(t, "/app/uploads", previewVols[0].MountPath)

	previewFiles, err := mod.Repo.GetFileMountsByAppID(context.Background(), preview.ID)
	require.NoError(t, err)
	require.Len(t, previewFiles, 1)
	assert.Equal(t, "/app/config.json", previewFiles[0].MountPath)
	assert.Equal(t, `{"env":"prod"}`, previewFiles[0].Content)

	// 2. Test Webhook GitHub: Pull Request Closed -> Teardown Preview App
	ghClosePayload := `{
		"action": "closed",
		"number": 42,
		"pull_request": {
			"title": "feat: add payment gateway",
			"head": {
				"ref": "feature/payments",
				"sha": "abc123commit"
			}
		}
	}`
	reqClose := httptest.NewRequest(http.MethodPost, "/webhooks/github/"+app.ID, strings.NewReader(ghClosePayload))
	reqClose.Header.Set("X-GitHub-Event", "pull_request")
	reqClose.Header.Set("X-Hub-Signature-256", signGH(ghClosePayload))
	reqClose.Header.Set("Content-Type", "application/json")
	rctxClose := chi.NewRouteContext()
	rctxClose.URLParams.Add("application_id", app.ID)
	reqClose = reqClose.WithContext(context.WithValue(reqClose.Context(), chi.RouteCtxKey, rctxClose))

	recClose := httptest.NewRecorder()
	mod.Webhooks.GitHub(recClose, reqClose)
	assert.Equal(t, http.StatusOK, recClose.Code)

	// Verifikasi Preview App terhapus
	previewsAfterClose, err := mod.Repo.ListPreviewsWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	assert.Empty(t, previewsAfterClose)

	// 3. Test Webhook GitLab: Merge Request Hook -> Spawn & Teardown
	glPayload := `{
		"object_kind": "merge_request",
		"object_attributes": {
			"iid": 99,
			"title": "fix: resolve login bug",
			"source_branch": "fix/login-bug",
			"action": "open",
			"last_commit": {
				"id": "gitlabcommit123",
				"message": "fix: null check"
			}
		}
	}`
	reqGL := httptest.NewRequest(http.MethodPost, "/webhooks/gitlab/"+app.ID, strings.NewReader(glPayload))
	reqGL.Header.Set("X-Gitlab-Event", "Merge Request Hook")
	reqGL.Header.Set("X-Gitlab-Token", *app.WebhookSecret)
	reqGL.Header.Set("Content-Type", "application/json")
	rctxGL := chi.NewRouteContext()
	rctxGL.URLParams.Add("application_id", app.ID)
	reqGL = reqGL.WithContext(context.WithValue(reqGL.Context(), chi.RouteCtxKey, rctxGL))

	recGL := httptest.NewRecorder()
	mod.Webhooks.GitLab(recGL, reqGL)
	assert.Equal(t, http.StatusOK, recGL.Code)

	glPreviews, err := mod.Repo.ListPreviewsWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	require.Len(t, glPreviews, 1)
	assert.Equal(t, 99, *glPreviews[0].PRNumber)
	assert.Equal(t, "fix/login-bug", *glPreviews[0].Branch)

	// 4. Test HTTP Handler: Manual Teardown
	teardownReq := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/previews/"+glPreviews[0].ID+"/teardown", nil)
	rctxTeardown := chi.NewRouteContext()
	rctxTeardown.URLParams.Add("id", app.ID)
	rctxTeardown.URLParams.Add("preview_id", glPreviews[0].ID)
	teardownReq = teardownReq.WithContext(context.WithValue(teardownReq.Context(), chi.RouteCtxKey, rctxTeardown))

	recTeardown := httptest.NewRecorder()
	mod.Handler.TeardownPRPreview(recTeardown, teardownReq)
	assert.Equal(t, http.StatusOK, recTeardown.Code)

	glPreviewsAfterManual, err := mod.Repo.ListPreviewsWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	assert.Empty(t, glPreviewsAfterManual)
}

func TestApplications_PRPreviews_GitBot(t *stdtesting.T) {
	mod, _, _ := setupTestModule(t)

	var capturedComments []string
	gitServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer app_secret_git_token", r.Header.Get("Authorization"))
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/comments") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]any{})
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/comments") {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			capturedComments = append(capturedComments, body["body"])
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": 1, "body": body["body"]})
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer gitServer.Close()

	// Inject custom gitbot client to webhooks handler
	mod.Webhooks.SetGitBotClient(gitbot.NewClientWithBaseURLs(gitServer.URL, gitServer.URL))

	repoURL := "https://github.com/myorg/myapp.git"
	appToken := "app_secret_git_token"
	tmplDomain := "{{app}}-pr-{{pr}}.preview.test"
	app, err := mod.Repo.CreateWithContext(context.Background(), models.CreateApplicationInput{
		EnvironmentID:         "env-1",
		Name:                  "Bot App",
		Slug:                  "bot-app",
		SourceType:            models.SourceTypeGit,
		RepositoryURL:         &repoURL,
		PRPreviewsEnabled:     true,
		PreviewTemplateDomain: &tmplDomain,
		GitToken:              &appToken,
	})
	require.NoError(t, err)

	signGH := func(body string) string {
		mac := hmac.New(sha256.New, []byte(*app.WebhookSecret))
		mac.Write([]byte(body))
		return "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}

	// 1. Pull request opened -> must dispatch "Building & Deploying..." comment
	ghOpenPayload := `{
		"action": "opened",
		"number": 77,
		"pull_request": {
			"title": "feat: test gitbot comment",
			"head": {
				"ref": "feature/bot",
				"sha": "sha777"
			}
		}
	}`
	reqOpen := httptest.NewRequest(http.MethodPost, "/webhooks/github/"+app.ID, strings.NewReader(ghOpenPayload))
	reqOpen.Header.Set("X-GitHub-Event", "pull_request")
	reqOpen.Header.Set("X-Hub-Signature-256", signGH(ghOpenPayload))
	reqOpen.Header.Set("Content-Type", "application/json")
	rctxOpen := chi.NewRouteContext()
	rctxOpen.URLParams.Add("application_id", app.ID)
	reqOpen = reqOpen.WithContext(context.WithValue(reqOpen.Context(), chi.RouteCtxKey, rctxOpen))

	recOpen := httptest.NewRecorder()
	mod.Webhooks.GitHub(recOpen, reqOpen)
	assert.Equal(t, http.StatusOK, recOpen.Code)

	require.Eventually(t, func() bool {
		return len(capturedComments) == 1
	}, 1*time.Second, 10*time.Millisecond)

	assert.Contains(t, capturedComments[0], gitbot.Marker)
	assert.Contains(t, capturedComments[0], "Building & Deploying...")
}

func TestApplicationLifecycleFields(t *stdtesting.T) {
	mod, _, _ := setupTestModule(t)
	ctx := context.Background()

	preCmd := "php artisan migrate --force"
	postCmd := "php artisan optimize:clear"

	app, err := mod.Repo.CreateWithContext(ctx, models.CreateApplicationInput{
		EnvironmentID:         "env-1",
		Name:                  "Lifecycle App",
		Slug:                  "lifecycle-app",
		SourceType:            models.SourceTypeDockerImage,
		PreDeploymentCommand:  &preCmd,
		PostDeploymentCommand: &postCmd,
	})
	require.NoError(t, err)
	require.NotNil(t, app.PreDeploymentCommand)
	assert.Equal(t, "php artisan migrate --force", *app.PreDeploymentCommand)
	require.NotNil(t, app.PostDeploymentCommand)
	assert.Equal(t, "php artisan optimize:clear", *app.PostDeploymentCommand)
}

func TestUpdateLifecycleCommands(t *stdtesting.T) {
	mod, _, _ := setupTestModule(t)
	ctx := context.Background()

	app, err := mod.Repo.CreateWithContext(ctx, models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Lifecycle App 2",
		Slug:          "lifecycle-app-2",
		SourceType:    models.SourceTypeDockerImage,
	})
	require.NoError(t, err)
	assert.Nil(t, app.PreDeploymentCommand)
	assert.Nil(t, app.PostDeploymentCommand)

	newPre := "npm run prisma:migrate"
	newPost := "npm run cache:warm"
	err = mod.Repo.UpdateLifecycleCommands(ctx, app.ID, &newPre, &newPost)
	require.NoError(t, err)

	updated, err := mod.Repo.GetByIDWithContext(ctx, app.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.PreDeploymentCommand)
	assert.Equal(t, newPre, *updated.PreDeploymentCommand)
	require.NotNil(t, updated.PostDeploymentCommand)
	assert.Equal(t, newPost, *updated.PostDeploymentCommand)

	// Test reset to nil
	err = mod.Repo.UpdateLifecycleCommands(ctx, app.ID, nil, nil)
	require.NoError(t, err)

	updated2, err := mod.Repo.GetByIDWithContext(ctx, app.ID)
	require.NoError(t, err)
	assert.Nil(t, updated2.PreDeploymentCommand)
	assert.Nil(t, updated2.PostDeploymentCommand)
}

func TestApplicationsHandler_UpdateLifecycleCommands(t *stdtesting.T) {
	mod, _, _ := setupTestModule(t)

	app, err := mod.Repo.CreateWithContext(context.Background(), models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Lifecycle App 3",
		Slug:          "lifecycle-app-3",
		SourceType:    models.SourceTypeDockerImage,
	})
	require.NoError(t, err)

	form := url.Values{}
	form.Set("pre_deployment_command", "php artisan migrate --force")
	form.Set("post_deployment_command", "php artisan optimize:clear")

	req := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/lifecycle-commands", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", app.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	mod.Handler.UpdateLifecycleCommands(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/applications/"+app.ID+"#advanced", rec.Header().Get("Location"))

	updated, err := mod.Repo.GetByIDWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.PreDeploymentCommand)
	assert.Equal(t, "php artisan migrate --force", *updated.PreDeploymentCommand)
	require.NotNil(t, updated.PostDeploymentCommand)
	assert.Equal(t, "php artisan optimize:clear", *updated.PostDeploymentCommand)
}

func TestApplicationsHandler_BulkCreateEnvVars(t *stdtesting.T) {
	mod, _, _ := setupTestModule(t)

	app, err := mod.Repo.CreateWithContext(context.Background(), models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Bulk Env App Test",
		Slug:          "bulk-env-app-test",
		SourceType:    models.SourceTypeDockerImage,
	})
	require.NoError(t, err)

	form := url.Values{}
	form.Set("raw_env", "DATABASE_URL=postgres://localhost:5432/mydb\nAPP_DEBUG=true\nAPI_KEY=xyz123")

	req := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/env-vars/bulk", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", app.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	mod.Handler.BulkCreateEnvVars(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/applications/"+app.ID+"#env", rec.Header().Get("Location"))

	vars, err := mod.EnvVarRepo.GetByApplicationIDWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	assert.Len(t, vars, 3)
}

func TestVolumeHandlers(t *stdtesting.T) {
	mod, _, _ := setupTestModule(t)
	r := chi.NewRouter()
	mod.RegisterRoutes(r)

	// Create app first
	app, err := mod.Repo.CreateWithContext(context.Background(), models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "Storage Handler App",
		SourceType:    models.SourceTypeDockerImage,
		AppPort:       8080,
	})
	require.NoError(t, err)

	// 1. Create Volume (Named)
	form := url.Values{}
	form.Set("name", "uploads")
	form.Set("volume_type", "named")
	form.Set("mount_path", "/app/uploads")
	req := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/volumes", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "/applications/"+app.ID+"#storage", rec.Header().Get("HX-Redirect"))

	vols, err := mod.Repo.GetVolumesByAppID(context.Background(), app.ID)
	require.NoError(t, err)
	require.Len(t, vols, 1)

	// 2. Create Volume with Forbidden Host Path -> should fail 400
	formBad := url.Values{}
	formBad.Set("name", "bad-bind")
	formBad.Set("volume_type", "bind")
	formBad.Set("host_path", "/etc/shadow")
	formBad.Set("mount_path", "/app/etc")
	reqBad := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/volumes", strings.NewReader(formBad.Encode()))
	reqBad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recBad := httptest.NewRecorder()
	r.ServeHTTP(recBad, reqBad)
	assert.Equal(t, http.StatusBadRequest, recBad.Code)

	// 3. Create File Mount
	formFM := url.Values{}
	formFM.Set("mount_path", "/etc/nginx/nginx.conf")
	formFM.Set("content", "events {}")
	formFM.Set("file_mode", "0644")
	formFM.Set("is_read_only", "true")
	reqFM := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/file-mounts", strings.NewReader(formFM.Encode()))
	reqFM.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqFM.Header.Set("HX-Request", "true")
	recFM := httptest.NewRecorder()
	r.ServeHTTP(recFM, reqFM)
	assert.Equal(t, http.StatusOK, recFM.Code)

	fms, err := mod.Repo.GetFileMountsByAppID(context.Background(), app.ID)
	require.NoError(t, err)
	require.Len(t, fms, 1)

	// 4. Test Validations: Invalid volume name (spaces, special chars)
	formBadName := url.Values{}
	formBadName.Set("name", "my volume!")
	formBadName.Set("volume_type", "named")
	formBadName.Set("mount_path", "/app/my-vol")
	reqBadName := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/volumes", strings.NewReader(formBadName.Encode()))
	reqBadName.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recBadName := httptest.NewRecorder()
	r.ServeHTTP(recBadName, reqBadName)
	assert.Equal(t, http.StatusBadRequest, recBadName.Code)

	// 5. Test Validations: Invalid volume type
	formBadType := url.Values{}
	formBadType.Set("name", "nfs-vol")
	formBadType.Set("volume_type", "invalid-type")
	formBadType.Set("mount_path", "/app/nfs")
	reqBadType := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/volumes", strings.NewReader(formBadType.Encode()))
	reqBadType.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recBadType := httptest.NewRecorder()
	r.ServeHTTP(recBadType, reqBadType)
	assert.Equal(t, http.StatusBadRequest, recBadType.Code)

	// 6. Test Validations: Container root forbidden
	formRoot := url.Values{}
	formRoot.Set("name", "root-mount")
	formRoot.Set("volume_type", "named")
	formRoot.Set("mount_path", "/")
	reqRoot := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/volumes", strings.NewReader(formRoot.Encode()))
	reqRoot.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recRoot := httptest.NewRecorder()
	r.ServeHTTP(recRoot, reqRoot)
	assert.Equal(t, http.StatusBadRequest, recRoot.Code)

	// 7. Test Collision: Duplicate volume name
	formDupName := url.Values{}
	formDupName.Set("name", "uploads")
	formDupName.Set("volume_type", "named")
	formDupName.Set("mount_path", "/app/uploads-new")
	reqDupName := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/volumes", strings.NewReader(formDupName.Encode()))
	reqDupName.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recDupName := httptest.NewRecorder()
	r.ServeHTTP(recDupName, reqDupName)
	assert.Equal(t, http.StatusBadRequest, recDupName.Code)

	// 8. Test Collision: Duplicate volume mount path
	formDupMount := url.Values{}
	formDupMount.Set("name", "uploads-2")
	formDupMount.Set("volume_type", "named")
	formDupMount.Set("mount_path", "/app/uploads")
	reqDupMount := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/volumes", strings.NewReader(formDupMount.Encode()))
	reqDupMount.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recDupMount := httptest.NewRecorder()
	r.ServeHTTP(recDupMount, reqDupMount)
	assert.Equal(t, http.StatusBadRequest, recDupMount.Code)

	// 9. Test Collision: Volume mount path collides with existing File Mount
	formVolCollideFM := url.Values{}
	formVolCollideFM.Set("name", "nginx-vol")
	formVolCollideFM.Set("volume_type", "named")
	formVolCollideFM.Set("mount_path", "/etc/nginx/nginx.conf")
	reqVolCollideFM := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/volumes", strings.NewReader(formVolCollideFM.Encode()))
	reqVolCollideFM.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recVolCollideFM := httptest.NewRecorder()
	r.ServeHTTP(recVolCollideFM, reqVolCollideFM)
	assert.Equal(t, http.StatusBadRequest, recVolCollideFM.Code)

	// 10. Test Collision: File mount collides with existing Volume
	formFMCollideVol := url.Values{}
	formFMCollideVol.Set("mount_path", "/app/uploads")
	formFMCollideVol.Set("content", "conflict")
	reqFMCollideVol := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/file-mounts", strings.NewReader(formFMCollideVol.Encode()))
	reqFMCollideVol.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recFMCollideVol := httptest.NewRecorder()
	r.ServeHTTP(recFMCollideVol, reqFMCollideVol)
	assert.Equal(t, http.StatusBadRequest, recFMCollideVol.Code)

	// 11. Test Collision: File mount duplicate mount path
	formFMDup := url.Values{}
	formFMDup.Set("mount_path", "/etc/nginx/nginx.conf")
	formFMDup.Set("content", "conflict2")
	reqFMDup := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/file-mounts", strings.NewReader(formFMDup.Encode()))
	reqFMDup.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recFMDup := httptest.NewRecorder()
	r.ServeHTTP(recFMDup, reqFMDup)
	assert.Equal(t, http.StatusBadRequest, recFMDup.Code)
}

type mockVolumeBackupRunner struct {
	calledBackup  bool
	calledRestore bool
	backupID      string
	appID         string
}

func (m *mockVolumeBackupRunner) RunBackup(ctx context.Context, appID string) (*models.AppBackup, error) {
	m.calledBackup = true
	m.appID = appID
	return &models.AppBackup{
		ID:            "mock-b1",
		ApplicationID: appID,
		Filename:      "app-volume-test.tar.gz",
		Status:        "completed",
	}, nil
}

func (m *mockVolumeBackupRunner) RestoreAppVolume(ctx context.Context, backupID string) error {
	m.calledRestore = true
	m.backupID = backupID
	return nil
}

func TestAppVolumeBackupHandlers(t *stdtesting.T) {
	mod, _, _ := setupTestModule(t)
	r := chi.NewRouter()
	mod.RegisterRoutes(r)

	app, err := mod.Repo.CreateWithContext(context.Background(), models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "App Volume Backup Test",
		SourceType:    models.SourceTypeDockerImage,
		AppPort:       8080,
	})
	require.NoError(t, err)

	runner := &mockVolumeBackupRunner{}
	mod.Handler.SetVolumeBackupRunner(runner)

	// 1. Update Backup Config
	formCfg := url.Values{}
	formCfg.Set("enabled", "true")
	formCfg.Set("cron_expression", "0 3 * * *")
	formCfg.Set("retention_count", "7")
	reqCfg := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/backups/config", strings.NewReader(formCfg.Encode()))
	reqCfg.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqCfg.Header.Set("HX-Request", "true")
	recCfg := httptest.NewRecorder()
	r.ServeHTTP(recCfg, reqCfg)
	assert.Equal(t, http.StatusOK, recCfg.Code)

	cfg, err := mod.Repo.GetBackupConfigWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 7, cfg.RetentionCount)

	// 2. Trigger Manual Volume Backup
	reqCreate := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/backups/create", nil)
	reqCreate.Header.Set("HX-Request", "true")
	recCreate := httptest.NewRecorder()
	r.ServeHTTP(recCreate, reqCreate)
	assert.Equal(t, http.StatusOK, recCreate.Code)
	assert.True(t, runner.calledBackup)
	assert.Equal(t, app.ID, runner.appID)

	// 3. Create dummy backup record for restore & delete test
	bRecord, err := mod.Repo.CreateBackupWithContext(context.Background(), models.CreateAppBackupInput{
		ApplicationID: app.ID,
		Filename:      "test-snap.tar.gz",
		Status:        "completed",
	})
	require.NoError(t, err)

	// 4. Restore Volume Backup
	reqRestore := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/backups/"+bRecord.ID+"/restore", nil)
	reqRestore.Header.Set("HX-Request", "true")
	recRestore := httptest.NewRecorder()
	r.ServeHTTP(recRestore, reqRestore)
	assert.Equal(t, http.StatusOK, recRestore.Code)
	assert.True(t, runner.calledRestore)
	assert.Equal(t, bRecord.ID, runner.backupID)

	// 5. Delete Volume Backup
	reqDelete := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/backups/"+bRecord.ID+"/delete", nil)
	reqDelete.Header.Set("HX-Request", "true")
	recDelete := httptest.NewRecorder()
	r.ServeHTTP(recDelete, reqDelete)
	assert.Equal(t, http.StatusOK, recDelete.Code)

	_, err = mod.Repo.GetBackupByIDWithContext(context.Background(), bRecord.ID)
	assert.Error(t, err)
}

