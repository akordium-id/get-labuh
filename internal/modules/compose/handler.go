package compose

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/caddy"
	composeengine "github.com/akordium-id/get-labuh/internal/compose"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/modules/projects"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/compose"
)

// SettingProvider interface untuk mengambil setting Caddy
type SettingProvider interface {
	Get(ctx context.Context, key string) (string, error)
}

// Orchestrator interface untuk mengendalikan Docker Compose stack
type Orchestrator interface {
	Up(ctx context.Context, app *models.ComposeApplication, envVars []models.AppEnvVar) (string, error)
	Down(ctx context.Context, app *models.ComposeApplication) (string, error)
	RestartService(ctx context.Context, app *models.ComposeApplication, serviceName string) (string, error)
	InspectServices(ctx context.Context, app *models.ComposeApplication) ([]models.ComposeServiceRuntime, error)
	GetServiceLogs(ctx context.Context, app *models.ComposeApplication, serviceName string, tail int) (string, error)
}

// EnvResolver interface untuk me-resolve variabel lingkungan cascading
type EnvResolver interface {
	ResolveForEnvironment(ctx context.Context, environmentID string) ([]models.AppEnvVar, error)
}

// Handler mengelola HTTP request untuk aplikasi Docker Compose.
type Handler struct {
	composeRepo     *Repo
	caddyClient     *caddy.Client
	settingProvider SettingProvider
	orchestrator    Orchestrator
	envResolver     EnvResolver
}

func NewHandler(composeRepo *Repo, caddyClient *caddy.Client, settingProvider SettingProvider, orchestrator ...Orchestrator) *Handler {
	var orch Orchestrator
	if len(orchestrator) > 0 {
		orch = orchestrator[0]
	}
	return &Handler{
		composeRepo:     composeRepo,
		caddyClient:     caddyClient,
		settingProvider: settingProvider,
		orchestrator:    orch,
	}
}

func (h *Handler) SetOrchestrator(o Orchestrator) {
	h.orchestrator = o
}

