package settings

import (
	"context"
	"database/sql"

	"github.com/go-chi/chi/v5"
)

type Module struct {
	SettingRepo *SettingRepo
	AuditRepo   *AuditLogRepo
	StorageRepo *StorageDestinationRepo
	Handler     *Handler
}

func New(db *sql.DB, backupPath string) *Module {
	settingRepo := NewSettingRepo(db)
	auditRepo := NewAuditLogRepo(db)
	storageRepo := NewStorageDestinationRepo(db)
	handler := NewHandler(settingRepo, auditRepo, storageRepo, backupPath)

	_ = settingRepo.InitializeDefaults(context.Background())

	return &Module{
		SettingRepo: settingRepo,
		AuditRepo:   auditRepo,
		StorageRepo: storageRepo,
		Handler:     handler,
	}
}

// RegisterRoutes mendaftarkan seluruh endpoint settings (Caddy, mode, audit logs, storage destinations, backup)
func (m *Module) RegisterRoutes(r chi.Router) {
	r.Post("/settings/mode", m.Handler.SwitchMode)

	r.Get("/settings/caddy", m.Handler.Caddy)
	r.Post("/settings/caddy", m.Handler.UpdateCaddy)
	r.Post("/settings/caddy/test", m.Handler.TestCaddy)

	r.Get("/settings/audit-logs", m.Handler.AuditLogsPage)

	r.Get("/settings/storage-destinations", m.Handler.StorageList)
	r.Post("/settings/storage-destinations", m.Handler.StorageCreate)
	r.Post("/settings/storage-destinations/{id}/default", m.Handler.StorageSetDefault)
	r.Post("/settings/storage-destinations/{id}/delete", m.Handler.StorageDelete)
	r.Post("/settings/storage-destinations/{id}/test", m.Handler.StorageTestConnection)

	r.Get("/settings/backup", m.Handler.BackupPage)
	r.Post("/settings/backup/create", m.Handler.CreateBackup)
	r.Get("/settings/backup/download", m.Handler.DownloadBackup)
	r.Post("/settings/restore", m.Handler.RestoreBackup)
}
