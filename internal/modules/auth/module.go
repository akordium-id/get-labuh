package auth

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Module struct {
	UserRepo    *UserRepo
	SessionRepo *SessionRepo
	APIKeyRepo  *APIKeyRepo
	Handler     *Handler
	RequireAuth func(http.Handler) http.Handler
}

func New(db *sql.DB, secure bool) *Module {
	userRepo := NewUserRepo(db)
	sessionRepo := NewSessionRepo(db)
	apiKeyRepo := NewAPIKeyRepo(db)
	handler := NewHandler(userRepo, sessionRepo, apiKeyRepo, secure)
	requireAuth := RequireAuth(sessionRepo, userRepo, secure)

	return &Module{
		UserRepo:    userRepo,
		SessionRepo: sessionRepo,
		APIKeyRepo:  apiKeyRepo,
		Handler:     handler,
		RequireAuth: requireAuth,
	}
}

// RegisterPublicRoutes mendaftarkan rute login/logout publik
func (m *Module) RegisterPublicRoutes(r chi.Router) {
	r.Get("/auth/login", m.Handler.LoginPage)
	r.Post("/auth/login", m.Handler.Login)
	r.Get("/auth/register", m.Handler.RegisterPage)
	r.Post("/auth/register", m.Handler.Register)
	r.Get("/auth/logout", m.Handler.Logout)
}

// RegisterProtectedRoutes mendaftarkan rute manajemen API Key
func (m *Module) RegisterProtectedRoutes(r chi.Router) {
	r.Get("/settings/api-keys", m.Handler.ListAPIKeys)
	r.Post("/settings/api-keys/create", m.Handler.CreateAPIKey)
	r.Post("/settings/api-keys/{id}/delete", m.Handler.DeleteAPIKey)
}
