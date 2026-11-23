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

	"github.com/akordium-id/get-labuh/internal/models"
)

type mockRegistryProvider struct {
	registries []*models.DockerRegistry
}

func (m *mockRegistryProvider) List(ctx context.Context) ([]*models.DockerRegistry, error) {
	return m.registries, nil
}

func (m *mockRegistryProvider) GetByID(ctx context.Context, id string) (*models.DockerRegistry, error) {
	for _, r := range m.registries {
		if r.ID == id {
			return r, nil
		}
	}
	return nil, nil
}

func TestApplicationsHandler_RegistryConfig(t *stdtesting.T) {
	mod, _, _ := setupTestModule(t)

	mockRegs := &mockRegistryProvider{
		registries: []*models.DockerRegistry{
			{
				ID:            "reg-123",
				Name:          "GitHub Packages",
				ServerAddress: "ghcr.io",
				Username:      "myuser",
			},
		},
	}
	mod.SetRegistryRepo(mockRegs)

	// 1. Create app with docker_registry_id
	createForm := url.Values{
		"name":               {"Private App"},
		"source_type":        {string(models.SourceTypeDockerImage)},
		"docker_image":       {"ghcr.io/org/private-app:1.0.0"},
		"docker_registry_id": {"reg-123"},
		"app_port":           {"8080"},
	}

	reqCreate := httptest.NewRequest(http.MethodPost, "/projects/p-1/environments/env-1/applications", strings.NewReader(createForm.Encode()))
	reqCreate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("project_id", "p-1")
	rctx.URLParams.Add("env_id", "env-1")
	reqCreate = reqCreate.WithContext(context.WithValue(reqCreate.Context(), chi.RouteCtxKey, rctx))
	wCreate := httptest.NewRecorder()

	mod.Handler.Create(wCreate, reqCreate)
	require.Equal(t, http.StatusOK, wCreate.Code)

	// Verify app created with DockerRegistryID
	apps, err := mod.Repo.GetByEnvironmentIDWithContext(context.Background(), "env-1")
	require.NoError(t, err)
	require.Len(t, apps, 1)
	app := apps[0]
	assert.Equal(t, "Private App", app.Name)
	require.NotNil(t, app.DockerRegistryID)
	assert.Equal(t, "reg-123", *app.DockerRegistryID)

	// 2. Call Handler.Get to ensure rendering works with registries
	reqGet := httptest.NewRequest(http.MethodGet, "/applications/"+app.ID, nil)
	rctxGet := chi.NewRouteContext()
	rctxGet.URLParams.Add("id", app.ID)
	reqGet = reqGet.WithContext(context.WithValue(reqGet.Context(), chi.RouteCtxKey, rctxGet))
	wGet := httptest.NewRecorder()

	mod.Handler.Get(wGet, reqGet)
	assert.Equal(t, http.StatusOK, wGet.Code)

	// 3. Update Registry Config: Change image and clear registry ID
	updateForm := url.Values{
		"docker_image":       {"ghcr.io/org/private-app:2.0.0"},
		"docker_registry_id": {""}, // None
	}

	reqUpdate := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/registry", strings.NewReader(updateForm.Encode()))
	reqUpdate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctxUpdate := chi.NewRouteContext()
	rctxUpdate.URLParams.Add("id", app.ID)
	reqUpdate = reqUpdate.WithContext(context.WithValue(reqUpdate.Context(), chi.RouteCtxKey, rctxUpdate))
	wUpdate := httptest.NewRecorder()

	mod.Handler.UpdateRegistryConfig(wUpdate, reqUpdate)
	assert.Equal(t, http.StatusFound, wUpdate.Code)

	// Verify updated state
	updatedApp, err := mod.Repo.GetByIDWithContext(context.Background(), app.ID)
	require.NoError(t, err)
	assert.Nil(t, updatedApp.DockerRegistryID)
	require.NotNil(t, updatedApp.DockerImage)
	assert.Equal(t, "ghcr.io/org/private-app:2.0.0", *updatedApp.DockerImage)
}
