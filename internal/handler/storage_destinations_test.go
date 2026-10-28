package handler

import (
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
)

func TestStorageDestinationsHandler_CRUD(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	t.Cleanup(func() { _ = db.Close() })

	storageRepo := repo.NewStorageDestinationRepo(db)
	h := NewStorageDestinationsHandler(storageRepo)

	r := chi.NewRouter()
	r.Get("/settings/storage-destinations", h.List)
	r.Post("/settings/storage-destinations", h.Create)
	r.Post("/settings/storage-destinations/{id}/default", h.SetDefault)
	r.Post("/settings/storage-destinations/{id}/delete", h.Delete)

	// 1. Create Destination 1 (Default)
	form := url.Values{}
	form.Set("name", "Cloudflare R2 Primary")
	form.Set("endpoint", "https://r2.cloudflarestorage.com")
	form.Set("bucket", "r2-backups")
	form.Set("region", "auto")
	form.Set("access_key_id", "r2-key")
	form.Set("secret_access_key", "r2-secret")
	form.Set("prefix", "databases")
	form.Set("is_default", "true")

	req := httptest.NewRequest(http.MethodPost, "/settings/storage-destinations", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusFound, w.Code)

	list, err := storageRepo.List()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Cloudflare R2 Primary", list[0].Name)
	assert.True(t, list[0].IsDefault)

	// 2. Create Destination 2 (Not default)
	form2 := url.Values{}
	form2.Set("name", "Client AWS S3")
	form2.Set("endpoint", "https://s3.amazonaws.com")
	form2.Set("bucket", "client-bucket")
	form2.Set("region", "us-east-1")
	form2.Set("access_key_id", "aws-key")
	form2.Set("secret_access_key", "aws-secret")
	form2.Set("prefix", "db")

	req2 := httptest.NewRequest(http.MethodPost, "/settings/storage-destinations", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusFound, w2.Code)

	list, _ = storageRepo.List()
	assert.Len(t, list, 2)

	dest2 := list[1]
	if dest2.Name != "Client AWS S3" {
		dest2 = list[0]
	}

	// 3. Set Destination 2 as Default
	req3 := httptest.NewRequest(http.MethodPost, "/settings/storage-destinations/"+dest2.ID+"/default", nil)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusFound, w3.Code)

	defaultDest, err := storageRepo.GetDefault()
	require.NoError(t, err)
	assert.Equal(t, dest2.ID, defaultDest.ID)

	// 4. Delete Destination
	req4 := httptest.NewRequest(http.MethodPost, "/settings/storage-destinations/"+dest2.ID+"/delete", nil)
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, req4)
	assert.Equal(t, http.StatusFound, w4.Code)

	listAfter, _ := storageRepo.List()
	assert.Len(t, listAfter, 1)
}

func TestDatabasesHandler_BackupConfig(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	t.Cleanup(func() { _ = db.Close() })

	projectRepo := repo.NewProjectRepo(db)
	dbRepo := repo.NewDatabaseRepo(db)
	backupRepo := repo.NewDatabaseBackupRepo(db)
	storageRepo := repo.NewStorageDestinationRepo(db)

	proj, err := projectRepo.Create(models.CreateProjectInput{Name: "DB Project", Slug: "db-proj"})
	require.NoError(t, err)
	env, err := projectRepo.CreateEnvironment(proj.ID, "Production", "prod")
	require.NoError(t, err)

	dbService, err := dbRepo.Create(models.CreateDatabaseServiceInput{
		EnvironmentID: env.ID,
		Name:          "Analytics DB",
		Slug:          "analytics-db",
		Engine:        models.DatabaseEnginePostgres,
		Username:      "postgres",
		Password:      "pass",
		DatabaseName:  "analytics",
		Port:          5432,
	})
	require.NoError(t, err)

	h := NewDatabasesHandler(dbRepo, projectRepo)
	h.SetBackupDependencies(backupRepo, storageRepo, nil)

	r := chi.NewRouter()
	r.Post("/databases/{id}/backups/config", h.UpdateBackupConfig)

	form := url.Values{}
	form.Set("enabled", "true")
	form.Set("cron_expression", "0 2 * * *")
	form.Set("retention_count", "14")
	form.Set("custom_prefix", "analytics-backups")

	req := httptest.NewRequest(http.MethodPost, "/databases/"+dbService.ID+"/backups/config", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "/databases/"+dbService.ID, w.Header().Get("HX-Redirect"))

	cfg, err := backupRepo.GetConfig(dbService.ID)
	require.NoError(t, err)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 14, cfg.RetentionCount)
	assert.Equal(t, "0 2 * * *", cfg.CronExpression)
	assert.Equal(t, "analytics-backups", *cfg.CustomPrefix)
}
