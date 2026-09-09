package models

import (
	stdtesting "testing"

	"github.com/stretchr/testify/assert"
)

func TestAppStatusConstants(t *stdtesting.T) {
	assert.Equal(t, AppStatus("idle"), AppStatusIdle)
	assert.Equal(t, AppStatus("building"), AppStatusBuilding)
	assert.Equal(t, AppStatus("running"), AppStatusRunning)
	assert.Equal(t, AppStatus("stopped"), AppStatusStopped)
	assert.Equal(t, AppStatus("failed"), AppStatusFailed)
}

func TestSourceTypeConstants(t *stdtesting.T) {
	assert.Equal(t, SourceType("git"), SourceTypeGit)
	assert.Equal(t, SourceType("docker_image"), SourceTypeDockerImage)
	assert.Equal(t, SourceType("dockerfile"), SourceTypeDockerfile)
}

func TestDeployStatusConstants(t *stdtesting.T) {
	assert.Equal(t, DeployStatus("queued"), DeployStatusQueued)
	assert.Equal(t, DeployStatus("cloning"), DeployStatusCloning)
	assert.Equal(t, DeployStatus("building"), DeployStatusBuilding)
	assert.Equal(t, DeployStatus("deploying"), DeployStatusDeploying)
	assert.Equal(t, DeployStatus("success"), DeployStatusSuccess)
	assert.Equal(t, DeployStatus("failed"), DeployStatusFailed)
	assert.Equal(t, DeployStatus("cancelled"), DeployStatusCancelled)
}

func TestApplicationStruct(t *stdtesting.T) {
	app := &Application{
		ID:            "app-1",
		EnvironmentID: "env-1",
		Name:          "Test App",
		Slug:          "test-app",
		SourceType:    SourceTypeGit,
		AppPort:       8080,
		Status:        AppStatusIdle,
	}

	assert.Equal(t, "app-1", app.ID)
	assert.Equal(t, "env-1", app.EnvironmentID)
	assert.Equal(t, "Test App", app.Name)
	assert.Equal(t, "test-app", app.Slug)
	assert.Equal(t, SourceTypeGit, app.SourceType)
	assert.Equal(t, 8080, app.AppPort)
	assert.Equal(t, AppStatusIdle, app.Status)
}

func TestCreateApplicationInput(t *stdtesting.T) {
	input := CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "New App",
		Slug:          "new-app",
		SourceType:    SourceTypeDockerfile,
		AppPort:       3000,
	}

	assert.Equal(t, "env-1", input.EnvironmentID)
	assert.Equal(t, "New App", input.Name)
	assert.Equal(t, "new-app", input.Slug)
	assert.Equal(t, SourceTypeDockerfile, input.SourceType)
	assert.Equal(t, 3000, input.AppPort)
}