func (h *Handler) SetEnvResolver(r EnvResolver) {
	h.envResolver = r
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	envID := chi.URLParam(r, "env_id")
	if envID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	composes, err := h.composeRepo.GetByEnvironmentIDWithContext(r.Context(), envID)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	templ.Handler(layouts.AppLayout(compose.ComposeListPage(composes))).ServeHTTP(w, r)
}

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
	slug := r.FormValue("slug")
	composeFilePath := r.FormValue("compose_file_path")
	composeProjectName := r.FormValue("compose_project_name")
	customDomain := r.FormValue("custom_domain")
	rawYAML := r.FormValue("raw_yaml")

	if slug == "" {
		slug = projects.GenerateSlug(name)
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

	created, err := h.composeRepo.CreateWithContext(r.Context(), models.CreateComposeApplicationInput{
		EnvironmentID:      envID,
		Name:               name,
		Slug:               slug,
		ComposeFilePath:    composeFilePath,
		ComposeProjectName: composeProjectNamePtr,
		CustomDomain:       customDomainPtr,
		RawYAML:            rawYAML,
	})
	if err != nil {
		http.Error(w, "Internal Server Error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if rawYAML != "" {
		if parsed, err := composeengine.ParseComposeYAML(rawYAML); err == nil {
			_ = h.composeRepo.UpdateYAMLWithContext(r.Context(), created.ID, rawYAML, parsed)
		}
	}

	w.Header().Set("HX-Redirect", "/projects/"+chi.URLParam(r, "project_id")+"/environments/"+envID+"/compose-apps")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) CreatePage(w http.ResponseWriter, r *http.Request) {
	templ.Handler(layouts.AppLayout(compose.ComposeCreatePage())).ServeHTTP(w, r)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	composeApp, err := h.composeRepo.GetByIDWithContext(r.Context(), id)
	if err != nil || composeApp == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	var runtimes []models.ComposeServiceRuntime
	if h.orchestrator != nil {
		if rts, err := h.orchestrator.InspectServices(r.Context(), composeApp); err == nil {
			runtimes = rts
		}
	}

	templ.Handler(layouts.AppLayout(compose.ComposeDetailPage(composeApp, runtimes))).ServeHTTP(w, r)
}

func (h *Handler) UpdateYAML(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	rawYAML := r.FormValue("raw_yaml")
	parsedServices, err := composeengine.ParseComposeYAML(rawYAML)
	if err != nil {
		http.Error(w, "Sintaks YAML tidak valid: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.composeRepo.UpdateYAMLWithContext(r.Context(), id, rawYAML, parsedServices); err != nil {
		http.Error(w, "Gagal menyimpan YAML: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/compose-apps/"+id)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) ValidateYAML(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	rawYAML := r.FormValue("raw_yaml")
	services, err := composeengine.ParseComposeYAML(rawYAML)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	if err != nil {
		_, _ = fmt.Fprintf(w, `<div class="p-3 bg-red-50 border border-red-200 text-red-700 text-sm rounded-md">❌ %s</div>`, err.Error())
		return
	}

	var names []string
	for _, s := range services {
		names = append(names, s.Name)
	}
	_, _ = fmt.Fprintf(w, `<div class="p-3 bg-green-50 border border-green-200 text-green-700 text-sm rounded-md">✅ Valid! Terdeteksi %d service (%s)</div>`, len(services), strings.Join(names, ", "))
}

func (h *Handler) Deploy(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	composeApp, err := h.composeRepo.GetByIDWithContext(r.Context(), id)
	if err != nil || composeApp == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	_ = h.composeRepo.UpdateStatusWithContext(r.Context(), id, models.ComposeStatusBuilding)

	var envVars []models.AppEnvVar
	if h.envResolver != nil {
		if resolved, resErr := h.envResolver.ResolveForEnvironment(r.Context(), composeApp.EnvironmentID); resErr == nil {
			envVars = resolved
		}
	}

	if h.orchestrator != nil {
		_, upErr := h.orchestrator.Up(r.Context(), composeApp, envVars)
		now := time.Now()
		if upErr != nil {
			errMsg := upErr.Error()
			_ = h.composeRepo.UpdateDeploymentStatusWithContext(r.Context(), id, models.ComposeStatusFailed, &now, &errMsg)
		} else {
			_ = h.composeRepo.UpdateDeploymentStatusWithContext(r.Context(), id, models.ComposeStatusRunning, &now, nil)
		}
	} else {
		now := time.Now()
		_ = h.composeRepo.UpdateDeploymentStatusWithContext(r.Context(), id, models.ComposeStatusRunning, &now, nil)
	}

	w.Header().Set("HX-Redirect", "/compose-apps/"+id)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	composeApp, err := h.composeRepo.GetByIDWithContext(r.Context(), id)
	if err != nil || composeApp == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if h.orchestrator != nil {
		_, _ = h.orchestrator.Down(r.Context(), composeApp)
	}
	_ = h.composeRepo.UpdateStatusWithContext(r.Context(), id, models.ComposeStatusStopped)

	w.Header().Set("HX-Redirect", "/compose-apps/"+id)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	h.Deploy(w, r)
}

func (h *Handler) RestartService(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	serviceName := chi.URLParam(r, "service")
	if id == "" || serviceName == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	composeApp, err := h.composeRepo.GetByIDWithContext(r.Context(), id)
	if err != nil || composeApp == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if h.orchestrator != nil {
		_, err := h.orchestrator.RestartService(r.Context(), composeApp, serviceName)
		if err != nil {
			http.Error(w, fmt.Sprintf("Gagal me-restart service %s: %v", serviceName, err), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("HX-Redirect", "/compose-apps/"+id)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) ServiceLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	serviceName := chi.URLParam(r, "service")
	if id == "" || serviceName == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	composeApp, err := h.composeRepo.GetByIDWithContext(r.Context(), id)
	if err != nil || composeApp == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if h.orchestrator == nil {
		http.Error(w, "Docker orchestrator tidak tersedia", http.StatusInternalServerError)
		return
	}

	logs, err := h.orchestrator.GetServiceLogs(r.Context(), composeApp, serviceName, 100)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte(logs))
}

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

	composeApp, err := h.composeRepo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	domainPtr := &domain
	if err := h.composeRepo.UpdateCustomDomainWithContext(r.Context(), id, domainPtr); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if h.caddyClient != nil && h.settingProvider != nil {
		caddyAPIURL, _ := h.settingProvider.Get(r.Context(), "caddy_api_url")
		caddyAPIKey, _ := h.settingProvider.Get(r.Context(), "caddy_api_key")
		client := caddy.NewClient(caddyAPIURL, caddyAPIKey)

		_ = composeApp
		_ = client
	}

	w.Header().Set("HX-Redirect", "/compose-apps/"+id)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) RemoveDomain(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	composeApp, err := h.composeRepo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if err := h.composeRepo.UpdateCustomDomainWithContext(r.Context(), id, nil); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if h.caddyClient != nil && composeApp.CustomDomain != nil && *composeApp.CustomDomain != "" && h.settingProvider != nil {
		caddyAPIURL, _ := h.settingProvider.Get(r.Context(), "caddy_api_url")
		caddyAPIKey, _ := h.settingProvider.Get(r.Context(), "caddy_api_key")
		client := caddy.NewClient(caddyAPIURL, caddyAPIKey)

		_ = client
	}

	w.Header().Set("HX-Redirect", "/compose-apps/"+id)
	w.WriteHeader(http.StatusOK)
}
