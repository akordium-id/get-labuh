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

func TestAppVolumeModel(t *stdtesting.T) {
	vol := AppVolume{
		ID:            "vol-1",
		ApplicationID: "app-1",
		Name:          "uploads",
		VolumeType:    VolumeTypeNamed,
		MountPath:     "/app/storage",
		IsReadOnly:    false,
	}
	assert.Equal(t, VolumeTypeNamed, vol.VolumeType)
	assert.Equal(t, "/app/storage", vol.MountPath)

	fileMount := AppFileMount{
		ID:            "fm-1",
		ApplicationID: "app-1",
		MountPath:     "/etc/nginx/nginx.conf",
		Content:       "events {}",
		FileMode:      "0644",
		IsReadOnly:    true,
	}
	assert.Equal(t, "/etc/nginx/nginx.conf", fileMount.MountPath)
	assert.True(t, fileMount.IsReadOnly)
}

func TestAppBackupModels(t *stdtesting.T) {
	cfg := AppBackupConfig{
		ID:             "cfg-1",
		ApplicationID:  "app-1",
		Enabled:        true,
		CronExpression: "0 3 * * *",
		RetentionCount: 7,
	}
	assert.Equal(t, "cfg-1", cfg.ID)
	assert.Equal(t, "app-1", cfg.ApplicationID)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, "0 3 * * *", cfg.CronExpression)
	assert.Equal(t, 7, cfg.RetentionCount)

	backup := AppBackup{
		ID:            "backup-1",
		ApplicationID: "app-1",
		Filename:      "app-volume-test-20261123.tar.gz",
		FileSize:      1024,
		StorageType:   "s3",
		Status:        "completed",
	}
	assert.Equal(t, "backup-1", backup.ID)
	assert.Equal(t, "app-volume-test-20261123.tar.gz", backup.Filename)
	assert.Equal(t, int64(1024), backup.FileSize)
	assert.Equal(t, "s3", backup.StorageType)
	assert.Equal(t, "completed", backup.Status)
}

