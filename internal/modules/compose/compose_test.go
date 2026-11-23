package compose

import (
	"context"
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
	"github.com/akordium-id/get-labuh/internal/models"
)

type mockSettingProvider struct{}

func (m *mockSettingProvider) Get(ctx context.Context, key string) (string, error) {
	return "", nil
}

func setupTestModule(t *stdtesting.T) (*Module, *Handler) {
	t.Helper()
	db, err := database.Connect("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if err := database.RunAllMigrations(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	mod := New(db, nil, &mockSettingProvider{})
	return mod, mod.Handler
}

func TestComposeHandler_CRUD(t *stdtesting.T) {
	mod, h := setupTestModule(t)

	// Create
	app, err := mod.Repo.CreateWithContext(context.Background(), models.CreateComposeApplicationInput{
		EnvironmentID:   "env-123",
		Name:            "Wordpress Stack",
		ComposeFilePath: "docker-compose.yml",
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, app.ID)
	assert.Equal(t, "Wordpress Stack", app.Name)
	assert.Equal(t, models.ComposeStatusIdle, app.Status)

	// List
	reqList := httptest.NewRequest(http.MethodGet, "/projects/p1/environments/env-123/compose-apps", nil)
	wList := httptest.NewRecorder()
	h.List(wList, reqList)
	// without chi url param context, this might return 400 Bad Request, so we verify repo query
	apps, err := mod.Repo.GetByEnvironmentIDWithContext(context.Background(), "env-123")
	assert.NoError(t, err)
	assert.Len(t, apps, 1)

	// Update Status
	err = mod.Repo.UpdateStatusWithContext(context.Background(), app.ID, models.ComposeStatusRunning)
	assert.NoError(t, err)

	updated, err := mod.Repo.GetByIDWithContext(context.Background(), app.ID)
	assert.NoError(t, err)
	assert.Equal(t, models.ComposeStatusRunning, updated.Status)

	// Update Custom Domain
	domain := "wp.example.com"
	err = mod.Repo.UpdateCustomDomainWithContext(context.Background(), app.ID, &domain)
	assert.NoError(t, err)

	withDomain, err := mod.Repo.GetByIDWithContext(context.Background(), app.ID)
	assert.NoError(t, err)
	assert.NotNil(t, withDomain.CustomDomain)
	assert.Equal(t, "wp.example.com", *withDomain.CustomDomain)

	// Update YAML and Parsed Services
	rawYAML := `
services:
  web:
    image: nginx:alpine
    ports:
      - "80:80"
`
	parsedServices := []models.ComposeServiceDef{
		{
			Name:  "web",
			Image: "nginx:alpine",
			Ports: []string{"80:80"},
		},
	}
	err = mod.Repo.UpdateYAMLWithContext(context.Background(), app.ID, rawYAML, parsedServices)
	assert.NoError(t, err)

	withYAML, err := mod.Repo.GetByIDWithContext(context.Background(), app.ID)
	assert.NoError(t, err)
	assert.Equal(t, rawYAML, withYAML.RawYAML)
	assert.Len(t, withYAML.ParsedServices, 1)
	assert.Equal(t, "web", withYAML.ParsedServices[0].Name)

	// Update Deployment Status
	now := time.Now()
	errMsg := "some deploy error"
	err = mod.Repo.UpdateDeploymentStatusWithContext(context.Background(), app.ID, models.ComposeStatusFailed, &now, &errMsg)
	assert.NoError(t, err)

	withDeployStatus, err := mod.Repo.GetByIDWithContext(context.Background(), app.ID)
	assert.NoError(t, err)
	assert.Equal(t, models.ComposeStatusFailed, withDeployStatus.Status)
	assert.NotNil(t, withDeployStatus.LastDeployedAt)
	assert.NotNil(t, withDeployStatus.ErrorMessage)
	assert.Equal(t, "some deploy error", *withDeployStatus.ErrorMessage)

	// Delete
	err = mod.Repo.DeleteWithContext(context.Background(), app.ID)
	assert.NoError(t, err)

	deleted, err := mod.Repo.GetByIDWithContext(context.Background(), app.ID)
	assert.Error(t, err)
	assert.Nil(t, deleted)
}

type mockOrchestrator struct {
	upCalled      bool
	downCalled    bool
	restartCalled bool
	serviceName   string
}

func (m *mockOrchestrator) Up(ctx context.Context, app *models.ComposeApplication, envVars []models.AppEnvVar) (string, error) {
	m.upCalled = true
	return "up success", nil
}

func (m *mockOrchestrator) Down(ctx context.Context, app *models.ComposeApplication) (string, error) {
	m.downCalled = true
	return "down success", nil
}

func (m *mockOrchestrator) RestartService(ctx context.Context, app *models.ComposeApplication, serviceName string) (string, error) {
	m.restartCalled = true
	m.serviceName = serviceName
	return "restarted", nil
}

func (m *mockOrchestrator) InspectServices(ctx context.Context, app *models.ComposeApplication) ([]models.ComposeServiceRuntime, error) {
	return []models.ComposeServiceRuntime{
		{Name: "web", Status: "running"},
	}, nil
}

func (m *mockOrchestrator) GetServiceLogs(ctx context.Context, app *models.ComposeApplication, serviceName string, tail int) (string, error) {
	return "mock-log-for-" + serviceName, nil
}

func TestComposeHandler_YAMLValidationAndDeploy(t *stdtesting.T) {
	mod, h := setupTestModule(t)
	orch := &mockOrchestrator{}
	mod.SetOrchestrator(orch)

	// 1. Create Compose App
	app, err := mod.Repo.CreateWithContext(context.Background(), models.CreateComposeApplicationInput{
		EnvironmentID: "env-test",
		Name:          "Web Stack",
		RawYAML: `
services:
  web:
    image: nginx:alpine
`,
	})
	require.NoError(t, err)

	// 2. Validate YAML endpoint
	formVal := url.Values{"raw_yaml": {`services: { web: { image: nginx } }`}}
	reqVal := httptest.NewRequest(http.MethodPost, "/compose-apps/"+app.ID+"/validate", strings.NewReader(formVal.Encode()))
	reqVal.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wVal := httptest.NewRecorder()
	h.ValidateYAML(wVal, reqVal)
	assert.Equal(t, http.StatusOK, wVal.Code)
	assert.Contains(t, wVal.Body.String(), "Valid")

	// 3. Update YAML endpoint
	newYAML := `
services:
  web:
    image: nginx:alpine
  redis:
    image: redis:alpine
`
	formUp := url.Values{"raw_yaml": {newYAML}}
	reqUp := httptest.NewRequest(http.MethodPost, "/compose-apps/"+app.ID+"/yaml", strings.NewReader(formUp.Encode()))
	reqUp.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctxUp := chi.NewRouteContext()
	rctxUp.URLParams.Add("id", app.ID)
	reqUp = reqUp.WithContext(context.WithValue(reqUp.Context(), chi.RouteCtxKey, rctxUp))
	wUp := httptest.NewRecorder()
	h.UpdateYAML(wUp, reqUp)
	assert.Equal(t, http.StatusOK, wUp.Code)

	reloaded, err := mod.Repo.GetByIDWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	assert.Len(t, reloaded.ParsedServices, 2)

	// 4. Deploy endpoint
	reqDeploy := httptest.NewRequest(http.MethodPost, "/compose-apps/"+app.ID+"/deploy", nil)
	rctxDep := chi.NewRouteContext()
	rctxDep.URLParams.Add("id", app.ID)
	reqDeploy = reqDeploy.WithContext(context.WithValue(reqDeploy.Context(), chi.RouteCtxKey, rctxDep))
	wDeploy := httptest.NewRecorder()
	h.Deploy(wDeploy, reqDeploy)
	assert.Equal(t, http.StatusOK, wDeploy.Code)
	assert.True(t, orch.upCalled)

	deployed, err := mod.Repo.GetByIDWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ComposeStatusRunning, deployed.Status)
	assert.NotNil(t, deployed.LastDeployedAt)

	// 5. Restart Sub-service
	reqRestart := httptest.NewRequest(http.MethodPost, "/compose-apps/"+app.ID+"/services/web/restart", nil)
	rctxRestart := chi.NewRouteContext()
	rctxRestart.URLParams.Add("id", app.ID)
	rctxRestart.URLParams.Add("service", "web")
	reqRestart = reqRestart.WithContext(context.WithValue(reqRestart.Context(), chi.RouteCtxKey, rctxRestart))
	wRestart := httptest.NewRecorder()
	h.RestartService(wRestart, reqRestart)
	assert.Equal(t, http.StatusOK, wRestart.Code)
	assert.True(t, orch.restartCalled)
	assert.Equal(t, "web", orch.serviceName)

	// 6. Service Logs
	reqLogs := httptest.NewRequest(http.MethodGet, "/compose-apps/"+app.ID+"/services/web/logs", nil)
	rctxLogs := chi.NewRouteContext()
	rctxLogs.URLParams.Add("id", app.ID)
	rctxLogs.URLParams.Add("service", "web")
	reqLogs = reqLogs.WithContext(context.WithValue(reqLogs.Context(), chi.RouteCtxKey, rctxLogs))
	wLogs := httptest.NewRecorder()
	h.ServiceLogs(wLogs, reqLogs)
	assert.Equal(t, http.StatusOK, wLogs.Code)
	assert.Contains(t, wLogs.Body.String(), "mock-log-for-web")

	// 7. Stop Stack
	reqStop := httptest.NewRequest(http.MethodPost, "/compose-apps/"+app.ID+"/stop", nil)
	rctxStop := chi.NewRouteContext()
	rctxStop.URLParams.Add("id", app.ID)
	reqStop = reqStop.WithContext(context.WithValue(reqStop.Context(), chi.RouteCtxKey, rctxStop))
	wStop := httptest.NewRecorder()
	h.Stop(wStop, reqStop)
	assert.Equal(t, http.StatusOK, wStop.Code)
	assert.True(t, orch.downCalled)
}
