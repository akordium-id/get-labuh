package databases

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

type mockEnvProvider struct {
	env *models.Environment
}

func (m *mockEnvProvider) GetEnvironmentByID(id string) (*models.Environment, error) {
	if m.env != nil {
		return m.env, nil
	}
	return &models.Environment{ID: id, Slug: "prod", Name: "Production"}, nil
}

func setupTestModule(t *stdtesting.T) (*Module, *Handler) {
	t.Helper()
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	t.Cleanup(func() { _ = db.Close() })

	mod := New(db, &mockEnvProvider{})
	return mod, mod.Handler
}

func TestDatabasesHandler_CRUD(t *stdtesting.T) {
	mod, _ := setupTestModule(t)

	// Create service
	dbService, err := mod.ServiceRepo.CreateWithContext(context.Background(), models.CreateDatabaseServiceInput{
		EnvironmentID: "env-test-1",
		Name:          "Primary PG",
		Slug:          "primary-pg",
		Engine:        models.DatabaseEnginePostgres,
		Version:       "16",
		Port:          5432,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, dbService.ID)
	assert.Equal(t, "Primary PG", dbService.Name)
	assert.Equal(t, models.DatabaseStatusIdle, dbService.Status)

	// Get by ID
	found, err := mod.ServiceRepo.GetByIDWithContext(context.Background(), dbService.ID)
	require.NoError(t, err)
	assert.Equal(t, dbService.ID, found.ID)

	// Update status
	err = mod.ServiceRepo.UpdateStatusWithContext(context.Background(), dbService.ID, models.DatabaseStatusRunning)
	require.NoError(t, err)

	updated, err := mod.ServiceRepo.GetByIDWithContext(context.Background(), dbService.ID)
	require.NoError(t, err)
	assert.Equal(t, models.DatabaseStatusRunning, updated.Status)

	// Backup config upsert
	cron := "0 3 * * *"
	retention := 14
	err = mod.BackupRepo.UpsertConfigWithContext(context.Background(), &models.DatabaseBackupConfig{
		DatabaseID:     dbService.ID,
		Enabled:        true,
		CronExpression: cron,
		RetentionCount: retention,
	})
	require.NoError(t, err)

	cfg, err := mod.BackupRepo.GetConfigWithContext(context.Background(), dbService.ID)
	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, cron, cfg.CronExpression)
	assert.Equal(t, retention, cfg.RetentionCount)

	// Delete
	err = mod.ServiceRepo.DeleteWithContext(context.Background(), dbService.ID)
	require.NoError(t, err)

	deleted, err := mod.ServiceRepo.GetByIDWithContext(context.Background(), dbService.ID)
	assert.Error(t, err)
	assert.Nil(t, deleted)
}

func TestDatabasesHandler_List_Empty(t *stdtesting.T) {
	_, h := setupTestModule(t)

	req := httptest.NewRequest(http.MethodGet, "/projects/p1/environments/env-1/databases", nil)
	w := httptest.NewRecorder()
	// without chi URL param in context it returns Bad Request, verify status code handling
	h.List(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
}

func TestDatabasesHandler_UpdateResourceLimits(t *stdtesting.T) {
	mod, h := setupTestModule(t)

	dbService, err := mod.ServiceRepo.CreateWithContext(context.Background(), models.CreateDatabaseServiceInput{
		EnvironmentID: "env-1",
		Name:          "Primary Postgres",
		Slug:          "primary-postgres",
		Engine:        models.DatabaseEnginePostgres,
	})
	require.NoError(t, err)

	form := url.Values{}
	form.Set("cpu_limit", "0.75")
	form.Set("memory_limit", "1024")
	form.Set("memory_reservation", "512")

	req := httptest.NewRequest(http.MethodPost, "/databases/"+dbService.ID+"/resources", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", dbService.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	h.UpdateResourceLimits(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/databases/"+dbService.ID, rec.Header().Get("Location"))

	// Check DB
	updated, err := mod.ServiceRepo.GetByIDWithContext(context.Background(), dbService.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.CPULimit)
	assert.Equal(t, 0.75, *updated.CPULimit)
	require.NotNil(t, updated.MemoryLimit)
	assert.Equal(t, int64(1024), *updated.MemoryLimit)
	require.NotNil(t, updated.MemoryReservation)
	assert.Equal(t, int64(512), *updated.MemoryReservation)
}

type mockBackupRunner struct {
	calledRestore bool
	targetDB      *models.DatabaseService
	backupID      string
}

func (m *mockBackupRunner) RunBackup(ctx context.Context, databaseID string) (*models.DatabaseBackup, error) {
	return nil, nil
}

func (m *mockBackupRunner) RestoreBackup(ctx context.Context, backupID string, targetDB *models.DatabaseService) error {
	m.calledRestore = true
	m.backupID = backupID
	m.targetDB = targetDB
	return nil
}

func TestDatabaseRestoreHandler(t *stdtesting.T) {
	mod, h := setupTestModule(t)

	dbService, err := mod.ServiceRepo.CreateWithContext(context.Background(), models.CreateDatabaseServiceInput{
		EnvironmentID: "env-1",
		Name:          "Primary Postgres",
		Slug:          "primary-postgres",
		Engine:        models.DatabaseEnginePostgres,
	})
	require.NoError(t, err)

	runner := &mockBackupRunner{}
	h.SetBackupDependencies(mod.BackupRepo, nil, runner, nil)

	// 1. Test In-place Restore
	formInPlace := url.Values{}
	formInPlace.Set("mode", "in_place")

	reqInPlace := httptest.NewRequest(http.MethodPost, "/databases/"+dbService.ID+"/backups/b1/restore", strings.NewReader(formInPlace.Encode()))
	reqInPlace.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqInPlace.Header.Set("HX-Request", "true")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", dbService.ID)
	rctx.URLParams.Add("backup_id", "b1")
	reqInPlace = reqInPlace.WithContext(context.WithValue(reqInPlace.Context(), chi.RouteCtxKey, rctx))

	recInPlace := httptest.NewRecorder()
	h.RestoreBackup(recInPlace, reqInPlace)

	assert.Equal(t, http.StatusOK, recInPlace.Code)
	assert.True(t, runner.calledRestore)
	assert.Equal(t, "b1", runner.backupID)
	assert.Equal(t, dbService.ID, runner.targetDB.ID)

	// 2. Test Restore to New Database
	runner.calledRestore = false
	formNew := url.Values{}
	formNew.Set("mode", "new_database")
	formNew.Set("new_name", "Restored Postgres")

	reqNew := httptest.NewRequest(http.MethodPost, "/databases/"+dbService.ID+"/backups/b2/restore", strings.NewReader(formNew.Encode()))
	reqNew.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqNew.Header.Set("HX-Request", "true")

	rctxNew := chi.NewRouteContext()
	rctxNew.URLParams.Add("id", dbService.ID)
	rctxNew.URLParams.Add("backup_id", "b2")
	reqNew = reqNew.WithContext(context.WithValue(reqNew.Context(), chi.RouteCtxKey, rctxNew))

	recNew := httptest.NewRecorder()
	h.RestoreBackup(recNew, reqNew)

	assert.Equal(t, http.StatusOK, recNew.Code)
	assert.True(t, runner.calledRestore)
	assert.Equal(t, "b2", runner.backupID)
	assert.NotEqual(t, dbService.ID, runner.targetDB.ID)
	assert.Equal(t, "Restored Postgres", runner.targetDB.Name)
}
