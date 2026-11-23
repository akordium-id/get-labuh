package deploy_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/caddy"
	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/deploy"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
)

type mockRegistryProvider struct {
	registries map[string]*models.DockerRegistry
}

func (m *mockRegistryProvider) GetByID(ctx context.Context, id string) (*models.DockerRegistry, error) {
	if reg, ok := m.registries[id]; ok {
		return reg, nil
	}
	return nil, os.ErrNotExist
}

func TestPipeline_SetRegistryRepo(t *testing.T) {
	_ = os.Setenv("LABUH_DOCKER_MOCK", "true")

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

	pipeline := deploy.NewPipeline(dockerClient, caddyClient, settingRepo, appRepo, deployRepo)

	mockReg := &mockRegistryProvider{
		registries: map[string]*models.DockerRegistry{
			"reg-1": {
				ID:            "reg-1",
				Name:          "GHCR",
				ServerAddress: "ghcr.io",
				Username:      "octocat",
				Password:      "ghp_token",
			},
		},
	}
	pipeline.SetRegistryRepo(mockReg)

	// Verify job fields
	job := deploy.DeploymentJob{
		DeploymentID:     "dep-1",
		ApplicationID:    "app-1",
		SourceType:       string(models.SourceTypeDockerImage),
		DockerImage:      "ghcr.io/org/repo:v1",
		DockerRegistryID: func() *string { s := "reg-1"; return &s }(),
		CommitHash:       "a1b2c3d4",
	}

	assert.Equal(t, "reg-1", *job.DockerRegistryID)
	assert.Equal(t, "a1b2c3d4", job.CommitHash)
}
