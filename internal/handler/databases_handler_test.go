package handler

import (
	"net/http"
	"net/http/httptest"
	stdtesting "testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
)

func setupDatabasesHandler(t *stdtesting.T) (*DatabasesHandler, *repo.DatabaseRepo, *repo.ProjectRepo) {
	t.Helper()
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	t.Cleanup(func() { _ = db.Close() })

	dbRepo := repo.NewDatabaseRepo(db)
	projectRepo := repo.NewProjectRepo(db)
	dockerClient, _ := docker.NewClient()

	h := NewDatabasesHandler(dbRepo, projectRepo)
	h.SetDockerClient(dockerClient)
	return h, dbRepo, projectRepo
}

func TestDatabasesHandler_List(t *stdtesting.T) {
	h, dbRepo, projectRepo := setupDatabasesHandler(t)

	project, err := projectRepo.Create(models.CreateProjectInput{Name: "DB Project", Slug: "db-proj"})
	require.NoError(t, err)
	env, err := projectRepo.CreateEnvironment(project.ID, "Production", "prod")
	require.NoError(t, err)

	_, err = dbRepo.Create(models.CreateDatabaseServiceInput{
		EnvironmentID: env.ID,
		Name:          "Primary PG",
		Slug:          "primary-pg",
		Engine:        "postgres",
		Version:       "16",
		Port:          5432,
	})
	require.NoError(t, err)

	_, err = dbRepo.Create(models.CreateDatabaseServiceInput{
		EnvironmentID: env.ID,
		Name:          "Cache Redis",
		Slug:          "cache-redis",
		Engine:        "redis",
		Version:       "7",
		Port:          6379,
	})
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Get("/projects/{project_id}/environments/{env_id}/databases", h.List)

	// 1. Full page
	req := httptest.NewRequest(http.MethodGet, "/projects/"+project.ID+"/environments/"+env.ID+"/databases", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "<!doctype html>")
	assert.Contains(t, body, "databases-container")
	assert.Contains(t, body, "Primary PG")
	assert.Contains(t, body, "Cache Redis")

	// 2. HTMX search partial
	reqHTMX := httptest.NewRequest(http.MethodGet, "/projects/"+project.ID+"/environments/"+env.ID+"/databases?search=Primary", nil)
	reqHTMX.Header.Set("HX-Request", "true")
	wHTMX := httptest.NewRecorder()
	r.ServeHTTP(wHTMX, reqHTMX)

	assert.Equal(t, http.StatusOK, wHTMX.Code)
	bodyHTMX := wHTMX.Body.String()
	assert.NotContains(t, bodyHTMX, "<!doctype html>")
	assert.Contains(t, bodyHTMX, "Primary PG")
	assert.NotContains(t, bodyHTMX, "Cache Redis")
}

