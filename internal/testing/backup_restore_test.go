package testing

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
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/modules/applications"
	"github.com/akordium-id/get-labuh/internal/modules/databases"
	"github.com/akordium-id/get-labuh/internal/modules/projects"
	"github.com/akordium-id/get-labuh/internal/modules/settings"
)

type mockDatabaseBackupRunner struct {
	lastBackupID string
	lastTargetDB *models.DatabaseService
	backupCalled bool
}

func (m *mockDatabaseBackupRunner) RunBackup(ctx context.Context, databaseID string) (*models.DatabaseBackup, error) {
	m.backupCalled = true
	return &models.DatabaseBackup{
		ID:         "b-test-1",
		DatabaseID: databaseID,
		Filename:   "db-test.sql.gz",
		Status:     "completed",
	}, nil
}

func (m *mockDatabaseBackupRunner) RestoreBackup(ctx context.Context, backupID string, targetDB *models.DatabaseService) error {
	m.lastBackupID = backupID
	m.lastTargetDB = targetDB
	return nil
}

type mockAppVolumeBackupRunner struct {
	lastBackupAppID string
	lastRestoreID   string
}

func (m *mockAppVolumeBackupRunner) RunBackup(ctx context.Context, appID string) (*models.AppBackup, error) {
	m.lastBackupAppID = appID
	return &models.AppBackup{
		ID:            "vol-b1",
		ApplicationID: appID,
		Filename:      "vol-snapshot.tar.gz",
		Status:        "completed",
	}, nil
}

func (m *mockAppVolumeBackupRunner) RestoreAppVolume(ctx context.Context, backupID string) error {
	m.lastRestoreID = backupID
	return nil
}

