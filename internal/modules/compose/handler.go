package compose

import (
	"context"
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/caddy"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/modules/projects"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/compose"
)

// SettingProvider interface untuk mengambil setting Caddy
type SettingProvider interface {
	Get(ctx context.Context, key string) (string, error)
}

// Handler mengelola HTTP request untuk aplikasi Docker Compose.
type Handler struct {
	composeRepo     *Repo
	caddyClient     *caddy.Client
	settingProvider SettingProvider
}

func NewHandler(composeRepo *Repo, caddyClient *caddy.Client, settingProvider SettingProvider) *Handler {
	return &Handler{
		composeRepo:     composeRepo,
		caddyClient:     caddyClient,
		settingProvider: settingProvider,
	}
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

	_, err := h.composeRepo.CreateWithContext(r.Context(), models.CreateComposeApplicationInput{
		EnvironmentID:      envID,
		Name:               name,
		Slug:               slug,
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
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	templ.Handler(layouts.AppLayout(compose.ComposeDetailPage(composeApp))).ServeHTTP(w, r)
}

func (h *Handler) Deploy(w http.ResponseWriter, r *http.Request) {
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

	_ = h.composeRepo.UpdateStatusWithContext(r.Context(), id, models.ComposeStatusBuilding)

	logPath := "/tmp/labuh-logs/" + id + ".log"

	_ = composeApp
	_ = logPath

	w.Header().Set("HX-Redirect", "/compose-apps/"+id)
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
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

	_ = h.composeRepo.UpdateStatusWithContext(r.Context(), id, models.ComposeStatusRunning)
	w.Header().Set("HX-Redirect", "/compose-apps/"+id)
	w.WriteHeader(http.StatusOK)
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
