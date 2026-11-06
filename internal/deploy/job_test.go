package deploy

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/caddy"
	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
)

func TestPipeline_Execute(t *testing.T) {
	_ = os.Setenv("LABUH_DOCKER_MOCK", "true")

	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	defer db.Close()

	err = database.RunAllMigrations(db)
	require.NoError(t, err)

	appRepo := repo.NewApplicationRepo(db)
	deployRepo := repo.NewDeploymentRepo(db)
	settingRepo := repo.NewSettingRepo(db)
	dockerClient, err := docker.NewClient()
	require.NoError(t, err)
	caddyClient := caddy.NewClient("http://localhost:2019", "")

	pipeline := NewPipeline(dockerClient, caddyClient, settingRepo, appRepo, deployRepo)

	domain := "myapp.example.com"
	app, err := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "My App",
		Slug:          "my-app",
		SourceType:    models.SourceTypeDockerImage,
		DockerImage:   &domain,
		CustomDomain:  &domain,
		AppPort:       8080,
	})
	require.NoError(t, err)

	logPath := "/tmp/labuh-test-logs/deploy.log"
	deployment, err := deployRepo.Create(models.CreateDeploymentInput{
		ApplicationID: app.ID,
		LogPath:       &logPath,
	})
	require.NoError(t, err)

	job := DeploymentJob{
		DeploymentID:  deployment.ID,
		ApplicationID: app.ID,
		SourceType:    string(models.SourceTypeDockerImage),
		DockerImage:   "nginx:alpine",
		AppPort:       8080,
		LogPath:       logPath,
	}

	err = pipeline.Execute(context.Background(), job)
	assert.NoError(t, err)

	// Verify deployment status
	updatedDeploy, err := deployRepo.GetByID(deployment.ID)
	require.NoError(t, err)
	assert.Equal(t, models.DeployStatusSuccess, updatedDeploy.Status)
	require.NotNil(t, updatedDeploy.Step)
	assert.Equal(t, "done", *updatedDeploy.Step)

	// Verify application status updated to running
	updatedApp, err := appRepo.GetByID(app.ID)
	require.NoError(t, err)
	assert.Equal(t, models.AppStatusRunning, updatedApp.Status)
	assert.NotNil(t, updatedApp.ContainerID)
	assert.NotEmpty(t, *updatedApp.ContainerID)
}

func TestPipeline_PreDeployCommand_Success(t *testing.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, database.RunAllMigrations(db))

	appRepo := repo.NewApplicationRepo(db)
	deployRepo := repo.NewDeploymentRepo(db)
	settingRepo := repo.NewSettingRepo(db)
	dockerClient, err := docker.NewClient()
	require.NoError(t, err)
	caddyClient := caddy.NewClient("http://localhost:2019", "")
	pipeline := NewPipeline(dockerClient, caddyClient, settingRepo, appRepo, deployRepo)

	preCmd := "echo 'running migrations' && exit 0"
	app, err := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID:        "env-1",
		Name:                 "App With PreDeploy",
		Slug:                 "app-with-predeploy",
		SourceType:           models.SourceTypeDockerImage,
		AppPort:              80,
		PreDeploymentCommand: &preCmd,
	})
	require.NoError(t, err)

	logPath := "/tmp/labuh-test-logs/predeploy-success.log"
	deployment, err := deployRepo.Create(models.CreateDeploymentInput{
		ApplicationID: app.ID,
		LogPath:       &logPath,
	})
	require.NoError(t, err)

	job := DeploymentJob{
		DeploymentID:  deployment.ID,
		ApplicationID: app.ID,
		SourceType:    string(models.SourceTypeDockerImage),
		DockerImage:   "nginx:alpine",
		AppPort:       80,
		LogPath:       logPath,
	}

	err = pipeline.Execute(context.Background(), job)
	assert.NoError(t, err)

	updatedDeploy, err := deployRepo.GetByID(deployment.ID)
	require.NoError(t, err)
	assert.Equal(t, models.DeployStatusSuccess, updatedDeploy.Status)
	require.NotNil(t, updatedDeploy.Step)
	assert.Equal(t, "done", *updatedDeploy.Step)
}