func TestIntegration_DatabaseAndAppVolumeRestoreFlow(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	defer db.Close()

	ctx := context.Background()
	projectRepo := repo.NewProjectRepo(db)
	userRepo := repo.NewUserRepo(db)
	projectsMod := projects.New(db, userRepo)
	dbMod := databases.New(db, projectRepo)
	appMod := applications.New(db, projectRepo, projectRepo, nil, nil, nil, nil)
	settingsMod := settings.New(db, "")

	// Setup Project & Environment
	proj, err := projectRepo.Create(models.CreateProjectInput{Name: "Backup E2E Project"})
	require.NoError(t, err)
	env, err := projectRepo.CreateEnvironment(proj.ID, "Production", "production")
	require.NoError(t, err)
	envID := env.ID

	// -------------------------------------------------------------
	// 1. Database Restore: In-Place & Restore To New Database
	// -------------------------------------------------------------
	dbRunner := &mockDatabaseBackupRunner{}
	dbMod.SetBackupDependencies(settingsMod.StorageRepo, dbRunner, nil)

	dbRouter := chi.NewRouter()
	dbMod.RegisterRoutes(dbRouter)

	// Create DB Service
	dbService, err := dbMod.ServiceRepo.CreateWithContext(ctx, models.CreateDatabaseServiceInput{
		EnvironmentID: envID,
		Name:          "Production Postgres",
		Slug:          "production-postgres",
		Engine:        models.DatabaseEnginePostgres,
		Version:       "16-alpine",
		Port:          5432,
		Username:      "labuh",
		Password:      "secret123",
		DatabaseName:  "production_db",
	})
	require.NoError(t, err)

	// Create dummy backup record
	bRecord := &models.DatabaseBackup{
		ID:          "b-test-1",
		DatabaseID:  dbService.ID,
		Filename:    "prod-dump-2026.sql.gz",
		StorageType: "local",
		Status:      "completed",
	}
	err = dbMod.BackupRepo.CreateBackupWithContext(ctx, bRecord)
	require.NoError(t, err)

	// 1A. In-Place Restore
	formInPlace := url.Values{}
	formInPlace.Set("restore_mode", "in_place")
	reqInPlace := httptest.NewRequest(http.MethodPost, "/databases/"+dbService.ID+"/backups/"+bRecord.ID+"/restore", strings.NewReader(formInPlace.Encode()))
	reqInPlace.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqInPlace.Header.Set("HX-Request", "true")
	recInPlace := httptest.NewRecorder()
	dbRouter.ServeHTTP(recInPlace, reqInPlace)

	assert.Equal(t, http.StatusOK, recInPlace.Code)
	assert.Equal(t, bRecord.ID, dbRunner.lastBackupID)
	require.NotNil(t, dbRunner.lastTargetDB)
	assert.Equal(t, dbService.ID, dbRunner.lastTargetDB.ID)

	// 1B. Restore to New Database
	formNew := url.Values{}
	formNew.Set("restore_mode", "new_database")
	formNew.Set("new_database_name", "Production Postgres Staging")
	reqNew := httptest.NewRequest(http.MethodPost, "/databases/"+dbService.ID+"/backups/"+bRecord.ID+"/restore", strings.NewReader(formNew.Encode()))
	reqNew.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqNew.Header.Set("HX-Request", "true")
	recNew := httptest.NewRecorder()
	dbRouter.ServeHTTP(recNew, reqNew)

	assert.Equal(t, http.StatusOK, recNew.Code)
	assert.Equal(t, bRecord.ID, dbRunner.lastBackupID)
	require.NotNil(t, dbRunner.lastTargetDB)
	assert.NotEqual(t, dbService.ID, dbRunner.lastTargetDB.ID)
	assert.Equal(t, "Production Postgres Staging", dbRunner.lastTargetDB.Name)

	// -------------------------------------------------------------
	// 2. Application Volume Backup & Safe Restore Flow
	// -------------------------------------------------------------
	volRunner := &mockAppVolumeBackupRunner{}
	appMod.SetVolumeBackupRunner(volRunner)
	appMod.SetStorageProvider(settingsMod.StorageRepo)

	appRouter := chi.NewRouter()
	appMod.RegisterRoutes(appRouter)

	appService, err := appMod.Repo.CreateWithContext(ctx, models.CreateApplicationInput{
		EnvironmentID: envID,
		Name:          "Web App With Persistent Volume",
		SourceType:    models.SourceTypeDockerImage,
		AppPort:       8080,
	})
	require.NoError(t, err)

	// Add Persistent Volume
	vol, err := appMod.Repo.CreateVolume(ctx, models.CreateAppVolumeInput{
		ApplicationID: appService.ID,
		Name:          "uploads-data",
		MountPath:     "/app/uploads",
		VolumeType:    "named",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, vol.ID)

	// Configure Backup
	formAppCfg := url.Values{}
	formAppCfg.Set("enabled", "true")
	formAppCfg.Set("cron_expression", "0 2 * * *")
	formAppCfg.Set("retention_count", "14")
	reqAppCfg := httptest.NewRequest(http.MethodPost, "/applications/"+appService.ID+"/backups/config", strings.NewReader(formAppCfg.Encode()))
	reqAppCfg.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqAppCfg.Header.Set("HX-Request", "true")
	recAppCfg := httptest.NewRecorder()
	appRouter.ServeHTTP(recAppCfg, reqAppCfg)
	assert.Equal(t, http.StatusOK, recAppCfg.Code)

	cfg, err := appMod.Repo.GetBackupConfigWithContext(ctx, appService.ID)
	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 14, cfg.RetentionCount)

	// Trigger Manual Volume Backup
	reqAppManual := httptest.NewRequest(http.MethodPost, "/applications/"+appService.ID+"/backups/create", nil)
	reqAppManual.Header.Set("HX-Request", "true")
	recAppManual := httptest.NewRecorder()
	appRouter.ServeHTTP(recAppManual, reqAppManual)
	assert.Equal(t, http.StatusOK, recAppManual.Code)
	assert.Equal(t, appService.ID, volRunner.lastBackupAppID)

	// Create Snapshot Record
	appBackupRecord, err := appMod.Repo.CreateBackupWithContext(ctx, models.CreateAppBackupInput{
		ApplicationID: appService.ID,
		Filename:      "app-vol-backup.tar.gz",
		Status:        "completed",
		StorageType:   "local",
	})
	require.NoError(t, err)

	// Safe Restore Application Volume
	reqAppRestore := httptest.NewRequest(http.MethodPost, "/applications/"+appService.ID+"/backups/"+appBackupRecord.ID+"/restore", nil)
	reqAppRestore.Header.Set("HX-Request", "true")
	recAppRestore := httptest.NewRecorder()
	appRouter.ServeHTTP(recAppRestore, reqAppRestore)
	assert.Equal(t, http.StatusOK, recAppRestore.Code)
	assert.Equal(t, appBackupRecord.ID, volRunner.lastRestoreID)

	_ = projectsMod // ensure referenced
}
