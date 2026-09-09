package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/worker"
)

type ProjectsHandler struct {
	projectRepo *repo.ProjectRepo
}

func NewProjectsHandler(projectRepo *repo.ProjectRepo) *ProjectsHandler {
	return &ProjectsHandler{projectRepo: projectRepo}
}

func (h *ProjectsHandler) List(w http.ResponseWriter, r *http.Request) {
	projects, err := h.projectRepo.GetAll()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	writeJSONData(w, http.StatusOK, projects)
}

func (h *ProjectsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string  `json:"name"`
		Slug        string  `json:"slug"`
		Description *string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	project, err := h.projectRepo.Create(models.CreateProjectInput{
		Name:        input.Name,
		Slug:        input.Slug,
		Description: input.Description,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to create project")
		return
	}

	writeJSONData(w, http.StatusCreated, project)
}

func (h *ProjectsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "missing project id")
		return
	}

	project, err := h.projectRepo.GetByID(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "project not found")
		return
	}

	writeJSONData(w, http.StatusOK, project)
}

func (h *ProjectsHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "missing project id")
		return
	}

	_ = id
	writeJSONData(w, http.StatusOK, []any{})
}

type ApplicationsHandler struct {
	appRepo      *repo.ApplicationRepo
	projectRepo  *repo.ProjectRepo
	deployRepo   *repo.DeploymentRepo
	envVarRepo   *repo.EnvVarRepo
	deployWorker *worker.DeployWorker
}

func NewApplicationsHandler(appRepo *repo.ApplicationRepo, projectRepo *repo.ProjectRepo, deployRepo *repo.DeploymentRepo) *ApplicationsHandler {
	return &ApplicationsHandler{
		appRepo:     appRepo,
		projectRepo: projectRepo,
		deployRepo:  deployRepo,
	}
}

func (h *ApplicationsHandler) SetDeployWorker(w *worker.DeployWorker) {
	h.deployWorker = w
}

func (h *ApplicationsHandler) SetEnvVarRepo(r *repo.EnvVarRepo) {
	h.envVarRepo = r
}

func (h *ApplicationsHandler) ListByProject(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")
	if projectID == "" {
		writeJSONError(w, http.StatusBadRequest, "missing project id")
		return
	}

	_ = projectID
	writeJSONData(w, http.StatusOK, []any{})
}

func (h *ApplicationsHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "missing application id")
		return
	}

	app, err := h.appRepo.GetByID(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "application not found")
		return
	}

	logPath := "/tmp/labuh-logs/"
	deployment, err := h.deployRepo.Create(models.CreateDeploymentInput{
		ApplicationID: app.ID,
		LogPath:       &logPath,
	})
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to create deployment")
		return
	}

	fullLogPath := "/tmp/labuh-logs/" + deployment.ID + ".log"
	_ = h.appRepo.UpdateStatus(app.ID, models.AppStatusBuilding)
	_ = h.deployRepo.MarkStarted(deployment.ID)
	_ = h.deployRepo.UpdateStatus(deployment.ID, models.DeployStatusQueued)

	if h.deployWorker != nil {
		var envVars []*models.AppEnvVar
		if h.envVarRepo != nil {
			envVars, _ = h.envVarRepo.GetByApplicationID(app.ID)
		}
		job := worker.BuildDeploymentJob(app, deployment.ID, envVars)
		job.LogPath = fullLogPath
		_ = h.deployWorker.Enqueue(job)
	}

	writeJSONData(w, http.StatusAccepted, map[string]string{"deployment_id": deployment.ID})
}

func (h *ApplicationsHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "missing application id")
		return
	}

	app, err := h.appRepo.GetByID(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "application not found")
		return
	}

	writeJSONData(w, http.StatusOK, map[string]any{
		"id":     app.ID,
		"name":   app.Name,
		"status": app.Status,
	})
}

type DeploymentsHandler struct {
	deployRepo *repo.DeploymentRepo
	appRepo    *repo.ApplicationRepo
}

func NewDeploymentsHandler(deployRepo *repo.DeploymentRepo, appRepo *repo.ApplicationRepo) *DeploymentsHandler {
	return &DeploymentsHandler{deployRepo: deployRepo, appRepo: appRepo}
}

func (h *DeploymentsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "missing deployment id")
		return
	}

	deployment, err := h.deployRepo.GetByID(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "deployment not found")
		return
	}

	writeJSONData(w, http.StatusOK, deployment)
}

func writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := APIResponse{Error: message}
	_ = json.NewEncoder(w).Encode(response)
}

func writeJSONData(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	response := APIResponse{Data: data}
	_ = json.NewEncoder(w).Encode(response)
}
