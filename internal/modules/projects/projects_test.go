package projects

import (
	"context"
	"net/http"
	"net/http/httptest"
	stdtesting "testing"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/stretchr/testify/assert"
)

type mockUserProvider struct{}

func (m *mockUserProvider) GetAll() ([]*models.User, error) {
	return []*models.User{
		{ID: "u1", Name: "Admin User", Email: "admin@example.com"},
	}, nil
}

func setupTestModule(t *stdtesting.T) (*Module, *Handler) {
	t.Helper()
	db, err := database.Connect("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if err := database.RunAllMigrations(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	mod := New(db, &mockUserProvider{})
	return mod, mod.Handler
}

func TestProjectsHandler_List(t *stdtesting.T) {
	mod, h := setupTestModule(t)
	_, _ = mod.ProjectRepo.Create(models.CreateProjectInput{Name: "P1", Slug: "p1"})
	_, _ = mod.ProjectRepo.Create(models.CreateProjectInput{Name: "P2", Slug: "p2"})

	// 1. Standard request returns full page with layout
	req := httptest.NewRequest(http.MethodGet, "/projects", nil)
	w := httptest.NewRecorder()
	h.ProjectsList(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body := w.Body.String()
	assert.Contains(t, body, "<!doctype html>")
	assert.Contains(t, body, "projects-container")
	assert.Contains(t, body, "P1")
	assert.Contains(t, body, "P2")

	// 2. HTMX search request returns only table partial
	reqHTMX := httptest.NewRequest(http.MethodGet, "/projects?search=P1", nil)
	reqHTMX.Header.Set("HX-Request", "true")
	wHTMX := httptest.NewRecorder()
	h.ProjectsList(wHTMX, reqHTMX)

	respHTMX := wHTMX.Result()
	assert.Equal(t, http.StatusOK, respHTMX.StatusCode)
	bodyHTMX := wHTMX.Body.String()
	assert.NotContains(t, bodyHTMX, "<!doctype html>")
	assert.NotContains(t, bodyHTMX, "<aside")
	assert.Contains(t, bodyHTMX, "P1")
	assert.NotContains(t, bodyHTMX, "P2")
}

func TestProjectsHandler_Create(t *stdtesting.T) {
	_, h := setupTestModule(t)

	req := httptest.NewRequest(http.MethodPost, "/projects", nil)
	req.PostForm = map[string][]string{
		"name": {"New Project"},
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.ProjectsCreate(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/projects", resp.Header.Get("HX-Redirect"))
}

func TestProjectsHandler_Get(t *stdtesting.T) {
	mod, h := setupTestModule(t)
	project, _ := mod.ProjectRepo.Create(models.CreateProjectInput{Name: "Get Project", Slug: "get-project"})

	req := httptest.NewRequest(http.MethodGet, "/projects/"+project.ID, nil)
	w := httptest.NewRecorder()

	h.ProjectsGet(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestProjectsHandler_Delete(t *stdtesting.T) {
	mod, h := setupTestModule(t)
	project, _ := mod.ProjectRepo.Create(models.CreateProjectInput{Name: "Delete Project", Slug: "delete-project"})

	req := httptest.NewRequest(http.MethodPost, "/projects/"+project.ID, nil)
	req.PostForm = map[string][]string{
		"_method": {"delete"},
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.ProjectsDelete(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "/projects", resp.Header.Get("HX-Redirect"))
}

func TestTeams_CRUD(t *stdtesting.T) {
	mod, h := setupTestModule(t)

	// Create Team
	req := httptest.NewRequest(http.MethodPost, "/settings/teams", nil)
	req.PostForm = map[string][]string{
		"name": {"Alpha Team"},
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.TeamsCreate(w, req)
	assert.Equal(t, http.StatusOK, w.Result().StatusCode)

	teams, err := mod.TeamRepo.GetAllWithContext(context.Background())
	assert.NoError(t, err)
	assert.Len(t, teams, 1)
	assert.Equal(t, "Alpha Team", teams[0].Name)

	// List Teams
	reqList := httptest.NewRequest(http.MethodGet, "/settings/teams", nil)
	wList := httptest.NewRecorder()
	h.TeamsList(wList, reqList)
	assert.Equal(t, http.StatusOK, wList.Result().StatusCode)
}
