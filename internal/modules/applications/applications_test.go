package applications

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	stdtesting "testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
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
