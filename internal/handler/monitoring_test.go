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

func TestMonitoringHandler_Stats(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	t.Cleanup(func() { _ = db.Close() })

	appRepo := repo.NewApplicationRepo(db)
	dbRepo := repo.NewDatabaseRepo(db)
	projectRepo := repo.NewProjectRepo(db)
	dockerClient, _ := docker.NewClient()

	h := NewMonitoringHandler(appRepo, dbRepo, dockerClient)

	project, err := projectRepo.Create(models.CreateProjectInput{Name: "Monitoring Project", Slug: "mon-proj"})
	require.NoError(t, err)
	env, err := projectRepo.CreateEnvironment(project.ID, "Production", "prod")
	require.NoError(t, err)

	app, err := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "App Mon",
		Slug:          "app-mon",
		SourceType:    models.SourceTypeGit,
	})
	require.NoError(t, err)

	dbService, err := dbRepo.Create(models.CreateDatabaseServiceInput{
		EnvironmentID: env.ID,
		Name:          "DB Mon",
		Slug:          "db-mon",
		Engine:        "postgres",
		Version:       "16",
		Port:          5432,
	})
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Get("/applications/{id}/stats", h.ApplicationStats)
	r.Get("/databases/{id}/stats", h.DatabaseStats)

	// 1. JSON Request for Application
	reqJSON := httptest.NewRequest(http.MethodGet, "/applications/"+app.ID+"/stats", nil)
	wJSON := httptest.NewRecorder()
	r.ServeHTTP(wJSON, reqJSON)
	assert.Equal(t, http.StatusOK, wJSON.Code)
	assert.Equal(t, "application/json", wJSON.Header().Get("Content-Type"))

	// 2. HTMX Polling Request for Application returns MetricsCard partial
	reqHTMX := httptest.NewRequest(http.MethodGet, "/applications/"+app.ID+"/stats", nil)
	reqHTMX.Header.Set("HX-Request", "true")
	wHTMX := httptest.NewRecorder()
	r.ServeHTTP(wHTMX, reqHTMX)
	assert.Equal(t, http.StatusOK, wHTMX.Code)
	bodyHTMX := wHTMX.Body.String()
	assert.Contains(t, bodyHTMX, `id="metrics-card"`)
	assert.Contains(t, bodyHTMX, "Live Resource Usage")
	assert.Contains(t, bodyHTMX, "CPU Usage")
	assert.Contains(t, bodyHTMX, "Memory")

	// 3. HTMX Polling Request for Database returns MetricsCard partial
	reqDBHTMX := httptest.NewRequest(http.MethodGet, "/databases/"+dbService.ID+"/stats", nil)
	reqDBHTMX.Header.Set("HX-Request", "true")
	wDBHTMX := httptest.NewRecorder()
	r.ServeHTTP(wDBHTMX, reqDBHTMX)
	assert.Equal(t, http.StatusOK, wDBHTMX.Code)
	bodyDBHTMX := wDBHTMX.Body.String()
	assert.Contains(t, bodyDBHTMX, `id="metrics-card"`)
	assert.Contains(t, bodyDBHTMX, "Live Resource Usage")
}
