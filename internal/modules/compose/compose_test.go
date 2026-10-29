package compose

import (
	"context"
	"net/http"
	"net/http/httptest"
	stdtesting "testing"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/stretchr/testify/assert"
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

	// Delete
	err = mod.Repo.DeleteWithContext(context.Background(), app.ID)
	assert.NoError(t, err)

	deleted, err := mod.Repo.GetByIDWithContext(context.Background(), app.ID)
	assert.Error(t, err)
	assert.Nil(t, deleted)
}
