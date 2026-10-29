package app

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/akordium-id/get-labuh/internal/api"
	"github.com/akordium-id/get-labuh/internal/auth"
	"github.com/akordium-id/get-labuh/internal/backup"
	"github.com/akordium-id/get-labuh/internal/caddy"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/encryption"
	"github.com/akordium-id/get-labuh/internal/handler"
	authmod "github.com/akordium-id/get-labuh/internal/modules/auth"
	"github.com/akordium-id/get-labuh/internal/modules/notifications"
	"github.com/akordium-id/get-labuh/internal/worker"
)

// Container menyimpan semua instans handler, repo, dan worker aplikasi.
type Container struct {
	AuthMod                    *authmod.Module
	AuthMiddleware             func(http.Handler) http.Handler
	APIKeyAuthMiddleware       *api.APIKeyAuthMiddleware
	ProjectsHandler            *handler.ProjectsHandler
	ApplicationsHandler        *handler.ApplicationsHandler
	DeploymentsHandler         *handler.DeploymentsHandler
	LogsHandler                *handler.LogsHandler
	ComposeHandler             *handler.ComposeHandler
	DatabasesHandler           *handler.DatabasesHandler
	StorageDestinationsHandler *handler.StorageDestinationsHandler
	DeployKeysHandler          *handler.DeployKeysHandler
	WebhooksHandler            *handler.WebhooksHandler
	MonitoringHandler          *handler.MonitoringHandler
	TemplatesHandler           *handler.TemplatesHandler
	TerminalHandler            *handler.TerminalHandler
	NotificationsMod           *notifications.Module
	DropzoneHandler            *handler.DropzoneHandler
	SettingsHandler            *handler.SettingsHandler
	BackupHandler              *handler.BackupHandler
	AuditHandler               *handler.AuditHandler

	APIAppHandler         *api.ApplicationsHandler
	APIProjectsHandler    *api.ProjectsHandler
	APIDeploymentsHandler *api.DeploymentsHandler

	AppRepo    *repo.ApplicationRepo
	DeployRepo *repo.DeploymentRepo
	AuditRepo  *repo.AuditLogRepo

	DeployWorker    *worker.DeployWorker
	BackupScheduler *backup.BackupScheduler
}

