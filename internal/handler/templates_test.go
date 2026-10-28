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

func TestTemplatesHandler_Flow(t *testing.T) {
	db := itesting.SetupTestDB(t)
	templateRepo := repo.NewTemplateRepo(db)
	appRepo := repo.NewApplicationRepo(db)
	projectRepo := repo.NewProjectRepo(db)
	deployRepo := repo.NewDeploymentRepo(db)
	envVarRepo := repo.NewEnvVarRepo(db)

	p, err := projectRepo.Create(models.CreateProjectInput{Name: "Main Project", Slug: "main-proj"})
	require.NoError(t, err)
	env, err := projectRepo.CreateEnvironment(p.ID, "Production", "prod")
	require.NoError(t, err)

	handler := NewTemplatesHandler(templateRepo, appRepo, projectRepo, deployRepo, envVarRepo)

	// 1. List Templates
	req := httptest.NewRequest(http.MethodGet, "/templates", nil)
	rec := httptest.NewRecorder()
	handler.List(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Service Catalog")
	assert.Contains(t, body, "Redis")
	assert.Contains(t, body, "MinIO")
	assert.Contains(t, body, "Meilisearch")

	// 2. Get MinIO Detail
	req = httptest.NewRequest(http.MethodGet, "/templates/tpl-minio", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "tpl-minio")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec = httptest.NewRecorder()
	handler.Get(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	body = rec.Body.String()
	assert.Contains(t, body, "MinIO")
	assert.Contains(t, body, "MINIO_ROOT_USER")
	assert.Contains(t, body, "minioadmin")

	// 3. Deploy MinIO Template
	form := url.Values{}
	form.Set("environment_id", env.ID)
	form.Set("name", "Custom MinIO S3")
	form.Set("app_port", "9000")
	form.Set("var_MINIO_ROOT_USER", "myadmin")
	form.Set("var_MINIO_ROOT_PASSWORD", "mysecretpassword123")

	req = httptest.NewRequest(http.MethodPost, "/templates/tpl-minio/deploy", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec = httptest.NewRecorder()
	handler.Deploy(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code)
	redirectURL := rec.Header().Get("Location")
	assert.True(t, strings.HasPrefix(redirectURL, "/applications/"))

	appID := strings.TrimPrefix(redirectURL, "/applications/")
	app, err := appRepo.GetByID(appID)
	require.NoError(t, err)
	assert.Equal(t, "Custom MinIO S3", app.Name)
	assert.Equal(t, 9000, app.AppPort)
	require.NotNil(t, app.DockerImage)
	assert.Equal(t, "minio/minio:latest", *app.DockerImage)

	// Check environment variables saved
	vars, err := envVarRepo.GetByApplicationID(appID)
	require.NoError(t, err)
	var rootUser, rootPass string
	for _, v := range vars {
		switch v.Key {
		case "MINIO_ROOT_USER":
			rootUser = v.Value
		case "MINIO_ROOT_PASSWORD":
			rootPass = v.Value
		}
	}
	assert.Equal(t, "myadmin", rootUser)
	assert.Equal(t, "mysecretpassword123", rootPass)
}
