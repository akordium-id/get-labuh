package app

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/akordium-id/get-labuh/internal/api"
	"github.com/akordium-id/get-labuh/internal/auth"
	"github.com/akordium-id/get-labuh/internal/middleware"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/observability"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages"
	"github.com/akordium-id/get-labuh/internal/web/pages/compose"
	"github.com/akordium-id/get-labuh/internal/web/pages/databases"
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
		r.Post("/lite/deploy/upload", c.DropzoneHandler.UploadDropzone)

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

		r.Get("/projects/{project_id}/environments/{env_id}/applications", c.ApplicationsHandler.List)
		r.Post("/projects/{project_id}/environments/{env_id}/applications", c.ApplicationsHandler.Create)
		r.Get("/applications/{id}", c.ApplicationsHandler.Get)
		r.Post("/applications/{id}/start", c.ApplicationsHandler.Start)
		r.Post("/applications/{id}/stop", c.ApplicationsHandler.Stop)
		r.Post("/applications/{id}/restart", c.ApplicationsHandler.Restart)
		r.Post("/applications/{id}/deploy", c.ApplicationsHandler.Deploy)
		r.Post("/applications/{id}/env-vars", c.ApplicationsHandler.CreateEnvVar)
		r.Post("/applications/{id}/env-vars/{var_id}/delete", c.ApplicationsHandler.DeleteEnvVar)
		r.Post("/applications/{id}/domain", c.ApplicationsHandler.SetDomain)
		r.Post("/applications/{id}/domain/remove", c.ApplicationsHandler.RemoveDomain)
		r.Post("/applications/{id}/resources", c.ApplicationsHandler.UpdateResourceLimits)
		r.Post("/applications/{id}/webhook/regenerate", c.ApplicationsHandler.RegenerateWebhookSecret)
		r.Get("/applications/{id}/status", c.ApplicationsHandler.Status)
		r.Get("/applications/{id}/stats", c.MonitoringHandler.ApplicationStats)

		r.Get("/projects/{project_id}/environments/{env_id}/compose-apps", c.ComposeHandler.List)
		r.Get("/projects/{project_id}/environments/{env_id}/compose-apps/new", func(w http.ResponseWriter, r *http.Request) {
			templ.Handler(layouts.AppLayout(compose.ComposeCreatePage())).ServeHTTP(w, r)
		})
		r.Post("/projects/{project_id}/environments/{env_id}/compose-apps", c.ComposeHandler.Create)
		r.Get("/compose-apps/{id}", c.ComposeHandler.Get)
		r.Post("/compose-apps/{id}/deploy", c.ComposeHandler.Deploy)
		r.Post("/compose-apps/{id}/stop", c.ComposeHandler.Stop)
		r.Post("/compose-apps/{id}/start", c.ComposeHandler.Start)
		r.Post("/compose-apps/{id}/domain", c.ComposeHandler.SetDomain)
		r.Post("/compose-apps/{id}/domain/remove", c.ComposeHandler.RemoveDomain)

		r.Get("/projects/{project_id}/environments/{env_id}/databases", c.DatabasesHandler.List)
		r.Get("/projects/{project_id}/environments/{env_id}/databases/new", func(w http.ResponseWriter, r *http.Request) {
			templ.Handler(layouts.AppLayout(databases.DatabaseCreatePage())).ServeHTTP(w, r)
		})
		r.Post("/projects/{project_id}/environments/{env_id}/databases", c.DatabasesHandler.Create)
		r.Get("/databases/{id}", c.DatabasesHandler.Get)
		r.Post("/databases/{id}/start", c.DatabasesHandler.Start)
		r.Post("/databases/{id}/stop", c.DatabasesHandler.Stop)
		r.Post("/databases/{id}/delete", c.DatabasesHandler.Delete)
		r.Post("/databases/{id}/resources", c.DatabasesHandler.UpdateResourceLimits)
		r.Get("/databases/{id}/status", c.DatabasesHandler.Status)
		r.Get("/databases/{id}/stats", c.MonitoringHandler.DatabaseStats)
		r.Get("/databases/{id}/terminal", c.TerminalHandler.DatabaseTerminal)
		r.Post("/databases/{id}/backups/config", c.DatabasesHandler.UpdateBackupConfig)
		r.Post("/databases/{id}/backups/create", c.DatabasesHandler.CreateBackup)
		r.Get("/databases/{id}/backups/{backup_id}/download", c.DatabasesHandler.DownloadBackup)
		r.Post("/databases/{id}/backups/{backup_id}/delete", c.DatabasesHandler.DeleteBackup)

		r.Get("/applications/{id}/terminal", c.TerminalHandler.ApplicationTerminal)
		r.Get("/containers/{container_id}/terminal/ws", c.TerminalHandler.ContainerTerminalWS)

		r.Get("/deployments/{id}", c.DeploymentsHandler.Get)
		r.Get("/deployments/{id}/logs", c.DeploymentsHandler.Logs)
		r.Get("/deployments/{id}/logs/stream", c.LogsHandler.StreamDeploymentLogs)
		r.Get("/applications/{id}/logs/stream", c.LogsHandler.StreamContainerLogs)

		c.AuthMod.RegisterProtectedRoutes(r)
		c.NotificationsMod.RegisterRoutes(r)
		c.SettingsMod.RegisterRoutes(r)
	})

	r.Post("/deployments/{id}/start", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		deployment, err := c.DeployRepo.GetByID(id)
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		_ = c.DeployRepo.MarkStarted(deployment.ID)
		_ = c.DeployRepo.UpdateStatus(deployment.ID, models.DeployStatusCloning)

		w.Header().Set("HX-Redirect", "/deployments/"+id)
		w.WriteHeader(http.StatusOK)
	})

	r.Post("/webhooks/deploy/{application_id}", c.WebhooksHandler.Deploy)
	r.Post("/webhooks/github/{application_id}", c.WebhooksHandler.GitHub)
	r.Post("/webhooks/gitlab/{application_id}", c.WebhooksHandler.GitLab)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})

	return r
}
