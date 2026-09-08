package handler

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/compose"
)

type ComposeHandler struct {
	composeRepo *repo.ComposeRepo
	envRepo     *repo.ProjectRepo
}

func NewComposeHandler(composeRepo *repo.ComposeRepo, envRepo *repo.ProjectRepo) *ComposeHandler {
	return &ComposeHandler{
		composeRepo: composeRepo,
		envRepo:     envRepo,
	}
}

func (h *ComposeHandler) List(w http.ResponseWriter, r *http.Request) {
	envID := chi.URLParam(r, "env_id")
	if envID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	composes, err := h.composeRepo.GetByEnvironmentID(envID)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	templ.Handler(layouts.AppLayout(compose.ComposeListPage(composes))).ServeHTTP(w, r)
}

func (h *ComposeHandler) Create(w http.ResponseWriter, r *http.Request) {
	envID := chi.URLParam(r, "env_id")
	if envID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	slug := r.FormValue("slug")
	composeFilePath := r.FormValue("compose_file_path")
	composeProjectName := r.FormValue("compose_project_name")
	customDomain := r.FormValue("custom_domain")

	if slug == "" {
		slug = repo.GenerateSlug(name)
	}
	if composeFilePath == "" {
		composeFilePath = "docker-compose.yml"
	}

	var composeProjectNamePtr *string
	if composeProjectName != "" {
		composeProjectNamePtr = &composeProjectName
	}
	var customDomainPtr *string
	if customDomain != "" {
		customDomainPtr = &customDomain
	}

	_, err := h.composeRepo.Create(models.CreateComposeApplicationInput{
		EnvironmentID:      envID,
		Name:              name,
		Slug:              slug,
		ComposeFilePath:    composeFilePath,
		ComposeProjectName: composeProjectNamePtr,
		CustomDomain:       customDomainPtr,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/projects/"+chi.URLParam(r, "project_id")+"/environments/"+envID+"/compose-apps")
	w.WriteHeader(http.StatusOK)
}

func (h *ComposeHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	composeApp, err := h.composeRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	templ.Handler(layouts.AppLayout(compose.ComposeDetailPage(composeApp))).ServeHTTP(w, r)
}

func (h *ComposeHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	composeApp, err := h.composeRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	_ = h.composeRepo.UpdateStatus(id, models.ComposeStatusBuilding)

	logPath := "/tmp/labuh-logs/" + id + ".log"

	_ = composeApp
	_ = logPath

	w.Header().Set("HX-Redirect", "/compose-apps/"+id)
	w.WriteHeader(http.StatusOK)
}

func (h *ComposeHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	_ = h.composeRepo.UpdateStatus(id, models.ComposeStatusStopped)
	w.Header().Set("HX-Redirect", "/compose-apps/"+id)
	w.WriteHeader(http.StatusOK)
}

func (h *ComposeHandler) Start(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	_ = h.composeRepo.UpdateStatus(id, models.ComposeStatusRunning)
	w.Header().Set("HX-Redirect", "/compose-apps/"+id)
	w.WriteHeader(http.StatusOK)
}
