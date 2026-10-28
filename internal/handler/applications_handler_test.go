package handler

import (
	"net/http"
	"net/http/httptest"
	stdtesting "testing"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/caddy"
	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/stretchr/testify/assert"
)

func setupApplicationsHandler(t *stdtesting.T) (*ApplicationsHandler, *repo.ApplicationRepo, *repo.ProjectRepo) {
	t.Helper()
	db, err := database.Connect("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if err := database.RunAllMigrations(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	appRepo := repo.NewApplicationRepo(db)
	projectRepo := repo.NewProjectRepo(db)
	deployRepo := repo.NewDeploymentRepo(db)
	envVarRepo := repo.NewEnvVarRepo(db)
	settingRepo := repo.NewSettingRepo(db)

	dockerClient, _ := docker.NewClient()
	caddyClient := caddy.NewClient("http://localhost:2019", "")
	h := NewApplicationsHandler(appRepo, projectRepo, deployRepo, envVarRepo, caddyClient, settingRepo, dockerClient)
	return h, appRepo, projectRepo
}

func TestApplicationsHandler_Create(t *stdtesting.T) {
	h, _, projectRepo := setupApplicationsHandler(t)
	project, _ := projectRepo.Create(models.CreateProjectInput{Name: "P", Slug: "p"})
	env, _ := projectRepo.CreateEnvironment(project.ID, "Prod", "prod")

	req := httptest.NewRequest(http.MethodPost, "/environments/"+env.ID+"/applications", nil)
	req.PostForm = map[string][]string{
		"name":        {"Test App"},
		"source_type": {"git"},
		"app_port":    {"8080"},
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/environments/{env_id}/applications", h.ServeHTTP)
	r.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestApplicationsHandler_List(t *stdtesting.T) {
	h, appRepo, projectRepo := setupApplicationsHandler(t)
	project, _ := projectRepo.Create(models.CreateProjectInput{Name: "P", Slug: "p"})
	env, _ := projectRepo.CreateEnvironment(project.ID, "Prod", "prod")

	_, _ = appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "Web App",
		Slug:          "web-app",
		SourceType:    models.SourceTypeGit,
	})
	_, _ = appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "Worker App",
		Slug:          "worker-app",
		SourceType:    models.SourceTypeGit,
	})

	r := chi.NewRouter()
	r.Get("/projects/{project_id}/environments/{env_id}/applications", h.List)

	// 1. Full page
	req := httptest.NewRequest(http.MethodGet, "/projects/"+project.ID+"/environments/"+env.ID+"/applications", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "<!doctype html>")
	assert.Contains(t, body, "applications-container")
	assert.Contains(t, body, "Web App")
	assert.Contains(t, body, "Worker App")

	// 2. HTMX search partial
	reqHTMX := httptest.NewRequest(http.MethodGet, "/projects/"+project.ID+"/environments/"+env.ID+"/applications?search=Web", nil)
	reqHTMX.Header.Set("HX-Request", "true")
	wHTMX := httptest.NewRecorder()
	r.ServeHTTP(wHTMX, reqHTMX)

	assert.Equal(t, http.StatusOK, wHTMX.Code)
	bodyHTMX := wHTMX.Body.String()
	assert.NotContains(t, bodyHTMX, "<!doctype html>")
	assert.Contains(t, bodyHTMX, "Web App")
	assert.NotContains(t, bodyHTMX, "Worker App")
}

