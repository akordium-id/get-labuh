package deployments

import (
	"database/sql"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/docker"
)

// Module membungkus komponen domain Deployments.
type Module struct {
	Repo    *Repo
	Handler *Handler
}

// New menginisialisasi modul Deployments.
func New(db *sql.DB, appProvider ApplicationProvider) *Module {
	repo := NewRepo(db)
	handler := NewHandler(repo, appProvider)

	return &Module{
		Repo:    repo,
		Handler: handler,
	}
}

// SetDockerClient mengatur docker client untuk modul deployments.
func (m *Module) SetDockerClient(client *docker.Client) {
	m.Handler.SetDockerClient(client)
}

// SetDeployWorker menyetel worker antrean deployment untuk instant rollback.
func (m *Module) SetDeployWorker(w DeployEnqueuer) {
	m.Handler.SetDeployWorker(w)
}

// SetEnvVarRepo menyetel repository env vars untuk instant rollback.
func (m *Module) SetEnvVarRepo(r EnvVarProvider) {
	m.Handler.SetEnvVarRepo(r)
}

// RegisterRoutes mendaftarkan rute-rute deployment.
func (m *Module) RegisterRoutes(r chi.Router) {
	r.Get("/deployments/{id}", m.Handler.Get)
	r.Get("/deployments/{id}/logs", m.Handler.Logs)
	r.Post("/deployments/{id}/rollback", m.Handler.Rollback)
	r.Post("/deployments/{id}/start", m.Handler.Start)

	// SSE log streams
	r.Get("/deployments/{id}/logs/stream", m.Handler.StreamDeploymentLogs)
	r.Get("/applications/{id}/logs/stream", m.Handler.StreamContainerLogs)
}
