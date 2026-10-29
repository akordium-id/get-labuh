package applications

import (
	"database/sql"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/caddy"
	"github.com/akordium-id/get-labuh/internal/docker"
)

// Module mengelompokkan repository dan HTTP handler domain Applications.
type Module struct {
	Repo       *Repo
	EnvVarRepo *EnvVarRepo
	Handler    *Handler
	Webhooks   *WebhooksHandler
	Dropzone   *DropzoneHandler
	Monitoring *MonitoringHandler
}

// New menginisialisasi modul Applications.
func New(
	db *sql.DB,
	envRepo EnvironmentProvider,
	projectRepo ProjectEnvironmentProvider,
	deployRepo DeploymentCreator,
	settingRepo SettingProvider,
	caddyClient *caddy.Client,
	dockerClient *docker.Client,
) *Module {
	repo := NewRepo(db)
	envVarRepo := NewEnvVarRepo(db)

	handler := NewHandler(repo, envVarRepo, envRepo, deployRepo, settingRepo, caddyClient, dockerClient)
	webhooks := NewWebhooksHandler(repo, deployRepo, envVarRepo)
	dropzone := NewDropzoneHandler(repo, projectRepo, deployRepo, envVarRepo)
	monitoring := NewMonitoringHandler(repo, dockerClient)

	return &Module{
		Repo:       repo,
		EnvVarRepo: envVarRepo,
		Handler:    handler,
		Webhooks:   webhooks,
		Dropzone:   dropzone,
		Monitoring: monitoring,
	}
}

// SetDeployWorker menyetel worker deploy untuk seluruh handler.
func (m *Module) SetDeployWorker(w DeployEnqueuer) {
	m.Handler.SetDeployWorker(w)
	m.Webhooks.SetDeployWorker(w)
	m.Dropzone.SetDeployWorker(w)
}

// SetDockerClient menyetel docker client untuk seluruh handler.
func (m *Module) SetDockerClient(c *docker.Client) {
	m.Handler.SetDockerClient(c)
	m.Monitoring.SetDockerClient(c)
}

// SetCaddyClient menyetel caddy client.
func (m *Module) SetCaddyClient(c *caddy.Client) {
	m.Handler.SetCaddyClient(c)
}

// RegisterRoutes mendaftarkan rute dashboard dan manajemen aplikasi (terproteksi).
func (m *Module) RegisterRoutes(r chi.Router) {
	r.Post("/lite/deploy/upload", m.Dropzone.UploadDropzone)

	r.Get("/projects/{project_id}/environments/{env_id}/applications", m.Handler.List)
	r.Post("/projects/{project_id}/environments/{env_id}/applications", m.Handler.Create)
	r.Get("/applications/{id}", m.Handler.Get)
	r.Post("/applications/{id}/start", m.Handler.Start)
	r.Post("/applications/{id}/stop", m.Handler.Stop)
	r.Post("/applications/{id}/restart", m.Handler.Restart)
	r.Post("/applications/{id}/deploy", m.Handler.Deploy)
	r.Post("/applications/{id}/env-vars", m.Handler.CreateEnvVar)
	r.Post("/applications/{id}/env-vars/{var_id}/delete", m.Handler.DeleteEnvVar)
	r.Post("/applications/{id}/domain", m.Handler.SetDomain)
	r.Post("/applications/{id}/domain/remove", m.Handler.RemoveDomain)
	r.Post("/applications/{id}/resources", m.Handler.UpdateResourceLimits)
	r.Post("/applications/{id}/webhook/regenerate", m.Handler.RegenerateWebhookSecret)
	r.Get("/applications/{id}/status", m.Handler.Status)
	r.Get("/applications/{id}/stats", m.Monitoring.Stats)
}

// RegisterPublicRoutes mendaftarkan rute publik webhook.
func (m *Module) RegisterPublicRoutes(r chi.Router) {
	r.Post("/webhooks/deploy/{application_id}", m.Webhooks.Deploy)
	r.Post("/webhooks/github/{application_id}", m.Webhooks.GitHub)
	r.Post("/webhooks/gitlab/{application_id}", m.Webhooks.GitLab)
}
