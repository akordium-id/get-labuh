package app

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/akordium-id/get-labuh/internal/api"
	"github.com/akordium-id/get-labuh/internal/auth"
	"github.com/akordium-id/get-labuh/internal/middleware"
	"github.com/akordium-id/get-labuh/internal/observability"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages"
	"github.com/akordium-id/get-labuh/internal/web/pages/lite"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// SetupRoutes mendefinisikan rute dan middleware HTTP utama aplikasi.
func SetupRoutes(c *Container) http.Handler {
	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(observability.RequestIDMiddleware)
	r.Use(middleware.SecurityHeadersMiddleware)
	r.Use(middleware.RateLimitMiddleware)
	r.Use(middleware.CSRFProtectionMiddleware)
	r.Use(middleware.AuditLoggingMiddleware(c.AuditRepo))
	r.Use(middleware.CacheMiddleware)
	r.Use(middleware.ModeMiddleware)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy"}`))
	})
	r.Get("/metrics", observability.MetricsHandler)

	apiRouter := api.NewRouter(
		c.APIProjectsHandler,
		c.APIAppHandler,
		c.APIDeploymentsHandler,
		c.APIKeyAuthMiddleware,
	)
	r.Mount("/api/v1", apiRouter)

	c.AuthMod.RegisterPublicRoutes(r)

	r.Group(func(r chi.Router) {
		r.Use(c.AuthMiddleware)

		r.Get("/dashboard", func(w http.ResponseWriter, r *http.Request) {
			user, _ := auth.UserFromContext(r.Context())
			if user == nil {
				http.Redirect(w, r, "/auth/login", http.StatusFound)
				return
			}

			userMode := middleware.GetUserMode(r.Context())
			if userMode == middleware.ModeLite {
				apps, _ := c.AppRepo.GetAll()
				templ.Handler(layouts.AppLayout(lite.LiteDashboardPage(user.Name, apps), userMode)).ServeHTTP(w, r)
				return
			}

			templ.Handler(layouts.AppLayout(pages.DashboardPage(user.Name), userMode)).ServeHTTP(w, r)
		})
		c.ProjectsMod.RegisterRoutes(r)

		r.Get("/templates", c.TemplatesHandler.List)
		r.Get("/templates/{id}", c.TemplatesHandler.Get)
		r.Post("/templates/{id}/deploy", c.TemplatesHandler.Deploy)

		c.ApplicationsMod.RegisterRoutes(r)

		c.ComposeMod.RegisterRoutes(r)
		c.DatabasesMod.RegisterRoutes(r)
		r.Get("/databases/{id}/stats", c.MonitoringHandler.DatabaseStats)
		r.Get("/databases/{id}/terminal", c.TerminalHandler.DatabaseTerminal)

		r.Get("/applications/{id}/terminal", c.TerminalHandler.ApplicationTerminal)
		r.Get("/containers/{container_id}/terminal/ws", c.TerminalHandler.ContainerTerminalWS)

		c.DeploymentsMod.RegisterRoutes(r)

		c.AuthMod.RegisterProtectedRoutes(r)
		c.NotificationsMod.RegisterRoutes(r)
		c.SettingsMod.RegisterRoutes(r)

		if c.ServersHandler != nil {
			r.Get("/settings/servers", c.ServersHandler.List)
			r.Get("/settings/servers/new", c.ServersHandler.CreateForm)
			r.Post("/settings/servers", c.ServersHandler.Create)
			r.Post("/settings/servers/{id}/test", c.ServersHandler.Test)
			r.Post("/settings/servers/{id}/delete", c.ServersHandler.Delete)
		}
	})

	c.ApplicationsMod.RegisterPublicRoutes(r)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})

	return r
}