// BuildContainer melakukan wiring dependency injection untuk seluruh repository, service, worker, dan handler.
func BuildContainer(db *sql.DB, cfg Config) (*Container, error) {
	authMod := authmod.New(db, false)
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
	storageRepo := repo.NewStorageDestinationRepo(db)
	databaseBackupRepo := repo.NewDatabaseBackupRepo(db)
	notificationsMod := notifications.New(db, projectRepo)

	_ = settingRepo.InitializeDefaults()
	_ = encryption.Init(cfg.MasterKeyEnv)

	caddyAPIURL, _ := settingRepo.Get("caddy_api_url")
	caddyAPIKey, _ := settingRepo.Get("caddy_api_key")
	caddyClient := caddy.NewClient(caddyAPIURL, caddyAPIKey)

	SeedTemplates(templateRepo)

	settingsHandler := handler.NewSettingsHandler(settingRepo)
	backupHandler := handler.NewBackupHandler(settingRepo, cfg.BackupDir)
	auditHandler := handler.NewAuditHandler(auditRepo, userRepo)

	authMiddleware := auth.RequireAuth(sessionRepo, userRepo, false)
	apiKeyAuthMiddleware := api.NewAPIKeyAuthMiddleware(apiKeyRepo, userRepo)

	dockerClient, err := docker.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker: %w", err)
	}

	projectsHandler := handler.NewProjectsHandler(projectRepo)
	applicationsHandler := handler.NewApplicationsHandler(appRepo, projectRepo, deployRepo, envVarRepo, caddyClient, settingRepo, dockerClient)
	deploymentsHandler := handler.NewDeploymentsHandler(deployRepo, appRepo)
	logsHandler := handler.NewLogsHandler(deployRepo, appRepo)
	logsHandler.SetDockerClient(dockerClient)
	composeHandler := handler.NewComposeHandler(composeRepo, projectRepo, caddyClient, settingRepo)
	databasesHandler := handler.NewDatabasesHandler(databaseRepo, projectRepo)
	databasesHandler.SetDockerClient(dockerClient)
	storageDestinationsHandler := handler.NewStorageDestinationsHandler(storageRepo)

	databaseBackupRunner := backup.NewDatabaseBackupRunner(dockerClient, databaseBackupRepo, storageRepo, databaseRepo, cfg.BackupDir+"/databases")
	databaseBackupRunner.SetNotifier(notificationsMod.Notifier, projectRepo)
	databasesHandler.SetBackupDependencies(databaseBackupRepo, storageRepo, databaseBackupRunner)

	backupScheduler := backup.NewBackupScheduler(databaseBackupRunner)
	backupScheduler.Start()

	deployKeysHandler := handler.NewDeployKeysHandler(deployKeyRepo, projectRepo)
	webhooksHandler := handler.NewWebhooksHandler(appRepo, deployRepo)
	monitoringHandler := handler.NewMonitoringHandler(appRepo, databaseRepo, dockerClient)
	templatesHandler := handler.NewTemplatesHandler(templateRepo, appRepo, projectRepo, deployRepo, envVarRepo)
	terminalHandler := handler.NewTerminalHandler(dockerClient, appRepo, databaseRepo, composeRepo)

	deployWorker := worker.NewDeployWorker(10)
	deployWorker.SetMaxConcurrent(3)
	deployWorker.SetWorkerCount(1)
	deployWorker.SetNotifier(notificationsMod.Notifier, projectRepo)
	deployWorker.Start(dockerClient, caddyClient, settingRepo, appRepo, deployRepo)

	dropzoneHandler := handler.NewDropzoneHandler(appRepo, projectRepo, deployRepo, envVarRepo)
	dropzoneHandler.SetDeployWorker(deployWorker)
	applicationsHandler.SetDeployWorker(deployWorker)
	webhooksHandler.SetDeployWorker(deployWorker)
	webhooksHandler.SetEnvVarRepo(envVarRepo)

	apiAppHandler := api.NewApplicationsHandler(appRepo, projectRepo, deployRepo)
	apiAppHandler.SetDeployWorker(deployWorker)
	apiAppHandler.SetEnvVarRepo(envVarRepo)

	return &Container{
		AuthMod:                    authMod,
		AuthMiddleware:             authMiddleware,
		APIKeyAuthMiddleware:       apiKeyAuthMiddleware,
		ProjectsHandler:            projectsHandler,
		ApplicationsHandler:        applicationsHandler,
		DeploymentsHandler:         deploymentsHandler,
		LogsHandler:                logsHandler,
		ComposeHandler:             composeHandler,
		DatabasesHandler:           databasesHandler,
		StorageDestinationsHandler: storageDestinationsHandler,
		DeployKeysHandler:          deployKeysHandler,
		WebhooksHandler:            webhooksHandler,
		MonitoringHandler:          monitoringHandler,
		TemplatesHandler:           templatesHandler,
		TerminalHandler:            terminalHandler,
		NotificationsMod:           notificationsMod,
		DropzoneHandler:            dropzoneHandler,
		SettingsHandler:            settingsHandler,
		BackupHandler:              backupHandler,
		AuditHandler:               auditHandler,
		APIAppHandler:              apiAppHandler,
		APIProjectsHandler:         api.NewProjectsHandler(projectRepo),
		APIDeploymentsHandler:      api.NewDeploymentsHandler(deployRepo, appRepo),
		AppRepo:                    appRepo,
		DeployRepo:                 deployRepo,
		AuditRepo:                  auditRepo,
		DeployWorker:               deployWorker,
		BackupScheduler:            backupScheduler,
	}, nil
}
