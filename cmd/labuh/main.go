package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/a-h/templ"
	"github.com/akordium-id/get-labuh/internal/api"
	"github.com/akordium-id/get-labuh/internal/auth"
	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/handler"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/observability"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages"
	"github.com/akordium-id/get-labuh/internal/web/pages/compose"
	"github.com/akordium-id/get-labuh/internal/web/pages/databases"
	"github.com/akordium-id/get-labuh/internal/web/pages/projects"
	"github.com/akordium-id/get-labuh/internal/worker"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	ctx := context.Background()

	db, err := database.Connect("labuh.db")
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	if err := database.RunAllMigrations(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	userRepo := repo.NewUserRepo(db)
	sessionRepo := repo.NewSessionRepo(db)
	projectRepo := repo.NewProjectRepo(db)
	appRepo := repo.NewApplicationRepo(db)
	deployRepo := repo.NewDeploymentRepo(db)
	envVarRepo := repo.NewEnvVarRepo(db)
	composeRepo := repo.NewComposeRepo(db)
	databaseRepo := repo.NewDatabaseRepo(db)
	deployKeyRepo := repo.NewDeployKeyRepo(db)
	templateRepo := repo.NewTemplateRepo(db)
	apiKeyRepo := repo.NewApiKeyRepo(db)

	seedTemplates(templateRepo)

	authHandler := handler.NewAuthHandler(userRepo, sessionRepo, false)
	projectsHandler := handler.NewProjectsHandler(projectRepo)
	applicationsHandler := handler.NewApplicationsHandler(appRepo, projectRepo, deployRepo, envVarRepo)
	deploymentsHandler := handler.NewDeploymentsHandler(deployRepo, appRepo)
	logsHandler := handler.NewLogsHandler(deployRepo, appRepo)
	composeHandler := handler.NewComposeHandler(composeRepo, projectRepo)
	databasesHandler := handler.NewDatabasesHandler(databaseRepo, projectRepo)
	deployKeysHandler := handler.NewDeployKeysHandler(deployKeyRepo, projectRepo)
	webhooksHandler := handler.NewWebhooksHandler(appRepo, deployRepo)
	monitoringHandler := handler.NewMonitoringHandler(appRepo, databaseRepo, nil)
	templatesHandler := handler.NewTemplatesHandler(templateRepo, appRepo, projectRepo, deployRepo, envVarRepo)
	apiKeysHandler := handler.NewAPIKeysHandler(apiKeyRepo, userRepo)

	authMiddleware := auth.RequireAuth(sessionRepo, userRepo, false)

	api.NewAPIKeyAuthMiddleware(apiKeyRepo, userRepo)

	dockerClient, err := docker.NewClient()
	if err != nil {
		log.Fatalf("Failed to connect to Docker: %v", err)
	}

	deployWorker := worker.NewDeployWorker(10)
	deployWorker.Start(dockerClient)
	defer deployWorker.Stop()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})
	r.Get("/metrics", observability.MetricsHandler)

	apiRouter := api.NewRouter(
		api.NewProjectsHandler(projectRepo),
		api.NewApplicationsHandler(appRepo, projectRepo, deployRepo),
		api.NewDeploymentsHandler(deployRepo, appRepo),
	)
	r.Mount("/api/v1", apiRouter)

	r.Group(func(r chi.Router) {
		r.Get("/auth/login", authHandler.LoginPage)
		r.Post("/auth/login", authHandler.Login)
		r.Get("/auth/logout", authHandler.Logout)
	})

	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Get("/dashboard", func(w http.ResponseWriter, r *http.Request) {
			user, _ := auth.UserFromContext(r.Context())
			if user == nil {
				http.Redirect(w, r, "/auth/login", http.StatusFound)
				return
			}
			templ.Handler(layouts.AppLayout(pages.DashboardPage(user.Name))).ServeHTTP(w, r)
		})
		r.Get("/projects", projectsHandler.List)
		r.Post("/projects", projectsHandler.Create)
		r.Get("/projects/{id}", projectsHandler.Get)
		r.Post("/projects/{id}/delete", projectsHandler.Delete)
		r.Get("/projects/create", func(w http.ResponseWriter, r *http.Request) {
			templ.Handler(layouts.AppLayout(projects.ProjectCreatePage())).ServeHTTP(w, r)
		})
		r.Get("/projects/{id}/deploy-keys", deployKeysHandler.List)
		r.Post("/projects/{id}/deploy-keys", deployKeysHandler.Create)
		r.Post("/deploy-keys/{id}/delete", deployKeysHandler.Delete)

		r.Get("/templates", templatesHandler.List)
		r.Get("/templates/{id}", templatesHandler.Get)
		r.Post("/templates/{id}/deploy", templatesHandler.Deploy)

		r.Get("/projects/{project_id}/environments/{env_id}/applications", applicationsHandler.List)
		r.Post("/projects/{project_id}/environments/{env_id}/applications", applicationsHandler.Create)
		r.Get("/applications/{id}", applicationsHandler.Get)
		r.Post("/applications/{id}/start", applicationsHandler.Start)
		r.Post("/applications/{id}/stop", applicationsHandler.Stop)
		r.Post("/applications/{id}/restart", applicationsHandler.Restart)
		r.Post("/applications/{id}/deploy", applicationsHandler.Deploy)
		r.Post("/applications/{id}/env-vars", applicationsHandler.CreateEnvVar)
		r.Post("/applications/{id}/env-vars/{var_id}/delete", applicationsHandler.DeleteEnvVar)
		r.Get("/applications/{id}/stats", monitoringHandler.ApplicationStats)

		r.Get("/projects/{project_id}/environments/{env_id}/compose-apps", composeHandler.List)
		r.Get("/projects/{project_id}/environments/{env_id}/compose-apps/new", func(w http.ResponseWriter, r *http.Request) {
			templ.Handler(layouts.AppLayout(compose.ComposeCreatePage())).ServeHTTP(w, r)
		})
		r.Post("/projects/{project_id}/environments/{env_id}/compose-apps", composeHandler.Create)
		r.Get("/compose-apps/{id}", composeHandler.Get)
		r.Post("/compose-apps/{id}/deploy", composeHandler.Deploy)
		r.Post("/compose-apps/{id}/stop", composeHandler.Stop)
		r.Post("/compose-apps/{id}/start", composeHandler.Start)

		r.Get("/projects/{project_id}/environments/{env_id}/databases", databasesHandler.List)
		r.Get("/projects/{project_id}/environments/{env_id}/databases/new", func(w http.ResponseWriter, r *http.Request) {
			templ.Handler(layouts.AppLayout(databases.DatabaseCreatePage())).ServeHTTP(w, r)
		})
		r.Post("/projects/{project_id}/environments/{env_id}/databases", databasesHandler.Create)
		r.Get("/databases/{id}", databasesHandler.Get)
		r.Post("/databases/{id}/start", databasesHandler.Start)
		r.Post("/databases/{id}/stop", databasesHandler.Stop)
		r.Post("/databases/{id}/delete", databasesHandler.Delete)
		r.Get("/databases/{id}/stats", monitoringHandler.DatabaseStats)

		r.Get("/deployments/{id}", deploymentsHandler.Get)
		r.Get("/deployments/{id}/logs", deploymentsHandler.Logs)
		r.Get("/deployments/{id}/logs/stream", logsHandler.StreamDeploymentLogs)
		r.Get("/applications/{id}/logs/stream", logsHandler.StreamContainerLogs)

		r.Get("/settings/api-keys", apiKeysHandler.List)
		r.Post("/settings/api-keys/create", apiKeysHandler.Create)
		r.Post("/settings/api-keys/{id}/delete", apiKeysHandler.Delete)
	})

	r.Post("/deployments/{id}/start", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if id == "" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		deployment, err := deployRepo.GetByID(id)
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		logPath := "/tmp/labuh-logs/" + deployment.ID + ".log"
		_ = deployRepo.MarkStarted(deployment.ID)
		_ = deployRepo.UpdateStatus(deployment.ID, models.DeployStatusCloning)
		_ = logPath
		_ = appRepo

		w.Header().Set("HX-Redirect", "/deployments/"+id)
		w.WriteHeader(http.StatusOK)
	})

	r.Post("/webhooks/deploy/{application_id}", webhooksHandler.Deploy)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	log.Printf("Labuh server starting on :%s", port)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	<-ctx.Done()
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
}

