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
	"github.com/akordium-id/get-labuh/internal/models"
)

func TestTerminalHandler_Pages(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	t.Cleanup(func() { _ = db.Close() })

	projectRepo := repo.NewProjectRepo(db)
	appRepo := repo.NewApplicationRepo(db)
	dbRepo := repo.NewDatabaseRepo(db)

	proj, err := projectRepo.Create(models.CreateProjectInput{Name: "Terminal Project", Slug: "term-proj"})
	require.NoError(t, err)
	env, err := projectRepo.CreateEnvironment(proj.ID, "Production", "prod")
	require.NoError(t, err)

	// 1. App running with container
	app, err := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "Web App",
		Slug:          "web-app",
		SourceType:    models.SourceTypeDockerImage,
	})
	require.NoError(t, err)

	cid := "mock-container-id-123456"
	_ = appRepo.UpdateContainer(app.ID, cid)

	// 2. Database running with container
	dbService, err := dbRepo.Create(models.CreateDatabaseServiceInput{
		EnvironmentID: env.ID,
		Name:          "Postgres DB",
		Slug:          "pg-db",
		Engine:        models.DatabaseEnginePostgres,
		Username:      "postgres",
		Password:      "pass",
		DatabaseName:  "appdb",
		Port:          5432,
	})
	require.NoError(t, err)
	_ = dbRepo.UpdateContainer(dbService.ID, "mock-db-container-789")

	h := NewTerminalHandler(nil, appRepo, dbRepo, nil)

	r := chi.NewRouter()
	r.Get("/applications/{id}/terminal", h.ApplicationTerminal)
	r.Get("/databases/{id}/terminal", h.DatabaseTerminal)

	// Test GET application terminal page
	reqApp := httptest.NewRequest(http.MethodGet, "/applications/"+app.ID+"/terminal", nil)
	wApp := httptest.NewRecorder()
	r.ServeHTTP(wApp, reqApp)
	assert.Equal(t, http.StatusOK, wApp.Code)
	assert.Contains(t, wApp.Body.String(), "Web App (App)")
	assert.Contains(t, wApp.Body.String(), "/containers/mock-container-id-123456/terminal/ws")

	// Test GET database terminal page
	reqDB := httptest.NewRequest(http.MethodGet, "/databases/"+dbService.ID+"/terminal", nil)
	wDB := httptest.NewRecorder()
	r.ServeHTTP(wDB, reqDB)
	assert.Equal(t, http.StatusOK, wDB.Code)
	assert.Contains(t, wDB.Body.String(), "Postgres DB (postgres)")
	assert.Contains(t, wDB.Body.String(), "/containers/mock-db-container-789/terminal/ws")
}

func TestTerminalHandler_NotRunningContainer(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	t.Cleanup(func() { _ = db.Close() })

	projectRepo := repo.NewProjectRepo(db)
	appRepo := repo.NewApplicationRepo(db)

	proj, err := projectRepo.Create(models.CreateProjectInput{Name: "Proj", Slug: "proj"})
	require.NoError(t, err)
	env, err := projectRepo.CreateEnvironment(proj.ID, "Production", "prod")
	require.NoError(t, err)

	// App without container ID
	app, err := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "Idle App",
		Slug:          "idle-app",
	})
	require.NoError(t, err)

	h := NewTerminalHandler(nil, appRepo, nil, nil)
	r := chi.NewRouter()
	r.Get("/applications/{id}/terminal", h.ApplicationTerminal)

	req := httptest.NewRequest(http.MethodGet, "/applications/"+app.ID+"/terminal", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Container is not running")
}
