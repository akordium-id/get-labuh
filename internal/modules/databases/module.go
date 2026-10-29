package databases

import (
	"database/sql"

	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/go-chi/chi/v5"
)

// Module mengemas repository dan HTTP handler untuk domain managed databases.
type Module struct {
	ServiceRepo *ServiceRepo
	BackupRepo  *BackupRepo
	Handler     *Handler
}

func New(db *sql.DB, envRepo EnvironmentProvider) *Module {
	serviceRepo := NewServiceRepo(db)
	backupRepo := NewBackupRepo(db)
	handler := NewHandler(serviceRepo, envRepo)

	return &Module{
		ServiceRepo: serviceRepo,
		BackupRepo:  backupRepo,
		Handler:     handler,
	}
}

func (m *Module) SetDockerClient(client *docker.Client) {
	m.Handler.SetDockerClient(client)
}

func (m *Module) SetBackupDependencies(
	storageRepo StorageProvider,
	runner BackupRunner,
	s3Factory S3ClientFactory,
) {
	m.Handler.SetBackupDependencies(m.BackupRepo, storageRepo, runner, s3Factory)
}

// RegisterRoutes mendaftarkan rute Chi untuk database services dan backups.
func (m *Module) RegisterRoutes(r chi.Router) {
	r.Get("/projects/{project_id}/environments/{env_id}/databases", m.Handler.List)
	r.Get("/projects/{project_id}/environments/{env_id}/databases/new", m.Handler.CreatePage)
	r.Post("/projects/{project_id}/environments/{env_id}/databases", m.Handler.Create)
	r.Get("/databases/{id}", m.Handler.Get)
	r.Post("/databases/{id}/start", m.Handler.Start)
	r.Post("/databases/{id}/stop", m.Handler.Stop)
	r.Post("/databases/{id}/delete", m.Handler.Delete)
	r.Post("/databases/{id}/resources", m.Handler.UpdateResourceLimits)
	r.Get("/databases/{id}/status", m.Handler.Status)
	r.Post("/databases/{id}/backups/config", m.Handler.UpdateBackupConfig)
	r.Post("/databases/{id}/backups/create", m.Handler.CreateBackup)
	r.Get("/databases/{id}/backups/{backup_id}/download", m.Handler.DownloadBackup)
	r.Post("/databases/{id}/backups/{backup_id}/delete", m.Handler.DeleteBackup)
}
