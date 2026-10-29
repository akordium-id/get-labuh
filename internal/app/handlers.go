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
	"github.com/akordium-id/get-labuh/internal/models"
	authmod "github.com/akordium-id/get-labuh/internal/modules/auth"
	composemod "github.com/akordium-id/get-labuh/internal/modules/compose"
	databasesmod "github.com/akordium-id/get-labuh/internal/modules/databases"
	deploymentsmod "github.com/akordium-id/get-labuh/internal/modules/deployments"
	"github.com/akordium-id/get-labuh/internal/modules/notifications"
	projectsmod "github.com/akordium-id/get-labuh/internal/modules/projects"
	settingsmod "github.com/akordium-id/get-labuh/internal/modules/settings"
	"github.com/akordium-id/get-labuh/internal/worker"
)

// Container menyimpan semua instans handler, repo, dan worker aplikasi.
type Container struct {
	AuthMod              *authmod.Module
	AuthMiddleware       func(http.Handler) http.Handler
	APIKeyAuthMiddleware *api.APIKeyAuthMiddleware
	ApplicationsHandler  *handler.ApplicationsHandler
	DeploymentsMod       *deploymentsmod.Module
	ComposeMod           *composemod.Module
	DatabasesMod         *databasesmod.Module
	WebhooksHandler      *handler.WebhooksHandler
	MonitoringHandler    *handler.MonitoringHandler
	TemplatesHandler     *handler.TemplatesHandler
	TerminalHandler      *handler.TerminalHandler
	ProjectsMod          *projectsmod.Module
	NotificationsMod     *notifications.Module
	SettingsMod          *settingsmod.Module
	DropzoneHandler      *handler.DropzoneHandler

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
	templateRepo := repo.NewTemplateRepo(db)
	apiKeyRepo := repo.NewApiKeyRepo(db)
	settingRepo := repo.NewSettingRepo(db)
	settingsMod := settingsmod.New(db, cfg.BackupDir)
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

	authMiddleware := auth.RequireAuth(sessionRepo, userRepo, false)
	apiKeyAuthMiddleware := api.NewAPIKeyAuthMiddleware(apiKeyRepo, userRepo)

	dockerClient, err := docker.NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Docker: %w", err)
	}

	projectsMod := projectsmod.New(db, userRepo)
	composeMod := composemod.New(db, caddyClient, settingsMod.SettingRepo)
	applicationsHandler := handler.NewApplicationsHandler(appRepo, projectRepo, deployRepo, envVarRepo, caddyClient, settingRepo, dockerClient)
	deploymentsMod := deploymentsmod.New(db, appRepo)
	deploymentsMod.SetDockerClient(dockerClient)
	databasesMod := databasesmod.New(db, projectRepo)
	databasesMod.SetDockerClient(dockerClient)

	databaseBackupRunner := backup.NewDatabaseBackupRunner(dockerClient, databaseBackupRepo, storageRepo, databaseRepo, cfg.BackupDir+"/databases")
	databaseBackupRunner.SetNotifier(notificationsMod.Notifier, projectRepo)
	databasesMod.SetBackupDependencies(settingsMod.StorageRepo, databaseBackupRunner, func(dest *models.StorageDestination) (databasesmod.S3Deleter, error) {
		return backup.NewS3ClientFromDestination(dest)
	})

	backupScheduler := backup.NewBackupScheduler(databaseBackupRunner)
	backupScheduler.Start()

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
		AuthMod:               authMod,
		AuthMiddleware:        authMiddleware,
		APIKeyAuthMiddleware:  apiKeyAuthMiddleware,
		ApplicationsHandler:   applicationsHandler,
		DeploymentsMod:        deploymentsMod,
		ComposeMod:            composeMod,
		DatabasesMod:          databasesMod,
		WebhooksHandler:       webhooksHandler,
		MonitoringHandler:     monitoringHandler,
		TemplatesHandler:      templatesHandler,
		TerminalHandler:       terminalHandler,
		ProjectsMod:           projectsMod,
		NotificationsMod:      notificationsMod,
		SettingsMod:           settingsMod,
		DropzoneHandler:       dropzoneHandler,
		APIAppHandler:         apiAppHandler,
		APIProjectsHandler:    api.NewProjectsHandler(projectRepo),
		APIDeploymentsHandler: api.NewDeploymentsHandler(deployRepo, appRepo),
		AppRepo:               appRepo,
		DeployRepo:            deployRepo,
		AuditRepo:             auditRepo,
		DeployWorker:          deployWorker,
		BackupScheduler:       backupScheduler,
	}, nil
}
