package handler

import (
	"net/http"
	"net/http/httptest"
	stdtesting "testing"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/stretchr/testify/assert"
)

func setupProjectsHandler(t *stdtesting.T) (*ProjectsHandler, *repo.ProjectRepo) {
	t.Helper()
	db, err := database.Connect("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if err := database.RunAllMigrations(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	projectRepo := repo.NewProjectRepo(db)
	return NewProjectsHandler(projectRepo), projectRepo
}

func TestProjectsHandler_List(t *stdtesting.T) {
	h, _ := setupProjectsHandler(t)
	h.projectRepo.Create(models.CreateProjectInput{Name: "P1", Slug: "p1"})
	h.projectRepo.Create(models.CreateProjectInput{Name: "P2", Slug: "p2"})

	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	w := httptest.NewRecorder()

	h.List(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestProjectsHandler_Create(t *stdtesting.T) {
	h, _ := setupProjectsHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/projects", nil)
	req.PostForm = map[string][]string{
		"name": {"New Project"},
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.Create(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/projects", resp.Header.Get("HX-Redirect"))
}

func TestProjectsHandler_Get(t *stdtesting.T) {
	h, projectRepo := setupProjectsHandler(t)
	project, _ := projectRepo.Create(models.CreateProjectInput{Name: "Get Project", Slug: "get-project"})

	req := httptest.NewRequest(http.MethodGet, "/projects/"+project.ID, nil)
	w := httptest.NewRecorder()

	h.Get(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestProjectsHandler_Delete(t *stdtesting.T) {
	h, projectRepo := setupProjectsHandler(t)
	project, _ := projectRepo.Create(models.CreateProjectInput{Name: "Delete Project", Slug: "delete-project"})

	req := httptest.NewRequest(http.MethodPost, "/projects/"+project.ID, nil)
	req.PostForm = map[string][]string{
		"_method": {"delete"},
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.Delete(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/projects", resp.Header.Get("HX-Redirect"))
}
