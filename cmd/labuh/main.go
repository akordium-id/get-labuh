package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/a-h/templ"
	"github.com/akordium-id/get-labuh/internal/api"
	"github.com/akordium-id/get-labuh/internal/auth"
	"github.com/akordium-id/get-labuh/internal/caddy"
	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/encryption"
	"github.com/akordium-id/get-labuh/internal/handler"
	"github.com/akordium-id/get-labuh/internal/middleware"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/observability"
	"github.com/akordium-id/get-labuh/internal/runner"
	"github.com/akordium-id/get-labuh/internal/server"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages"
	"github.com/akordium-id/get-labuh/internal/web/pages/compose"
	"github.com/akordium-id/get-labuh/internal/web/pages/databases"
	"github.com/akordium-id/get-labuh/internal/web/pages/projects"
	"github.com/akordium-id/get-labuh/internal/worker"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "runner" {
		runRunnerMode(os.Args[2:])
		return
	}
	if os.Getenv("LABUH_MODE") == "runner" {
		runRunnerMode(os.Args[1:])
		return
	}

	ctx := context.Background()

	dbPath := os.Getenv("DATABASE_URL")
	if dbPath == "" {
		dbPath = "labuh.db"
	}
	db, err := database.Connect(dbPath)
	if err != nil {
		slog.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := database.RunAllMigrations(db); err != nil {
		slog.Error("Failed to run migrations", "error", err)
		os.Exit(1)
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
	settingRepo := repo.NewSettingRepo(db)
	auditRepo := repo.NewAuditLogRepo(db)

	_ = settingRepo.InitializeDefaults()

	_ = encryption.Init("LABUH_MASTER_KEY")

	caddyAPIURL, _ := settingRepo.Get("caddy_api_url")
	caddyAPIKey, _ := settingRepo.Get("caddy_api_key")
	caddyClient := caddy.NewClient(caddyAPIURL, caddyAPIKey)

	seedTemplates(templateRepo)

	apiKeysHandler := handler.NewAPIKeysHandler(apiKeyRepo, userRepo)
	settingsHandler := handler.NewSettingsHandler(settingRepo)
	backupDir := os.Getenv("LABUH_BACKUP_DIR")
	if backupDir == "" {
		if os.Geteuid() == 0 {
			backupDir = "/var/lib/labuh/backups"
		} else {
			backupDir = "./backups"
		}
	}
	backupHandler := handler.NewBackupHandler(settingRepo, backupDir)
	auditHandler := handler.NewAuditHandler(auditRepo, userRepo)

	authMiddleware := auth.RequireAuth(sessionRepo, userRepo, false)

	apiKeyAuthMiddleware := api.NewAPIKeyAuthMiddleware(apiKeyRepo, userRepo)

	dockerClient, err := docker.NewClient()
	if err != nil {
		slog.Error("Failed to connect to Docker", "error", err)
		os.Exit(1)
	}

	authHandler := handler.NewAuthHandler(userRepo, sessionRepo, false)
	projectsHandler := handler.NewProjectsHandler(projectRepo)
	applicationsHandler := handler.NewApplicationsHandler(appRepo, projectRepo, deployRepo, envVarRepo, caddyClient, settingRepo, dockerClient)
	deploymentsHandler := handler.NewDeploymentsHandler(deployRepo, appRepo)
	logsHandler := handler.NewLogsHandler(deployRepo, appRepo)
	logsHandler.SetDockerClient(dockerClient)
	composeHandler := handler.NewComposeHandler(composeRepo, projectRepo, caddyClient, settingRepo)
	databasesHandler := handler.NewDatabasesHandler(databaseRepo, projectRepo)
	deployKeysHandler := handler.NewDeployKeysHandler(deployKeyRepo, projectRepo)
	webhooksHandler := handler.NewWebhooksHandler(appRepo, deployRepo)
	monitoringHandler := handler.NewMonitoringHandler(appRepo, databaseRepo, nil)
	templatesHandler := handler.NewTemplatesHandler(templateRepo, appRepo, projectRepo, deployRepo, envVarRepo)

	deployWorker := worker.NewDeployWorker(10)
	deployWorker.SetMaxConcurrent(3)
	deployWorker.SetWorkerCount(1)
	deployWorker.Start(dockerClient, caddyClient, settingRepo, appRepo, deployRepo)
	defer deployWorker.Stop()

	applicationsHandler.SetDeployWorker(deployWorker)
	webhooksHandler.SetDeployWorker(deployWorker)
	webhooksHandler.SetEnvVarRepo(envVarRepo)

	r := chi.NewRouter()
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(observability.RequestIDMiddleware)
	r.Use(middleware.SecurityHeadersMiddleware)
	r.Use(middleware.RateLimitMiddleware)
	r.Use(middleware.CSRFProtectionMiddleware)
	r.Use(middleware.AuditLoggingMiddleware(auditRepo))
	r.Use(middleware.CacheMiddleware)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})
	r.Get("/metrics", observability.MetricsHandler)

	apiAppHandler := api.NewApplicationsHandler(appRepo, projectRepo, deployRepo)
	apiAppHandler.SetDeployWorker(deployWorker)
	apiAppHandler.SetEnvVarRepo(envVarRepo)

	apiRouter := api.NewRouter(
		api.NewProjectsHandler(projectRepo),
		apiAppHandler,
		api.NewDeploymentsHandler(deployRepo, appRepo),
		apiKeyAuthMiddleware,
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
		r.Post("/applications/{id}/domain", applicationsHandler.SetDomain)
		r.Post("/applications/{id}/domain/remove", applicationsHandler.RemoveDomain)
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
		r.Post("/compose-apps/{id}/domain", composeHandler.SetDomain)
		r.Post("/compose-apps/{id}/domain/remove", composeHandler.RemoveDomain)

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

		r.Get("/settings/caddy", settingsHandler.Caddy)
		r.Post("/settings/caddy", settingsHandler.UpdateCaddy)
		r.Post("/settings/caddy/test", settingsHandler.TestCaddy)

		r.Get("/settings/backup", backupHandler.BackupPage)
		r.Post("/settings/backup/create", backupHandler.CreateBackup)
		r.Get("/settings/backup/download", backupHandler.DownloadBackup)
		r.Post("/settings/restore", backupHandler.RestoreBackup)

		r.Get("/settings/audit-logs", auditHandler.AuditLogsPage)
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

	tlsEnabled := os.Getenv("LABUH_TLS_ENABLED") == "true"
	tlsCert := os.Getenv("LABUH_TLS_CERT_PATH")
	tlsKey := os.Getenv("LABUH_TLS_KEY_PATH")

	profilingAddr := os.Getenv("LABUH_PPROF_ADDR")
	_ = observability.SetupProfiling(profilingAddr)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	if tlsEnabled && tlsCert != "" && tlsKey != "" {
		slog.Info("Labuh server starting with TLS", "port", port)
		if err := srv.ListenAndServeTLS(tlsCert, tlsKey); err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	} else {
		slog.Info("Labuh server starting", "port", port)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	_ = server.GracefulShutdown(shutdownCtx, srv, deployWorker, db)
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

func runRunnerMode(args []string) {
	fs := flag.NewFlagSet("runner", flag.ExitOnError)
	hubURL := fs.String("hub-url", os.Getenv("LABUH_HUB_URL"), "WebSocket URL of the SaaS Control Plane Hub (e.g. wss://hub.labuh.id/v1/tunnel)")
	token := fs.String("token", os.Getenv("LABUH_NODE_TOKEN"), "Secret authentication token for this node")
	heartbeatSec := fs.Int("heartbeat", 15, "Heartbeat interval in seconds")
	caddyURL := fs.String("caddy-url", os.Getenv("CADDY_API_URL"), "Caddy Admin API URL")
	caddyKey := fs.String("caddy-key", os.Getenv("CADDY_API_KEY"), "Caddy Admin API Key")
	_ = fs.Parse(args)

	if *hubURL == "" {
		slog.Error("missing required hub-url (specify --hub-url flag or LABUH_HUB_URL env var)")
		os.Exit(1)
	}
	if *token == "" {
		slog.Error("missing required node token (specify --token flag or LABUH_NODE_TOKEN env var)")
		os.Exit(1)
	}

	slog.Info("starting Labuh in Runner Agent mode (BYOS)")

	dockerClient, err := docker.NewClient()
	if err != nil {
		slog.Error("failed to connect to Docker", "error", err)
		os.Exit(1)
	}

	var caddyClient *caddy.Client
	if *caddyURL != "" {
		caddyClient = caddy.NewClient(*caddyURL, *caddyKey)
	}

	executor := runner.NewExecutor(dockerClient, caddyClient)
	collector := runner.NewMetricsCollector(dockerClient)

	cfg := runner.Config{
		HubURL:            *hubURL,
		NodeToken:         *token,
		Version:           "1.0.0",
		HeartbeatInterval: time.Duration(*heartbeatSec) * time.Second,
	}

	client := runner.NewClient(cfg, executor, collector)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := client.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("runner stopped with error", "error", err)
		os.Exit(1)
	}
	slog.Info("runner shut down gracefully")
}

