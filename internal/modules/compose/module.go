package compose

import (
	"database/sql"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/caddy"
)

// Module mengemas repository dan HTTP handler untuk aplikasi Docker Compose.
type Module struct {
	Repo    *Repo
	Handler *Handler
}

func New(db *sql.DB, caddyClient *caddy.Client, settingProvider SettingProvider) *Module {
	repo := NewRepo(db)
	handler := NewHandler(repo, caddyClient, settingProvider)

	return &Module{
		Repo:    repo,
		Handler: handler,
	}
}

// SetOrchestrator menginjeksi orchestrator engine Docker Compose.
func (m *Module) SetOrchestrator(o Orchestrator) {
	m.Handler.SetOrchestrator(o)
}

// SetEnvResolver menginjeksi resolver variabel lingkungan bersama (Team & Project).
func (m *Module) SetEnvResolver(r EnvResolver) {
	m.Handler.SetEnvResolver(r)
}

// RegisterRoutes mendaftarkan rute Chi untuk compose apps ke router aplikasi.
func (m *Module) RegisterRoutes(r chi.Router) {
	r.Get("/projects/{project_id}/environments/{env_id}/compose-apps", m.Handler.List)
	r.Get("/projects/{project_id}/environments/{env_id}/compose-apps/new", m.Handler.CreatePage)
	r.Post("/projects/{project_id}/environments/{env_id}/compose-apps", m.Handler.Create)
	r.Get("/compose-apps/{id}", m.Handler.Get)
	r.Post("/compose-apps/{id}/yaml", m.Handler.UpdateYAML)
	r.Post("/compose-apps/{id}/validate", m.Handler.ValidateYAML)
	r.Post("/compose-apps/{id}/deploy", m.Handler.Deploy)
	r.Post("/compose-apps/{id}/stop", m.Handler.Stop)
	r.Post("/compose-apps/{id}/start", m.Handler.Start)
	r.Post("/compose-apps/{id}/services/{service}/restart", m.Handler.RestartService)
	r.Get("/compose-apps/{id}/services/{service}/logs", m.Handler.ServiceLogs)
	r.Post("/compose-apps/{id}/domain", m.Handler.SetDomain)
	r.Post("/compose-apps/{id}/domain/remove", m.Handler.RemoveDomain)
}
