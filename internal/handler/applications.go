package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/caddy"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/applications"
)

type ApplicationsHandler struct {
	appRepo      *repo.ApplicationRepo
	envRepo      *repo.ProjectRepo
	deployRepo   *repo.DeploymentRepo
	envVarRepo   *repo.EnvVarRepo
	caddyClient  *caddy.Client
	settingRepo  *repo.SettingRepo
	dockerClient *docker.Client
}

func NewApplicationsHandler(appRepo *repo.ApplicationRepo, envRepo *repo.ProjectRepo, deployRepo *repo.DeploymentRepo, envVarRepo *repo.EnvVarRepo, caddyClient *caddy.Client, settingRepo *repo.SettingRepo, dockerClient *docker.Client) *ApplicationsHandler {
	return &ApplicationsHandler{
		appRepo:      appRepo,
		envRepo:      envRepo,
		deployRepo:   deployRepo,
		envVarRepo:   envVarRepo,
		caddyClient:  caddyClient,
		settingRepo:  settingRepo,
		dockerClient: dockerClient,
	}
}

func (h *ApplicationsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if r.URL.Path == "/applications" {
			h.List(w, r)
		} else {
			h.Get(w, r)
		}
	case http.MethodPost:
		h.Create(w, r)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ApplicationsHandler) List(w http.ResponseWriter, r *http.Request) {
	envID := chi.URLParam(r, "env_id")
	if envID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	page := parseInt(r.URL.Query().Get("page"), 1)
	search := r.URL.Query().Get("search")
	limit := 10
	offset := (page - 1) * limit

	var apps []*models.Application
	var total int
	var err error

	if search != "" {
		apps, total, err = h.appRepo.SearchPaginated(envID, search, limit, offset)
	} else {
		apps, total, err = h.appRepo.GetByEnvironmentIDPaginated(envID, limit, offset)
	}

	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	templ.Handler(layouts.AppLayout(applications.ApplicationsListPage(apps, page, totalPages, search))).ServeHTTP(w, r)
}

func (h *ApplicationsHandler) Create(w http.ResponseWriter, r *http.Request) {
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
	sourceType := r.FormValue("source_type")
	repoURL := r.FormValue("repository_url")
	branch := r.FormValue("branch")
	dockerImage := r.FormValue("docker_image")
	appPort, _ := strconv.Atoi(r.FormValue("app_port"))
	if appPort == 0 {
		appPort = 8080
	}

	slug := repo.GenerateSlug(name)

	var repoURLPtr *string
	if repoURL != "" {
		repoURLPtr = &repoURL
	}
	var branchPtr *string
	if branch != "" {
		branchPtr = &branch
	}
	var dockerImagePtr *string
	if dockerImage != "" {
		dockerImagePtr = &dockerImage
	}

	_, err := h.appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: envID,
		Name:          name,
		Slug:          slug,
		SourceType:    models.SourceType(sourceType),
		RepositoryURL: repoURLPtr,
		Branch:        branchPtr,
		DockerImage:   dockerImagePtr,
		AppPort:       appPort,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/projects/"+chi.URLParam(r, "project_id")+"/environments/"+envID+"/applications")
	w.WriteHeader(http.StatusOK)
}

func (h *ApplicationsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	deployments, _ := h.deployRepo.GetByApplicationID(id)
	envVars, _ := h.envVarRepo.GetByApplicationID(id)

	templ.Handler(layouts.AppLayout(applications.ApplicationDetailPage(app, deployments, envVars))).ServeHTTP(w, r)
}

