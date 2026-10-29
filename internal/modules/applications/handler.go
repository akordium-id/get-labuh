package applications

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/caddy"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/components"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	webapp "github.com/akordium-id/get-labuh/internal/web/pages/applications"
)

// Handler menangani operasi HTTP untuk domain Applications.
type Handler struct {
	appRepo      *Repo
	envVarRepo   *EnvVarRepo
	envRepo      EnvironmentProvider
	deployRepo   DeploymentCreator
	settingRepo  SettingProvider
	caddyClient  *caddy.Client
	dockerClient *docker.Client
	deployWorker DeployEnqueuer
}

// NewHandler membuat instance baru Handler applications.
func NewHandler(
	appRepo *Repo,
	envVarRepo *EnvVarRepo,
	envRepo EnvironmentProvider,
	deployRepo DeploymentCreator,
	settingRepo SettingProvider,
	caddyClient *caddy.Client,
	dockerClient *docker.Client,
) *Handler {
	return &Handler{
		appRepo:      appRepo,
		envVarRepo:   envVarRepo,
		envRepo:      envRepo,
		deployRepo:   deployRepo,
		settingRepo:  settingRepo,
		caddyClient:  caddyClient,
		dockerClient: dockerClient,
	}
}

// SetDeployWorker menyetel worker antrean deployment.
func (h *Handler) SetDeployWorker(w DeployEnqueuer) {
	h.deployWorker = w
}

// SetDockerClient menyetel docker client.
func (h *Handler) SetDockerClient(c *docker.Client) {
	h.dockerClient = c
}

// SetCaddyClient menyetel caddy client.
func (h *Handler) SetCaddyClient(c *caddy.Client) {
	h.caddyClient = c
}

// List menampilkan daftar aplikasi pada suatu environment.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
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
		apps, total, err = h.appRepo.SearchPaginatedWithContext(r.Context(), envID, search, limit, offset)
	} else {
		apps, total, err = h.appRepo.GetByEnvironmentIDPaginatedWithContext(r.Context(), envID, limit, offset)
	}

	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	projectID := chi.URLParam(r, "project_id")
	baseURL := "/projects/" + projectID + "/environments/" + envID + "/applications"
	if projectID == "" {
		baseURL = "/environments/" + envID + "/applications"
	}

	if r.Header.Get("HX-Request") == "true" {
		templ.Handler(webapp.ApplicationsTable(apps, page, totalPages, search, baseURL)).ServeHTTP(w, r)
		return
	}

	templ.Handler(layouts.AppLayout(webapp.ApplicationsListPage(apps, page, totalPages, search, baseURL))).ServeHTTP(w, r)
}