func TestDatabasesHandler_CreateAndLifecycle(t *stdtesting.T) {
	h, dbRepo, projectRepo := setupDatabasesHandler(t)

	project, err := projectRepo.Create(models.CreateProjectInput{Name: "Project DB", Slug: "proj-db"})
	require.NoError(t, err)
	env, err := projectRepo.CreateEnvironment(project.ID, "Production", "prod")
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Post("/projects/{project_id}/environments/{env_id}/databases", h.Create)
	r.Post("/databases/{id}/stop", h.Stop)
	r.Post("/databases/{id}/start", h.Start)
	r.Post("/databases/{id}/delete", h.Delete)

	// 1. Test Create Database (Auto password and default ports)
	req := httptest.NewRequest(http.MethodPost, "/projects/"+project.ID+"/environments/"+env.ID+"/databases", nil)
	req.PostForm = map[string][]string{
		"name":   {"Analytics DB"},
		"slug":   {"analytics-db"},
		"engine": {"postgres"},
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	dbs, err := dbRepo.GetByEnvironmentID(env.ID)
	require.NoError(t, err)
	require.Len(t, dbs, 1)

	createdDB := dbs[0]
	assert.Equal(t, "Analytics DB", createdDB.Name)
	assert.Equal(t, models.DatabaseEnginePostgres, createdDB.Engine)
	assert.Equal(t, 5432, createdDB.Port)
	assert.NotEmpty(t, createdDB.Password)
	assert.Equal(t, "postgres", createdDB.Username)

	// 2. Test Stop Database
	stopReq := httptest.NewRequest(http.MethodPost, "/databases/"+createdDB.ID+"/stop", nil)
	stopRec := httptest.NewRecorder()
	r.ServeHTTP(stopRec, stopReq)
	assert.Equal(t, http.StatusOK, stopRec.Code)

	updatedDB, err := dbRepo.GetByID(createdDB.ID)
	require.NoError(t, err)
	assert.Equal(t, models.DatabaseStatusStopped, updatedDB.Status)

	// 3. Test Start Database
	startReq := httptest.NewRequest(http.MethodPost, "/databases/"+createdDB.ID+"/start", nil)
	startRec := httptest.NewRecorder()
	r.ServeHTTP(startRec, startReq)
	assert.Equal(t, http.StatusOK, startRec.Code)

	startedDB, err := dbRepo.GetByID(createdDB.ID)
	require.NoError(t, err)
	assert.Equal(t, models.DatabaseStatusRunning, startedDB.Status)

	// 4. Test Delete Database
	delReq := httptest.NewRequest(http.MethodPost, "/databases/"+createdDB.ID+"/delete", nil)
	delRec := httptest.NewRecorder()
	r.ServeHTTP(delRec, delReq)
	assert.Equal(t, http.StatusOK, delRec.Code)

	deletedCheck, err := dbRepo.GetByID(createdDB.ID)
	assert.Error(t, err)
	assert.Nil(t, deletedCheck)
}

func TestGenerateConnectionString(t *stdtesting.T) {
	// Postgres
	pgConn := GenerateConnectionString(models.DatabaseEnginePostgres, "postgres", "secret123", "labuh-db-prod-pg", 5432, "mydb")
	assert.Equal(t, "postgres://postgres:secret123@labuh-db-prod-pg:5432/mydb?sslmode=disable", pgConn)

	// MySQL
	myConn := GenerateConnectionString(models.DatabaseEngineMySQL, "root", "secret123", "labuh-db-prod-mysql", 3306, "mydb")
	assert.Equal(t, "mysql://root:secret123@labuh-db-prod-mysql:3306/mydb", myConn)

	// Redis
	redisConn := GenerateConnectionString(models.DatabaseEngineRedis, "default", "secret123", "labuh-db-prod-redis", 6379, "")
	assert.Equal(t, "redis://:secret123@labuh-db-prod-redis:6379", redisConn)

	// Mongo
	mongoConn := GenerateConnectionString(models.DatabaseEngineMongoDB, "admin", "secret123", "labuh-db-prod-mongo", 27017, "mydb")
	assert.Equal(t, "mongodb://admin:secret123@labuh-db-prod-mongo:27017/mydb?authSource=admin", mongoConn)
}

func TestDatabasesHandler_Status(t *stdtesting.T) {
	h, dbRepo, projectRepo := setupDatabasesHandler(t)
	project, err := projectRepo.Create(models.CreateProjectInput{Name: "DB Status", Slug: "db-status"})
	require.NoError(t, err)
	env, err := projectRepo.CreateEnvironment(project.ID, "Production", "prod")
	require.NoError(t, err)

	dbService, err := dbRepo.Create(models.CreateDatabaseServiceInput{
		EnvironmentID: env.ID,
		Name:          "Status DB",
		Slug:          "status-db",
		Engine:        "postgres",
		Version:       "16",
		Port:          5432,
	})
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Get("/databases/{id}/status", h.Status)

	req := httptest.NewRequest(http.MethodGet, "/databases/"+dbService.ID+"/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `id="status-badge"`)
	assert.Contains(t, body, `/databases/`+dbService.ID+`/status`)
}
