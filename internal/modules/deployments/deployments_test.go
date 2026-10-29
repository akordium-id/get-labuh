package deployments

import (
	"context"
	"net/http"
	"net/http/httptest"
	stdtesting "testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/models"
)

type mockAppProvider struct {
	apps map[string]*models.Application
}

func (m *mockAppProvider) GetByID(id string) (*models.Application, error) {
	if app, ok := m.apps[id]; ok {
		return app, nil
	}
	return nil, sqlErrNotFound{}
}

func (m *mockAppProvider) UpdateStatus(id string, status models.AppStatus) error {
	if app, ok := m.apps[id]; ok {
		app.Status = status
	}
	return nil
}

type sqlErrNotFound struct{}

func (e sqlErrNotFound) Error() string { return "not found" }

func setupTestModule(t *stdtesting.T) (*Module, *Repo, *Handler) {
	t.Helper()
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	t.Cleanup(func() { _ = db.Close() })

	mockApp := &mockAppProvider{
		apps: map[string]*models.Application{
			"app-1": {ID: "app-1", Name: "App 1", Slug: "app-1"},
		},
	}

	mod := New(db, mockApp)
	return mod, mod.Repo, mod.Handler
}

func ptrString(s string) *string {
	return &s
}

func TestDeploymentRepo_CRUD(t *stdtesting.T) {
	_, repo, _ := setupTestModule(t)

	// Create
	deploy, err := repo.CreateWithContext(context.Background(), models.CreateDeploymentInput{
		ApplicationID: "app-1",
		CommitHash:    ptrString("abc1234"),
		CommitMessage: ptrString("initial commit"),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, deploy.ID)
	assert.Equal(t, "app-1", deploy.ApplicationID)
	assert.Equal(t, models.DeployStatusQueued, deploy.Status)

	// Get by ID
	found, err := repo.GetByIDWithContext(context.Background(), deploy.ID)
	require.NoError(t, err)
	assert.Equal(t, deploy.ID, found.ID)

	// Update status
	err = repo.UpdateStatusWithContext(context.Background(), deploy.ID, models.DeployStatusSuccess)
	require.NoError(t, err)

	found, err = repo.GetByIDWithContext(context.Background(), deploy.ID)
	require.NoError(t, err)
	assert.Equal(t, models.DeployStatusSuccess, found.Status)

	// Test ImageName and Retention
	img := "labuh-app-1:1234567890"
	require.NoError(t, repo.UpdateImageNameWithContext(context.Background(), deploy.ID, img))
	updated, err := repo.GetByIDWithContext(context.Background(), deploy.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.ImageName)
	assert.Equal(t, img, *updated.ImageName)

	recentImgs, err := repo.GetRecentSuccessfulImagesWithContext(context.Background(), "app-1", 5)
	require.NoError(t, err)
	assert.Contains(t, recentImgs, img)
}

type mockDeployEnqueuer struct {
	enqueued []models.DeploymentJob
}

func (m *mockDeployEnqueuer) Enqueue(job models.DeploymentJob) error {
	m.enqueued = append(m.enqueued, job)
	return nil
}

func TestDeploymentsHandler_StartAndRollback(t *stdtesting.T) {
	_, repo, handler := setupTestModule(t)
	workerMock := &mockDeployEnqueuer{}
	handler.SetDeployWorker(workerMock)

	// Create initial successful deployment with cached image
	deploy1, err := repo.CreateWithContext(context.Background(), models.CreateDeploymentInput{
		ApplicationID: "app-1",
		CommitHash:    ptrString("commit-v1"),
		CommitMessage: ptrString("v1 stable"),
		ImageName:     ptrString("labuh-app-1:v1-stable"),
	})
	require.NoError(t, err)
	require.NoError(t, repo.UpdateStatusWithContext(context.Background(), deploy1.ID, models.DeployStatusSuccess))

	// Create failed deployment
	deploy2, err := repo.CreateWithContext(context.Background(), models.CreateDeploymentInput{
		ApplicationID: "app-1",
		CommitHash:    ptrString("commit-v2"),
		CommitMessage: ptrString("v2 broken"),
	})
	require.NoError(t, err)
	require.NoError(t, repo.UpdateStatusWithContext(context.Background(), deploy2.ID, models.DeployStatusFailed))

	// Test Start
	req := httptest.NewRequest(http.MethodPost, "/deployments/"+deploy2.ID+"/start", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", deploy2.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.Start(w, req)
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)
	assert.Equal(t, "/deployments/"+deploy2.ID, w.Header().Get("HX-Redirect"))

	// Test Instant Rollback
	reqRollback := httptest.NewRequest(http.MethodPost, "/deployments/"+deploy2.ID+"/rollback", nil)
	reqRollback = reqRollback.WithContext(context.WithValue(reqRollback.Context(), chi.RouteCtxKey, rctx))

	wRollback := httptest.NewRecorder()
	handler.Rollback(wRollback, reqRollback)
	assert.Equal(t, http.StatusOK, wRollback.Result().StatusCode)
	assert.Contains(t, wRollback.Header().Get("HX-Redirect"), "/deployments/")

	// Pastikan Instant Rollback langsung enqueue Docker image yang ter-cache
	require.Len(t, workerMock.enqueued, 1)
	enqueuedJob := workerMock.enqueued[0]
	assert.Equal(t, string(models.SourceTypeDockerImage), enqueuedJob.SourceType)
	assert.Equal(t, "labuh-app-1:v1-stable", enqueuedJob.DockerImage)
	assert.Equal(t, 0, enqueuedJob.Priority)
}