// Create membuat aplikasi baru.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
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

	slug := GenerateSlug(name)

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

	_, err := h.appRepo.CreateWithContext(r.Context(), models.CreateApplicationInput{
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

// Get menampilkan detail aplikasi.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	var deployments []*models.Deployment
	// Deployments bisa kosong bila belum pernah deploy
	envVars, _ := h.envVarRepo.GetByApplicationIDWithContext(r.Context(), id)

	templ.Handler(layouts.AppLayout(webapp.ApplicationDetailPage(app, deployments, envVars))).ServeHTTP(w, r)
}

// Start menjalankan kontainer aplikasi.
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if app.ContainerID != nil && *app.ContainerID != "" && h.dockerClient != nil {
		running, err := h.dockerClient.IsContainerRunning(r.Context(), *app.ContainerID)
		if err != nil {
			slog.Error("failed to check container status", "error", err, "container_id", *app.ContainerID)
		} else if running {
			w.Header().Set("HX-Redirect", "/applications/"+id)
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	if err := h.appRepo.UpdateStatusWithContext(r.Context(), id, models.AppStatusRunning); err != nil {
		slog.Error("failed to update app status", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/applications/"+id)
	w.WriteHeader(http.StatusOK)
}

// Stop menghentikan kontainer aplikasi.
func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if app.ContainerID != nil && *app.ContainerID != "" && h.dockerClient != nil {
		if err := h.dockerClient.StopContainer(r.Context(), *app.ContainerID, 10); err != nil {
			slog.Error("failed to stop container", "error", err, "container_id", *app.ContainerID)
		}
	}

	if err := h.appRepo.UpdateStatusWithContext(r.Context(), id, models.AppStatusStopped); err != nil {
		slog.Error("failed to update app status", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/applications/"+id)
	w.WriteHeader(http.StatusOK)
}

// Restart me-restart kontainer aplikasi.
func (h *Handler) Restart(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if app.ContainerID != nil && *app.ContainerID != "" && h.dockerClient != nil {
		if err := h.dockerClient.RestartContainer(r.Context(), *app.ContainerID, 10); err != nil {
			slog.Error("failed to restart container", "error", err, "container_id", *app.ContainerID)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	if err := h.appRepo.UpdateStatusWithContext(r.Context(), id, models.AppStatusRunning); err != nil {
		slog.Error("failed to update app status", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/applications/"+id)
	w.WriteHeader(http.StatusOK)
}

// Deploy memicu pembuatan deployment dan memasukkannya ke antrean worker.
func (h *Handler) Deploy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if h.deployRepo == nil {
		http.Error(w, "Deployment creator not configured", http.StatusInternalServerError)
		return
	}

	deployment, err := h.deployRepo.CreateWithContext(r.Context(), models.CreateDeploymentInput{
		ApplicationID: app.ID,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	logPath := "/tmp/labuh-logs/" + deployment.ID + ".log"
	_ = h.appRepo.UpdateStatusWithContext(r.Context(), app.ID, models.AppStatusBuilding)

	if h.deployWorker != nil {
		var envVars []*models.AppEnvVar
		if h.envVarRepo != nil {
			envVars, _ = h.envVarRepo.GetByApplicationIDWithContext(r.Context(), app.ID)
		}
		job := BuildDeploymentJob(app, deployment.ID, envVars)
		job.LogPath = logPath
		if err := h.deployWorker.Enqueue(job); err != nil {
			slog.Error("failed to enqueue deployment job", "error", err, "deployment_id", deployment.ID)
		}
	}

	w.Header().Set("HX-Redirect", "/deployments/"+deployment.ID)
	w.WriteHeader(http.StatusOK)
}

// CreateEnvVar menambahkan variabel lingkungan ke aplikasi.
func (h *Handler) CreateEnvVar(w http.ResponseWriter, r *http.Request) {
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

	_, err := h.envVarRepo.CreateWithContext(r.Context(), models.CreateAppEnvVarInput{
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

// DeleteEnvVar menghapus variabel lingkungan aplikasi.
func (h *Handler) DeleteEnvVar(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")
	varID := chi.URLParam(r, "var_id")
	if appID == "" || varID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	_ = h.envVarRepo.DeleteWithContext(r.Context(), varID)
	w.Header().Set("HX-Redirect", "/applications/"+appID)
	w.WriteHeader(http.StatusOK)
}

// Move memindahkan aplikasi ke environment target.
func (h *Handler) Move(w http.ResponseWriter, r *http.Request) {
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

	if err := h.appRepo.MoveWithContext(r.Context(), id, targetEnvID); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/applications/"+id)
	w.WriteHeader(http.StatusOK)
}

// Clone mengkloning aplikasi dan variabel lingkungannya ke environment lain.
func (h *Handler) Clone(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByIDWithContext(r.Context(), id)
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

	clonedApp, err := h.appRepo.CloneWithContext(r.Context(), app, targetEnvID, newName)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	envVars, _ := h.envVarRepo.GetByApplicationIDWithContext(r.Context(), id)
	for _, envVar := range envVars {
		_, err := h.envVarRepo.CreateWithContext(r.Context(), models.CreateAppEnvVarInput{
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

	w.Header().Set("HX-Redirect", "/applications/"+clonedApp.ID)
	w.WriteHeader(http.StatusOK)
}

// SetDomain mengatur custom domain untuk aplikasi.
func (h *Handler) SetDomain(w http.ResponseWriter, r *http.Request) {
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

	_, err := h.appRepo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	domainPtr := &domain
	if err := h.appRepo.UpdateCustomDomainWithContext(r.Context(), id, domainPtr); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/applications/"+id)
	w.WriteHeader(http.StatusOK)
}

// RemoveDomain menghapus custom domain dari aplikasi.
func (h *Handler) RemoveDomain(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	_, err := h.appRepo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if err := h.appRepo.UpdateCustomDomainWithContext(r.Context(), id, nil); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/applications/"+id)
	w.WriteHeader(http.StatusOK)
}

// RegenerateWebhookSecret membuat secret baru untuk webhook aplikasi.
func (h *Handler) RegenerateWebhookSecret(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	secret, err := GenerateRandomSecret(32)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if err := h.appRepo.UpdateWebhookSecretWithContext(r.Context(), id, &secret); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	setToast(w, "Webhook secret berhasil diperbarui", "success")
	w.Header().Set("HX-Redirect", "/applications/"+id)
	w.WriteHeader(http.StatusOK)
}

// UpdateResourceLimits memperbarui batas CPU dan memori aplikasi.
func (h *Handler) UpdateResourceLimits(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	var cpuLimit *float64
	if cpuStr := strings.TrimSpace(r.FormValue("cpu_limit")); cpuStr != "" {
		if val, err := strconv.ParseFloat(cpuStr, 64); err == nil && val > 0 {
			cpuLimit = &val
		}
	}

	var memoryLimit *int64
	if memStr := strings.TrimSpace(r.FormValue("memory_limit")); memStr != "" {
		if val, err := strconv.ParseInt(memStr, 10, 64); err == nil && val > 0 {
			memoryLimit = &val
		}
	}

	var memoryReservation *int64
	if resStr := strings.TrimSpace(r.FormValue("memory_reservation")); resStr != "" {
		if val, err := strconv.ParseInt(resStr, 10, 64); err == nil && val > 0 {
			memoryReservation = &val
		}
	}

	if err := h.appRepo.UpdateResourceLimitsWithContext(r.Context(), id, cpuLimit, memoryLimit, memoryReservation); err != nil {
		http.Error(w, "Failed to update resource limits: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Live update ke Docker container jika sedang running
	if h.dockerClient != nil && app.ContainerID != nil && *app.ContainerID != "" {
		running, _ := h.dockerClient.IsContainerRunning(r.Context(), *app.ContainerID)
		if running {
			limits := docker.ContainerResourceLimits{
				CPULimit:          cpuLimit,
				MemoryLimit:       memoryLimit,
				MemoryReservation: memoryReservation,
			}
			if err := h.dockerClient.UpdateContainerResources(r.Context(), *app.ContainerID, limits); err != nil {
				slog.Warn("live update container resources failed", "error", err, "container_id", *app.ContainerID)
			}
		}
	}

	http.Redirect(w, r, "/applications/"+id, http.StatusFound)
}

// UpdateHealthcheck memperbarui path dan batas waktu healthcheck aplikasi.
func (h *Handler) UpdateHealthcheck(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	_, err := h.appRepo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	path := strings.TrimSpace(r.FormValue("healthcheck_path"))
	if path == "" {
		path = "/"
	}

	var timeoutVal = 30
	if timeoutStr := strings.TrimSpace(r.FormValue("healthcheck_timeout")); timeoutStr != "" {
		if val, err := strconv.Atoi(timeoutStr); err == nil && val > 0 {
			timeoutVal = val
		}
	}

	if err := h.appRepo.UpdateHealthcheckWithContext(r.Context(), id, &path, &timeoutVal); err != nil {
		http.Error(w, "Failed to update healthcheck: "+err.Error(), http.StatusInternalServerError)
		return
	}

	setToast(w, "Konfigurasi healthcheck berhasil disimpan", "success")
	http.Redirect(w, r, "/applications/"+id, http.StatusFound)
}

// UpdateBasicAuth memperbarui HTTP basic auth untuk proteksi reverse proxy aplikasi.
func (h *Handler) UpdateBasicAuth(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	user := strings.TrimSpace(r.FormValue("basic_auth_user"))
	password := strings.TrimSpace(r.FormValue("basic_auth_password"))

	var userPtr *string
	var passPtr *string

	if user != "" && password != "" {
		userPtr = &user
		passPtr = &password
	}

	if err := h.appRepo.UpdateBasicAuthWithContext(r.Context(), id, userPtr, passPtr); err != nil {
		http.Error(w, "Failed to update basic auth: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Live update ke Caddy jika aplikasi sedang running dan memiliki domain kustom
	if h.caddyClient != nil && app.CustomDomain != nil && *app.CustomDomain != "" && app.ContainerName != nil {
		opts := caddy.RouteOptions{}
		if userPtr != nil && passPtr != nil {
			opts.BasicAuthUser = *userPtr
			opts.BasicAuthPassword = *passPtr
		}
		if err := h.caddyClient.AddRouteWithOptions(*app.CustomDomain, *app.ContainerName, app.AppPort, opts); err != nil {
			slog.Warn("failed to live update caddy basic auth", "error", err, "domain", *app.CustomDomain)
		}
	}

	setToast(w, "Konfigurasi HTTP Basic Auth berhasil disimpan", "success")
	http.Redirect(w, r, "/applications/"+id, http.StatusFound)
}

// Status mengecek dan memperbarui status polling aplikasi.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if h.dockerClient != nil && app.ContainerID != nil && *app.ContainerID != "" {
		running, err := h.dockerClient.IsContainerRunning(r.Context(), *app.ContainerID)
		if err == nil {
			newStatus := models.AppStatusStopped
			if running {
				newStatus = models.AppStatusRunning
			}
			if app.Status != newStatus && app.Status != models.AppStatusIdle {
				_ = h.appRepo.UpdateStatusWithContext(r.Context(), app.ID, newStatus)
				app.Status = newStatus
			}
		}
	}

	templ.Handler(components.PollingStatusBadge(string(app.Status), "/applications/"+id+"/status")).ServeHTTP(w, r)
}

func parseInt(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	val, err := strconv.Atoi(s)
	if err != nil || val <= 0 {
		return defaultValue
	}
	return val
}

func setToast(w http.ResponseWriter, message string, toastType string) {
	if toastType == "" {
		toastType = "info"
	}
	payload := map[string]any{
		"showToast": map[string]string{
			"message": message,
			"type":    toastType,
		},
	}
	if data, err := json.Marshal(payload); err == nil {
		w.Header().Set("HX-Trigger", string(data))
	}
}
