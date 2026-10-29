package notifications

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
)

// Module mengemas repository, notifier engine, dan HTTP handler notifikasi.
type Module struct {
	Repo     *Repo
	Notifier *Notifier
	Handler  *Handler
}

func New(db *sql.DB, projectProvider ProjectProvider) *Module {
	repo := NewRepo(db)
	notifier := NewNotifier(repo)
	handler := NewHandler(repo, projectProvider, notifier)

	return &Module{
		Repo:     repo,
		Notifier: notifier,
		Handler:  handler,
	}
}

// RegisterRoutes mendaftarkan rute konfigurasi notifikasi ke Chi router
func (m *Module) RegisterRoutes(r chi.Router) {
	r.Get("/settings/notifications", m.Handler.List)
	r.Post("/settings/notifications", m.Handler.Create)
	r.Post("/settings/notifications/{id}/delete", m.Handler.Delete)
	r.Post("/settings/notifications/{id}/test", m.Handler.Test)
}
