package applications

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/caddy"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/encryption"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/components"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	webapp "github.com/akordium-id/get-labuh/internal/web/pages/applications"
)

// Handler menangani operasi HTTP untuk domain Applications.
var volumeNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

type VolumeBackupRunner interface {
	RunBackup(ctx context.Context, appID string) (*models.AppBackup, error)
	RestoreAppVolume(ctx context.Context, backupID string) error
}

type Handler struct {
	appRepo            *Repo
	envVarRepo         *EnvVarRepo
	envRepo            EnvironmentProvider
	deployRepo         DeploymentCreator
	settingRepo        SettingProvider
	caddyClient        *caddy.Client
	dockerClient       *docker.Client
	deployWorker       DeployEnqueuer
	cronRepo           *CronJobRepo
	cronScheduler      *CronScheduler
	volumeBackupRunner VolumeBackupRunner
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

// SetCronDependencies menyetel cron repository dan scheduler ke handler.
func (h *Handler) SetCronDependencies(repo *CronJobRepo, scheduler *CronScheduler) {
	h.cronRepo = repo
	h.cronScheduler = scheduler
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
	serverID := strings.TrimSpace(r.FormValue("server_id"))
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
	var serverIDPtr *string
	if serverID != "" {
		serverIDPtr = &serverID
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
		ServerID:      serverIDPtr,
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
	if h.deployRepo != nil {
		if deps, err := h.deployRepo.GetByApplicationIDWithContext(r.Context(), id); err == nil {
			deployments = deps
		}
	}
	envVars, _ := h.envVarRepo.GetByApplicationIDWithContext(r.Context(), id)

	var cronJobs []*models.ApplicationCronJob
	if h.cronRepo != nil {
		if jobs, err := h.cronRepo.GetByApplicationIDWithContext(r.Context(), id); err == nil {
			cronJobs = jobs
		}
	}

	var prPreviews []*models.Application
	if previews, err := h.appRepo.ListPreviewsWithContext(r.Context(), id); err == nil {
		prPreviews = previews
	}

	volumes, _ := h.appRepo.GetVolumesByAppID(r.Context(), id)
	fileMounts, _ := h.appRepo.GetFileMountsByAppID(r.Context(), id)

	templ.Handler(layouts.AppLayout(webapp.ApplicationDetailPage(app, deployments, envVars, cronJobs, prPreviews, volumes, fileMounts))).ServeHTTP(w, r)
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

// BulkCreateEnvVars mem-parsing teks .env mentah dan melakukan upsert ke variabel lingkungan aplikasi.
func (h *Handler) BulkCreateEnvVars(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")
	if appID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	rawEnv := r.FormValue("raw_env")
	forceSecret := r.FormValue("force_secret") == "on" || r.FormValue("force_secret") == "true"

	parsedVars := ParseEnvContent(rawEnv, forceSecret)
	if len(parsedVars) == 0 {
		setToast(w, "Tidak ada variabel yang ditemukan di format .env", "error")
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", "/applications/"+appID+"#env")
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Redirect(w, r, "/applications/"+appID+"#env", http.StatusFound)
		return
	}

	count, err := h.envVarRepo.UpsertBulkWithContext(r.Context(), appID, parsedVars)
	if err != nil {
		http.Error(w, "Failed to import environment variables: "+err.Error(), http.StatusInternalServerError)
		return
	}

	setToast(w, fmt.Sprintf("%d environment variables berhasil diimpor & disimpan", count), "success")
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/applications/"+appID+"#env")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/applications/"+appID+"#env", http.StatusFound)
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

// CreateCronJob membuat cron job baru untuk aplikasi.
func (h *Handler) CreateCronJob(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")
	if appID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	command := strings.TrimSpace(r.FormValue("command"))
	cronExpr := strings.TrimSpace(r.FormValue("cron_expression"))

	if name == "" || command == "" || cronExpr == "" {
		http.Error(w, "Name, command, and cron expression are required", http.StatusBadRequest)
		return
	}

	if h.cronRepo == nil {
		http.Error(w, "Cron repository not configured", http.StatusInternalServerError)
		return
	}

	_, err := h.cronRepo.CreateWithContext(r.Context(), models.CreateCronJobInput{
		ApplicationID:  appID,
		Name:           name,
		Command:        command,
		CronExpression: cronExpr,
	})
	if err != nil {
		http.Error(w, "Failed to create cron job: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/applications/"+appID)
	w.WriteHeader(http.StatusOK)
}

// ToggleCronJob mengaktifkan/menonaktifkan cron job.
func (h *Handler) ToggleCronJob(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")
	jobID := chi.URLParam(r, "job_id")
	if appID == "" || jobID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if h.cronRepo == nil {
		http.Error(w, "Cron repository not configured", http.StatusInternalServerError)
		return
	}

	_, err := h.cronRepo.ToggleEnabledWithContext(r.Context(), jobID)
	if err != nil {
		http.Error(w, "Failed to toggle cron job: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/applications/"+appID)
	w.WriteHeader(http.StatusOK)
}

// RunCronJobNow menjalankan cron job secara instan di dalam container aplikasi.
func (h *Handler) RunCronJobNow(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")
	jobID := chi.URLParam(r, "job_id")
	if appID == "" || jobID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if h.cronScheduler == nil {
		http.Error(w, "Cron scheduler not configured", http.StatusInternalServerError)
		return
	}

	_, err := h.cronScheduler.RunJob(r.Context(), jobID)
	if err != nil {
		slog.Warn("manual cron job run encountered error", "job_id", jobID, "error", err)
	}

	w.Header().Set("HX-Redirect", "/applications/"+appID)
	w.WriteHeader(http.StatusOK)
}

// DeleteCronJob menghapus cron job.
func (h *Handler) DeleteCronJob(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")
	jobID := chi.URLParam(r, "job_id")
	if appID == "" || jobID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if h.cronRepo == nil {
		http.Error(w, "Cron repository not configured", http.StatusInternalServerError)
		return
	}

	if err := h.cronRepo.DeleteWithContext(r.Context(), jobID); err != nil {
		http.Error(w, "Failed to delete cron job: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/applications/"+appID)
	w.WriteHeader(http.StatusOK)
}

// UpdatePRPreviewsConfig memperbarui konfigurasi otomatis ephemeral PR previews.
func (h *Handler) UpdatePRPreviewsConfig(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	enabled := r.FormValue("pr_previews_enabled") == "on" || r.FormValue("pr_previews_enabled") == "true"
	template := strings.TrimSpace(r.FormValue("preview_template_domain"))
	var templatePtr *string
	if template != "" {
		templatePtr = &template
	}

	if err := h.appRepo.UpdatePRPreviewsConfigWithContext(r.Context(), id, enabled, templatePtr); err != nil {
		http.Error(w, "Failed to update PR preview config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	gitTokenInput := strings.TrimSpace(r.FormValue("git_token"))
	if gitTokenInput != "" {
		encryptedToken, encErr := encryption.Encrypt(gitTokenInput)
		if encErr == nil {
			_ = h.appRepo.UpdateGitTokenWithContext(r.Context(), id, &encryptedToken)
		}
	}

	setToast(w, "Konfigurasi PR Previews berhasil disimpan", "success")
	w.Header().Set("HX-Redirect", "/applications/"+id+"#advanced")
	w.WriteHeader(http.StatusOK)
}

// TeardownPRPreview menghapus environment PR preview secara manual.
func (h *Handler) TeardownPRPreview(w http.ResponseWriter, r *http.Request) {
	parentID := chi.URLParam(r, "id")
	previewID := chi.URLParam(r, "preview_id")
	if parentID == "" || previewID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	previewApp, err := h.appRepo.GetByIDWithContext(r.Context(), previewID)
	if err != nil {
		http.Error(w, "Preview Not Found", http.StatusNotFound)
		return
	}

	// 1. Remove Docker Container
	if h.dockerClient != nil && previewApp.ContainerID != nil && *previewApp.ContainerID != "" {
		if err := h.dockerClient.RemoveContainer(r.Context(), *previewApp.ContainerID); err != nil {
			slog.Warn("failed to remove docker container during preview teardown", "error", err, "container_id", *previewApp.ContainerID)
		}
	}

	// 2. Remove Caddy Route if domain exists
	if h.caddyClient != nil && previewApp.CustomDomain != nil && *previewApp.CustomDomain != "" {
		if err := h.caddyClient.RemoveRoute(*previewApp.CustomDomain); err != nil {
			slog.Warn("failed to remove caddy route during preview teardown", "error", err, "domain", *previewApp.CustomDomain)
		}
	}

	// 3. Remove Ephemeral Named Volumes for PR preview
	if h.dockerClient != nil && previewApp.IsPreview && previewApp.PRNumber != nil {
		vols, vErr := h.appRepo.GetVolumesByAppID(r.Context(), previewApp.ID)
		if vErr == nil {
			cleanSlug := previewApp.Slug
			suffix := fmt.Sprintf("-pr-%d", *previewApp.PRNumber)
			cleanSlug = strings.TrimSuffix(cleanSlug, suffix)
			for _, v := range vols {
				if v.VolumeType == models.VolumeTypeNamed {
					volName := fmt.Sprintf("labuh-vol-app-%s-%s-pr-%d-%s", "prod", cleanSlug, *previewApp.PRNumber, v.Name)
					if err := h.dockerClient.RemoveVolume(r.Context(), volName); err != nil {
						slog.Warn("failed to remove docker volume during preview teardown", "error", err, "volume_name", volName)
					}
				}
			}
		}
	}

	// 4. Delete database record
	if err := h.appRepo.DeleteWithContext(r.Context(), previewApp.ID); err != nil {
		http.Error(w, "Failed to delete preview application: "+err.Error(), http.StatusInternalServerError)
		return
	}

	setToast(w, "Preview environment berhasil dihapus (teardown)", "success")
	w.Header().Set("HX-Redirect", "/applications/"+parentID+"#previews")
	w.WriteHeader(http.StatusOK)
}

// UpdateLifecycleCommands memperbarui pre dan post deployment commands aplikasi.
func (h *Handler) UpdateLifecycleCommands(w http.ResponseWriter, r *http.Request) {
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

	preCmd := strings.TrimSpace(r.FormValue("pre_deployment_command"))
	postCmd := strings.TrimSpace(r.FormValue("post_deployment_command"))

	var preCmdPtr, postCmdPtr *string
	if preCmd != "" {
		preCmdPtr = &preCmd
	}
	if postCmd != "" {
		postCmdPtr = &postCmd
	}

	if err := h.appRepo.UpdateLifecycleCommandsWithContext(r.Context(), id, preCmdPtr, postCmdPtr); err != nil {
		http.Error(w, "Failed to update lifecycle commands: "+err.Error(), http.StatusInternalServerError)
		return
	}

	setToast(w, "Konfigurasi lifecycle commands berhasil disimpan", "success")
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/applications/"+id+"#advanced")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/applications/"+id+"#advanced", http.StatusFound)
}

// CreateVolume menambahkan konfigurasi persistent volume untuk aplikasi.
func (h *Handler) CreateVolume(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")
	if appID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	volType := strings.TrimSpace(r.FormValue("volume_type"))
	hostPath := strings.TrimSpace(r.FormValue("host_path"))
	mountPath := strings.TrimSpace(r.FormValue("mount_path"))
	isReadOnly := r.FormValue("is_read_only") == "true" || r.FormValue("is_read_only") == "1" || r.FormValue("is_read_only") == "on"

	if name == "" || mountPath == "" {
		http.Error(w, "Name and mount path are required", http.StatusBadRequest)
		return
	}

	if !volumeNameRegex.MatchString(name) {
		http.Error(w, "Volume name must contain only alphanumeric characters, dashes, and underscores", http.StatusBadRequest)
		return
	}

	if volType == "" {
		volType = string(models.VolumeTypeNamed)
	}

	if volType != string(models.VolumeTypeNamed) && volType != string(models.VolumeTypeBind) {
		http.Error(w, "Invalid volume type, must be 'named' or 'bind'", http.StatusBadRequest)
		return
	}

	cleanMountPath, err := docker.ValidateContainerMountPath(mountPath)
	if err != nil {
		http.Error(w, "Invalid mount path: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Collision check terhadap volume yang sudah ada
	existingVols, err := h.appRepo.GetVolumesByAppID(r.Context(), appID)
	if err == nil {
		for _, ev := range existingVols {
			if ev.Name == name {
				http.Error(w, "Volume name already in use for this application", http.StatusBadRequest)
				return
			}
			if ev.MountPath == cleanMountPath {
				http.Error(w, "Mount path already in use by another volume", http.StatusBadRequest)
				return
			}
		}
	}

	// Collision check terhadap file mounts yang sudah ada
	existingFiles, err := h.appRepo.GetFileMountsByAppID(r.Context(), appID)
	if err == nil {
		for _, ef := range existingFiles {
			if ef.MountPath == cleanMountPath {
				http.Error(w, "Mount path conflicts with an existing file mount", http.StatusBadRequest)
				return
			}
		}
	}

	var hostPathPtr *string
	if volType == string(models.VolumeTypeBind) {
		if hostPath == "" {
			http.Error(w, "Host path is required for bind volume", http.StatusBadRequest)
			return
		}
		if err := docker.ValidateHostPathSecurity(hostPath); err != nil {
			http.Error(w, "Invalid host path: "+err.Error(), http.StatusBadRequest)
			return
		}
		cleanHostPath := strings.TrimSpace(hostPath)
		hostPathPtr = &cleanHostPath
	}

	_, err = h.appRepo.CreateVolume(r.Context(), models.CreateAppVolumeInput{
		ApplicationID: appID,
		Name:          name,
		VolumeType:    models.VolumeType(volType),
		HostPath:      hostPathPtr,
		MountPath:     cleanMountPath,
		IsReadOnly:    isReadOnly,
	})
	if err != nil {
		http.Error(w, "Failed to create volume: "+err.Error(), http.StatusInternalServerError)
		return
	}

	setToast(w, "Persistent volume berhasil ditambahkan", "success")
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/applications/"+appID+"#storage")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/applications/"+appID+"#storage", http.StatusFound)
}

// DeleteVolume menghapus persistent volume dari aplikasi.
func (h *Handler) DeleteVolume(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")
	volID := chi.URLParam(r, "vol_id")
	if appID == "" || volID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := h.appRepo.DeleteVolume(r.Context(), volID); err != nil {
		http.Error(w, "Failed to delete volume: "+err.Error(), http.StatusInternalServerError)
		return
	}

	setToast(w, "Persistent volume berhasil dihapus", "success")
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/applications/"+appID+"#storage")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/applications/"+appID+"#storage", http.StatusFound)
}

// CreateFileMount menambahkan konfigurasi single file mount untuk aplikasi.
func (h *Handler) CreateFileMount(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")
	if appID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	mountPath := strings.TrimSpace(r.FormValue("mount_path"))
	content := r.FormValue("content")
	fileMode := strings.TrimSpace(r.FormValue("file_mode"))
	isReadOnly := r.FormValue("is_read_only") == "true" || r.FormValue("is_read_only") == "1" || r.FormValue("is_read_only") == "on"

	if mountPath == "" {
		http.Error(w, "Mount path is required", http.StatusBadRequest)
		return
	}

	cleanMountPath, err := docker.ValidateContainerMountPath(mountPath)
	if err != nil {
		http.Error(w, "Invalid mount path: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Collision check terhadap file mounts yang sudah ada
	existingFiles, err := h.appRepo.GetFileMountsByAppID(r.Context(), appID)
	if err == nil {
		for _, ef := range existingFiles {
			if ef.MountPath == cleanMountPath {
				http.Error(w, "Mount path already in use by another file mount", http.StatusBadRequest)
				return
			}
		}
	}

	// Collision check terhadap persistent volumes yang sudah ada
	existingVols, err := h.appRepo.GetVolumesByAppID(r.Context(), appID)
	if err == nil {
		for _, ev := range existingVols {
			if ev.MountPath == cleanMountPath {
				http.Error(w, "Mount path conflicts with an existing persistent volume", http.StatusBadRequest)
				return
			}
		}
	}

	if fileMode == "" {
		fileMode = "0644"
	}

	_, err = h.appRepo.CreateFileMount(r.Context(), models.CreateAppFileMountInput{
		ApplicationID: appID,
		MountPath:     cleanMountPath,
		Content:       content,
		FileMode:      fileMode,
		IsReadOnly:    isReadOnly,
	})
	if err != nil {
		http.Error(w, "Failed to create file mount: "+err.Error(), http.StatusInternalServerError)
		return
	}

	setToast(w, "File mount berhasil ditambahkan", "success")
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/applications/"+appID+"#storage")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/applications/"+appID+"#storage", http.StatusFound)
}

// UpdateFileMount memperbarui isi konten file mount.
func (h *Handler) UpdateFileMount(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")
	fileID := chi.URLParam(r, "file_id")
	if appID == "" || fileID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")

	if err := h.appRepo.UpdateFileMount(r.Context(), fileID, content); err != nil {
		http.Error(w, "Failed to update file mount: "+err.Error(), http.StatusInternalServerError)
		return
	}

	setToast(w, "File mount berhasil diperbarui", "success")
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/applications/"+appID+"#storage")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/applications/"+appID+"#storage", http.StatusFound)
}

// DeleteFileMount menghapus konfigurasi file mount.
func (h *Handler) DeleteFileMount(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "id")
	fileID := chi.URLParam(r, "file_id")
	if appID == "" || fileID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := h.appRepo.DeleteFileMount(r.Context(), fileID); err != nil {
		http.Error(w, "Failed to delete file mount: "+err.Error(), http.StatusInternalServerError)
		return
	}

	setToast(w, "File mount berhasil dihapus", "success")
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/applications/"+appID+"#storage")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/applications/"+appID+"#storage", http.StatusFound)
}

// SetVolumeBackupRunner menyetel volume backup runner dependency.
func (h *Handler) SetVolumeBackupRunner(runner VolumeBackupRunner) {
	h.volumeBackupRunner = runner
}

// UpdateBackupConfig menyimpan konfigurasi backup volume aplikasi.
func (h *Handler) UpdateBackupConfig(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	enabled := r.FormValue("enabled") == "true" || r.FormValue("enabled") == "1" || r.FormValue("enabled") == "on"
	cronExp := strings.TrimSpace(r.FormValue("cron_expression"))
	if cronExp == "" {
		cronExp = "0 3 * * *"
	}
	retentionStr := r.FormValue("retention_count")
	retention := 7
	if parsed, err := strconv.Atoi(retentionStr); err == nil && parsed > 0 {
		retention = parsed
	}

	var storageDestIDPtr *string
	destID := strings.TrimSpace(r.FormValue("storage_destination_id"))
	if destID != "" {
		storageDestIDPtr = &destID
	}

	var customPrefixPtr *string
	prefix := strings.TrimSpace(r.FormValue("custom_prefix"))
	if prefix != "" {
		customPrefixPtr = &prefix
	}

	err := h.appRepo.UpsertBackupConfigWithContext(r.Context(), models.UpdateAppBackupConfigInput{
		ApplicationID:        id,
		StorageDestinationID: storageDestIDPtr,
		Enabled:              enabled,
		CronExpression:       cronExp,
		RetentionCount:       retention,
		CustomPrefix:         customPrefixPtr,
	})
	if err != nil {
		http.Error(w, "Failed to update backup config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	setToast(w, "Konfigurasi backup volume berhasil disimpan", "success")
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/applications/"+id+"#storage")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/applications/"+id+"#storage", http.StatusFound)
}

// CreateVolumeBackup memicu backup snapshot volume aplikasi.
func (h *Handler) CreateVolumeBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if h.volumeBackupRunner == nil {
		http.Error(w, "Volume backup runner not configured", http.StatusInternalServerError)
		return
	}

	_, err := h.volumeBackupRunner.RunBackup(r.Context(), id)
	if err != nil {
		http.Error(w, "Backup volume failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	setToast(w, "Backup volume aplikasi berhasil dibuat", "success")
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/applications/"+id+"#storage")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/applications/"+id+"#storage", http.StatusFound)
}

// DownloadVolumeBackup mengunduh file snapshot backup volume (.tar.gz).
func (h *Handler) DownloadVolumeBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	backupID := chi.URLParam(r, "backup_id")
	if id == "" || backupID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	bRecord, err := h.appRepo.GetBackupByIDWithContext(r.Context(), backupID)
	if err != nil {
		http.Error(w, "Backup not found", http.StatusNotFound)
		return
	}

	baseDir := "/var/lib/labuh/backups/apps"
	localPath := filepath.Join(baseDir, id, bRecord.Filename)
	f, err := os.Open(localPath)
	if err != nil {
		localPath = filepath.Join("./.labuh/backups/apps", id, bRecord.Filename)
		f, err = os.Open(localPath)
	}
	if err != nil {
		http.Error(w, "Backup file not found on server", http.StatusNotFound)
		return
	}
	defer f.Close()

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", bRecord.Filename))
	w.Header().Set("Content-Type", "application/gzip")
	http.ServeContent(w, r, bRecord.Filename, bRecord.CreatedAt, f)
}

// RestoreVolumeBackup mengeksekusi Safe Restore volume aplikasi.
func (h *Handler) RestoreVolumeBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	backupID := chi.URLParam(r, "backup_id")
	if id == "" || backupID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if h.volumeBackupRunner == nil {
		http.Error(w, "Volume backup runner not configured", http.StatusInternalServerError)
		return
	}

	if err := h.volumeBackupRunner.RestoreAppVolume(r.Context(), backupID); err != nil {
		http.Error(w, "Failed to restore volume: "+err.Error(), http.StatusInternalServerError)
		return
	}

	setToast(w, "Volume aplikasi berhasil di-restore dari snapshot", "success")
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/applications/"+id+"#storage")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/applications/"+id+"#storage", http.StatusFound)
}

// DeleteVolumeBackup menghapus snapshot volume aplikasi (lokal & S3).
func (h *Handler) DeleteVolumeBackup(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	backupID := chi.URLParam(r, "backup_id")
	if id == "" || backupID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	bRecord, err := h.appRepo.GetBackupByIDWithContext(r.Context(), backupID)
	if err == nil {
		localPath := filepath.Join("/var/lib/labuh/backups/apps", id, bRecord.Filename)
		_ = os.Remove(localPath)
		_ = os.Remove(filepath.Join("./.labuh/backups/apps", id, bRecord.Filename))

		_ = h.appRepo.DeleteBackupWithContext(r.Context(), backupID)
	}

	setToast(w, "Backup volume berhasil dihapus", "success")
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/applications/"+id+"#storage")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/applications/"+id+"#storage", http.StatusFound)
}