func seedTemplates(templateRepo *repo.TemplateRepo) {
	templates, _ := templateRepo.GetAll()
	if len(templates) > 0 {
		return
	}

	templatesToSeed := []models.CreateServiceTemplateInput{
		{
			Name:          "WordPress",
			Description:   new("WordPress is a free and open-source content management system."),
			SourceType:    models.TemplateSourceGit,
			RepositoryURL: new("https://github.com/wordpress/wordpress"),
			Category:      models.TemplateCategoryCMS,
			IsOfficial:    true,
		},
		{
			Name:          "Ghost",
			Description:   new("Ghost is a modern, professional publishing platform."),
			SourceType:    models.TemplateSourceGit,
			RepositoryURL: new("https://github.com/TryGhost/Ghost"),
			Category:      models.TemplateCategoryCMS,
			IsOfficial:    true,
		},
		{
			Name:        "Grafana",
			Description: new("Grafana is an open source analytics and interactive visualization web platform."),
			SourceType:  models.TemplateSourceDockerImage,
			DockerImage: new("grafana/grafana"),
			Category:    models.TemplateCategoryMonitoring,
			IsOfficial:  true,
		},
		{
			Name:        "Minio",
			Description: new("MinIO is a high-performance, S3 compatible object store."),
			SourceType:  models.TemplateSourceDockerImage,
			DockerImage: new("minio/minio"),
			Category:    models.TemplateCategoryStorage,
			IsOfficial:  true,
		},
	}

	for _, t := range templatesToSeed {
		_, _ = templateRepo.Create(t)
	}
}

//go:fix inline
func strPtr(s string) *string {
	return new(s)
}
