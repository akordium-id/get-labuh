package handler

import (
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/templates"
)

type TemplatesHandler struct {
	templateRepo *repo.TemplateRepo
	appRepo      *repo.ApplicationRepo
	projectRepo  *repo.ProjectRepo
	deployRepo   *repo.DeploymentRepo
	envVarRepo   *repo.EnvVarRepo
}

func NewTemplatesHandler(templateRepo *repo.TemplateRepo, appRepo *repo.ApplicationRepo, projectRepo *repo.ProjectRepo, deployRepo *repo.DeploymentRepo, envVarRepo *repo.EnvVarRepo) *TemplatesHandler {
	return &TemplatesHandler{
		templateRepo: templateRepo,
		appRepo:      appRepo,
		projectRepo:  projectRepo,
		deployRepo:   deployRepo,
		envVarRepo:   envVarRepo,
	}
}

func (h *TemplatesHandler) List(w http.ResponseWriter, r *http.Request) {
	templateList, err := h.templateRepo.GetAll()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	templ.Handler(layouts.AppLayout(templates.TemplatesListPage(templateList))).ServeHTTP(w, r)
}

func (h *TemplatesHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	template, err := h.templateRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	variables, _ := h.templateRepo.GetVariablesByTemplateID(id)

	templ.Handler(layouts.AppLayout(templates.TemplateDetailPage(template, variables))).ServeHTTP(w, r)
}

func (h *TemplatesHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	template, err := h.templateRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	envID := r.FormValue("environment_id")
	appName := r.FormValue("name")
	if appName == "" {
		appName = template.Name
	}

	slug := repo.GenerateSlug(appName)

	var repoURLPtr *string
	if template.RepositoryURL != nil {
		repoURLPtr = template.RepositoryURL
	}
	var dockerImagePtr *string
	if template.DockerImage != nil {
		dockerImagePtr = template.DockerImage
	}

	appPort := 8080
	if portStr := r.FormValue("app_port"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			appPort = port
		}
	}

	app, err := h.appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: envID,
		Name:          appName,
		Slug:          slug,
		SourceType:    models.SourceType(template.SourceType),
		RepositoryURL: repoURLPtr,
		DockerImage:   dockerImagePtr,
		AppPort:       appPort,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	_, _ = h.deployRepo.Create(models.CreateDeploymentInput{
		ApplicationID: app.ID,
	})

	w.Header().Set("HX-Redirect", "/applications/"+app.ID)
	w.WriteHeader(http.StatusOK)
}
