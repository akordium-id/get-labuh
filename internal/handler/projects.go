package handler

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/faiq/labuh/internal/database/repo"
	"github.com/faiq/labuh/internal/models"
	"github.com/faiq/labuh/internal/web/layouts"
	"github.com/faiq/labuh/internal/web/pages/projects"
)

type ProjectsHandler struct {
	projectRepo *repo.ProjectRepo
}

func NewProjectsHandler(projectRepo *repo.ProjectRepo) *ProjectsHandler {
	return &ProjectsHandler{
		projectRepo: projectRepo,
	}
}

func (h *ProjectsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if r.URL.Path == "/projects" {
			h.List(w, r)
		} else {
			h.Get(w, r)
		}
	case http.MethodPost:
		if r.URL.Path == "/projects" {
			h.Create(w, r)
		} else {
			h.Delete(w, r)
		}
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ProjectsHandler) List(w http.ResponseWriter, r *http.Request) {
	projectList, err := h.projectRepo.GetAll()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	envCounts := make(map[string]int)
	for _, p := range projectList {
		envs, _ := h.projectRepo.GetEnvironmentsByProjectID(p.ID)
		envCounts[p.ID] = len(envs)
	}

	templ.Handler(layouts.AppLayout(projects.ProjectsListPage(projectList, envCounts))).ServeHTTP(w, r)
}

func (h *ProjectsHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	description := r.FormValue("description")
	var descPtr *string
	if description != "" {
		descPtr = &description
	}

	slug := repo.GenerateSlug(name)

	_, err := h.projectRepo.Create(models.CreateProjectInput{
		Name:        name,
		Slug:        slug,
		Description: descPtr,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/projects")
	w.WriteHeader(http.StatusOK)
}

func (h *ProjectsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	project, err := h.projectRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	_ = project

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("<div>Project detail placeholder</div>"))
}

func (h *ProjectsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := h.projectRepo.Delete(id); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/projects")
	w.WriteHeader(http.StatusOK)
}