func TestPipeline_PreDeployCommand_Failure(t *testing.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, database.RunAllMigrations(db))

	appRepo := repo.NewApplicationRepo(db)
	deployRepo := repo.NewDeploymentRepo(db)
	settingRepo := repo.NewSettingRepo(db)
	dockerClient, err := docker.NewClient()
	require.NoError(t, err)
	caddyClient := caddy.NewClient("http://localhost:2019", "")
	pipeline := NewPipeline(dockerClient, caddyClient, settingRepo, appRepo, deployRepo)

	preCmd := "echo 'migration failed' && exit 1"
	app, err := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID:        "env-1",
		Name:                 "App Failed PreDeploy",
		Slug:                 "app-failed-predeploy",
		SourceType:           models.SourceTypeDockerImage,
		AppPort:              80,
		PreDeploymentCommand: &preCmd,
	})
	require.NoError(t, err)

	logPath := "/tmp/labuh-test-logs/predeploy-fail.log"
	deployment, err := deployRepo.Create(models.CreateDeploymentInput{
		ApplicationID: app.ID,
		LogPath:       &logPath,
	})
	require.NoError(t, err)

	job := DeploymentJob{
		DeploymentID:  deployment.ID,
		ApplicationID: app.ID,
		SourceType:    string(models.SourceTypeDockerImage),
		DockerImage:   "nginx:alpine",
		AppPort:       80,
		LogPath:       logPath,
	}

	err = pipeline.Execute(context.Background(), job)
	assert.Error(t, err)

	var deployErr *DeployError
	require.ErrorAs(t, err, &deployErr)
	assert.Equal(t, "pre_deploy", deployErr.Step)

	updatedDeploy, err := deployRepo.GetByID(deployment.ID)
	require.NoError(t, err)
	assert.Equal(t, models.DeployStatusFailed, updatedDeploy.Status)
	require.NotNil(t, updatedDeploy.Step)
	assert.Equal(t, "pre_deploy", *updatedDeploy.Step)
}

func TestPipeline_PostDeployCommand_Failure(t *testing.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, database.RunAllMigrations(db))

	appRepo := repo.NewApplicationRepo(db)
	deployRepo := repo.NewDeploymentRepo(db)
	settingRepo := repo.NewSettingRepo(db)
	dockerClient, err := docker.NewClient()
	require.NoError(t, err)
	caddyClient := caddy.NewClient("http://localhost:2019", "")
	pipeline := NewPipeline(dockerClient, caddyClient, settingRepo, appRepo, deployRepo)

	postCmd := "echo 'post hook warning' && exit 1"
	app, err := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID:         "env-1",
		Name:                  "App PostDeploy Fail",
		Slug:                  "app-postdeploy-fail",
		SourceType:            models.SourceTypeDockerImage,
		AppPort:               80,
		PostDeploymentCommand: &postCmd,
	})
	require.NoError(t, err)

	logPath := "/tmp/labuh-test-logs/postdeploy-fail.log"
	deployment, err := deployRepo.Create(models.CreateDeploymentInput{
		ApplicationID: app.ID,
		LogPath:       &logPath,
	})
	require.NoError(t, err)

	job := DeploymentJob{
		DeploymentID:  deployment.ID,
		ApplicationID: app.ID,
		SourceType:    string(models.SourceTypeDockerImage),
		DockerImage:   "nginx:alpine",
		AppPort:       80,
		LogPath:       logPath,
	}

	// Post deployment failure is non-fatal: deployment should succeed
	err = pipeline.Execute(context.Background(), job)
	assert.NoError(t, err)

	updatedDeploy, err := deployRepo.GetByID(deployment.ID)
	require.NoError(t, err)
	assert.Equal(t, models.DeployStatusSuccess, updatedDeploy.Status)
	require.NotNil(t, updatedDeploy.Step)
	assert.Equal(t, "done", *updatedDeploy.Step)
}
