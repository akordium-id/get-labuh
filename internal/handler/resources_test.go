package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	itesting "github.com/akordium-id/get-labuh/internal/testing"
)

func TestApplicationsHandler_UpdateResourceLimits(t *testing.T) {
	db := itesting.SetupTestDB(t)
	appRepo := repo.NewApplicationRepo(db)
	projectRepo := repo.NewProjectRepo(db)

	p, err := projectRepo.Create(models.CreateProjectInput{Name: "P1", Slug: "p1"})
	require.NoError(t, err)
	env, err := projectRepo.CreateEnvironment(p.ID, "Production", "prod")
	require.NoError(t, err)

	app, err := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "Web App",
		Slug:          "web-app",
		SourceType:    models.SourceTypeDockerImage,
	})
	require.NoError(t, err)

	handler := NewApplicationsHandler(appRepo, projectRepo, nil, nil, nil, nil, nil)

	form := url.Values{}
	form.Set("cpu_limit", "1.5")
	form.Set("memory_limit", "512")
	form.Set("memory_reservation", "256")

	req := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/resources", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", app.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rec := httptest.NewRecorder()
	handler.UpdateResourceLimits(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/applications/"+app.ID, rec.Header().Get("Location"))

	// Check DB
	updated, err := appRepo.GetByID(app.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.CPULimit)
	assert.Equal(t, 1.5, *updated.CPULimit)
	require.NotNil(t, updated.MemoryLimit)
	assert.Equal(t, int64(512), *updated.MemoryLimit)
	require.NotNil(t, updated.MemoryReservation)
	assert.Equal(t, int64(256), *updated.MemoryReservation)
}

func TestDatabasesHandler_UpdateResourceLimits(t *testing.T) {
	db := itesting.SetupTestDB(t)
	dbRepo := repo.NewDatabaseRepo(db)
	projectRepo := repo.NewProjectRepo(db)

	p, err := projectRepo.Create(models.CreateProjectInput{Name: "P1", Slug: "p1"})
	require.NoError(t, err)
	env, err := projectRepo.CreateEnvironment(p.ID, "Production", "prod")
	require.NoError(t, err)

	dbService, err := dbRepo.Create(models.CreateDatabaseServiceInput{
		EnvironmentID: env.ID,
		Name:          "Primary Postgres",
		Slug:          "primary-postgres",
		Engine:        models.DatabaseEnginePostgres,
	})
	require.NoError(t, err)

	handler := NewDatabasesHandler(dbRepo, projectRepo)

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
	handler.UpdateResourceLimits(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/databases/"+dbService.ID, rec.Header().Get("Location"))

	// Check DB
	updated, err := dbRepo.GetByID(dbService.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.CPULimit)
	assert.Equal(t, 0.75, *updated.CPULimit)
	require.NotNil(t, updated.MemoryLimit)
	assert.Equal(t, int64(1024), *updated.MemoryLimit)
	require.NotNil(t, updated.MemoryReservation)
	assert.Equal(t, int64(512), *updated.MemoryReservation)
}
