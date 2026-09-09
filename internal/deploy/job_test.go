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