func (h *ApplicationsHandler) Start(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if app.ContainerID != nil && *app.ContainerID != "" {
		running, err := h.dockerClient.IsContainerRunning(r.Context(), *app.ContainerID)
		if err != nil {
			slog.Error("failed to check container status", "error", err, "container_id", *app.ContainerID)
		} else if running {
			w.Header().Set("HX-Redirect", "/applications/"+id)
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	if err := h.appRepo.UpdateStatus(id, models.AppStatusRunning); err != nil {
		slog.Error("failed to update app status", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/applications/"+id)
	w.WriteHeader(http.StatusOK)
}

func (h *ApplicationsHandler) Stop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if app.ContainerID != nil && *app.ContainerID != "" {
		if err := h.dockerClient.StopContainer(r.Context(), *app.ContainerID, 10); err != nil {
			slog.Error("failed to stop container", "error", err, "container_id", *app.ContainerID)
		}
	}

	if err := h.appRepo.UpdateStatus(id, models.AppStatusStopped); err != nil {
		slog.Error("failed to update app status", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/applications/"+id)
	w.WriteHeader(http.StatusOK)
}

func (h *ApplicationsHandler) Restart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if app.ContainerID != nil && *app.ContainerID != "" {
		if err := h.dockerClient.RestartContainer(r.Context(), *app.ContainerID, 10); err != nil {
			slog.Error("failed to restart container", "error", err, "container_id", *app.ContainerID)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	if err := h.appRepo.UpdateStatus(id, models.AppStatusRunning); err != nil {
		slog.Error("failed to update app status", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/applications/"+id)
	w.WriteHeader(http.StatusOK)
}

func (h *ApplicationsHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	deployment, err := h.deployRepo.Create(models.CreateDeploymentInput{
		ApplicationID: app.ID,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	_ = h.appRepo.UpdateStatus(app.ID, models.AppStatusBuilding)
	_ = h.deployRepo.MarkStarted(deployment.ID)

	logPath := "/tmp/labuh-logs/" + deployment.ID + ".log"
	_ = h.deployRepo.UpdateStatus(deployment.ID, models.DeployStatusCloning)
	_ = logPath

	w.Header().Set("HX-Redirect", "/deployments/"+deployment.ID)
	w.WriteHeader(http.StatusOK)
}

func (h *ApplicationsHandler) CreateEnvVar(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")
	if appID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	key := r.FormValue("key")
	value := r.FormValue("value")
	isSecret := r.FormValue("is_secret") == "on"

	_, err := h.envVarRepo.Create(models.CreateAppEnvVarInput{
		ApplicationID: appID,
		Key:           key,
		Value:         value,
		IsSecret:      isSecret,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/applications/"+appID)
	w.WriteHeader(http.StatusOK)
}

func (h *ApplicationsHandler) DeleteEnvVar(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")
	varID := chi.URLParam(r, "var_id")
	if appID == "" || varID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	_ = h.envVarRepo.Delete(varID)
	w.Header().Set("HX-Redirect", "/applications/"+appID)
	w.WriteHeader(http.StatusOK)
}

func (h *ApplicationsHandler) Move(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	targetEnvID := r.FormValue("environment_id")
	if targetEnvID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := h.appRepo.Move(id, targetEnvID); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/applications/"+id)
	w.WriteHeader(http.StatusOK)
}

func (h *ApplicationsHandler) Clone(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	targetEnvID := r.FormValue("environment_id")
	newName := r.FormValue("name")
	if targetEnvID == "" || newName == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	clonedApp, err := h.appRepo.Clone(app, targetEnvID, newName)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	envVars, _ := h.envVarRepo.GetByApplicationID(id)
	for _, envVar := range envVars {
		_, err := h.envVarRepo.Create(models.CreateAppEnvVarInput{
			ApplicationID: clonedApp.ID,
			Key:           envVar.Key,
			Value:         envVar.Value,
			IsSecret:      envVar.IsSecret,
		})
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("HX-Redirect", "/applications/" + clonedApp.ID)
	w.WriteHeader(http.StatusOK)
}

func (h *ApplicationsHandler) SetDomain(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	domain := r.FormValue("custom_domain")
	if domain == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	domainPtr := &domain
	if err := h.appRepo.UpdateCustomDomain(id, domainPtr); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if h.caddyClient != nil {
		caddyAPIURL, _ := h.settingRepo.Get("caddy_api_url")
		caddyAPIKey, _ := h.settingRepo.Get("caddy_api_key")
		client := caddy.NewClient(caddyAPIURL, caddyAPIKey)

		_ = app
		_ = client
	}

	w.Header().Set("HX-Redirect", "/applications/"+id)
	w.WriteHeader(http.StatusOK)
}

func (h *ApplicationsHandler) RemoveDomain(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if err := h.appRepo.UpdateCustomDomain(id, nil); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if h.caddyClient != nil && app.CustomDomain != nil && *app.CustomDomain != "" {
		caddyAPIURL, _ := h.settingRepo.Get("caddy_api_url")
		caddyAPIKey, _ := h.settingRepo.Get("caddy_api_key")
		client := caddy.NewClient(caddyAPIURL, caddyAPIKey)

		_ = client
	}

	w.Header().Set("HX-Redirect", "/applications/"+id)
	w.WriteHeader(http.StatusOK)
}