func TestApplicationsHandler_Start(t *stdtesting.T) {
	h, appRepo, projectRepo := setupApplicationsHandler(t)
	project, _ := projectRepo.Create(models.CreateProjectInput{Name: "P", Slug: "p"})
	env, _ := projectRepo.CreateEnvironment(project.ID, "Prod", "prod")
	app, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "Start App",
		Slug:          "start-app",
		SourceType:    models.SourceTypeGit,
	})

	req := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/start", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/applications/{id}/start", h.ServeHTTP)
	r.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestApplicationsHandler_Stop(t *stdtesting.T) {
	h, appRepo, projectRepo := setupApplicationsHandler(t)
	project, _ := projectRepo.Create(models.CreateProjectInput{Name: "P", Slug: "p"})
	env, _ := projectRepo.CreateEnvironment(project.ID, "Prod", "prod")
	app, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "Stop App",
		Slug:          "stop-app",
		SourceType:    models.SourceTypeGit,
	})

	req := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/stop", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/applications/{id}/stop", h.ServeHTTP)
	r.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestApplicationsHandler_Restart(t *stdtesting.T) {
	h, appRepo, projectRepo := setupApplicationsHandler(t)
	project, _ := projectRepo.Create(models.CreateProjectInput{Name: "P", Slug: "p"})
	env, _ := projectRepo.CreateEnvironment(project.ID, "Prod", "prod")
	app, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "Restart App",
		Slug:          "restart-app",
		SourceType:    models.SourceTypeGit,
	})

	req := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/restart", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/applications/{id}/restart", h.ServeHTTP)
	r.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestApplicationsHandler_SetDomain(t *stdtesting.T) {
	h, appRepo, projectRepo := setupApplicationsHandler(t)
	project, _ := projectRepo.Create(models.CreateProjectInput{Name: "P", Slug: "p"})
	env, _ := projectRepo.CreateEnvironment(project.ID, "Prod", "prod")
	app, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "Domain App",
		Slug:          "domain-app",
		SourceType:    models.SourceTypeGit,
	})

	req := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/domain", nil)
	req.PostForm = map[string][]string{
		"custom_domain": {"example.com"},
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/applications/{id}/domain", h.ServeHTTP)
	r.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestApplicationsHandler_RemoveDomain(t *stdtesting.T) {
	h, appRepo, projectRepo := setupApplicationsHandler(t)
	project, _ := projectRepo.Create(models.CreateProjectInput{Name: "P", Slug: "p"})
	env, _ := projectRepo.CreateEnvironment(project.ID, "Prod", "prod")
	app, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "Remove Domain App",
		Slug:          "remove-domain-app",
		SourceType:    models.SourceTypeGit,
		CustomDomain:  new("example.com"),
	})

	req := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/domain/remove", nil)
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/applications/{id}/domain/remove", h.ServeHTTP)
	r.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestApplicationsHandler_CreateEnvVar(t *stdtesting.T) {
	h, appRepo, projectRepo := setupApplicationsHandler(t)
	project, _ := projectRepo.Create(models.CreateProjectInput{Name: "P", Slug: "p"})
	env, _ := projectRepo.CreateEnvironment(project.ID, "Prod", "prod")
	app, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "EnvVar App",
		Slug:          "envvar-app",
		SourceType:    models.SourceTypeGit,
	})

	req := httptest.NewRequest(http.MethodPost, "/applications/"+app.ID+"/env-vars", nil)
	req.PostForm = map[string][]string{
		"key":   {"API_KEY"},
		"value": {"secret"},
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	r.Post("/applications/{id}/env-vars", h.ServeHTTP)
	r.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestApplicationsHandler_Status(t *stdtesting.T) {
	h, appRepo, projectRepo := setupApplicationsHandler(t)
	project, _ := projectRepo.Create(models.CreateProjectInput{Name: "P", Slug: "p"})
	env, _ := projectRepo.CreateEnvironment(project.ID, "Prod", "prod")
	app, _ := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "Status App",
		Slug:          "status-app",
		SourceType:    models.SourceTypeGit,
	})

	r := chi.NewRouter()
	r.Get("/applications/{id}/status", h.Status)

	req := httptest.NewRequest(http.MethodGet, "/applications/"+app.ID+"/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `id="status-badge"`)
	assert.Contains(t, body, `/applications/`+app.ID+`/status`)
}

