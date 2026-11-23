package compose_test

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/compose"
	"github.com/akordium-id/get-labuh/internal/models"
)

type mockDockerClient struct {
	containers []types.Container
	logOutput  string
}

func (m *mockDockerClient) ListContainers(ctx context.Context, opts container.ListOptions) ([]types.Container, error) {
	return m.containers, nil
}

func (m *mockDockerClient) ContainerLogs(ctx context.Context, containerID string, opts container.LogsOptions) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(m.logOutput)), nil
}

func TestOrchestrator_PrepareWorkDir(t *testing.T) {
	tempBase := filepath.Join(os.TempDir(), "test-compose-base")
	defer os.RemoveAll(tempBase)

	mockCli := &mockDockerClient{}
	orch := compose.NewOrchestrator(tempBase, mockCli)

	rawYAML := `
services:
  app:
    image: node:18-alpine
    ports:
      - "3000:3000"
    environment:
      - PORT=${PORT}
      - SECRET_KEY=${SECRET_KEY}
`
	app := &models.ComposeApplication{
		ID:      "comp-test-1",
		Slug:    "test-stack",
		RawYAML: rawYAML,
	}

	envVars := []models.AppEnvVar{
		{Key: "PORT", Value: "3000"},
		{Key: "SECRET_KEY", Value: "my-secure-val-with-\"quotes\""},
	}

	workDir, err := orch.PrepareWorkDir(app, envVars)
	require.NoError(t, err)
	assert.DirExists(t, workDir)

	composePath := filepath.Join(workDir, "docker-compose.yml")
	assert.FileExists(t, composePath)
	content, err := os.ReadFile(composePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "node:18-alpine")

	envPath := filepath.Join(workDir, ".env")
	assert.FileExists(t, envPath)
	envContent, err := os.ReadFile(envPath)
	require.NoError(t, err)
	assert.Contains(t, string(envContent), "PORT=\"3000\"")
	assert.Contains(t, string(envContent), "SECRET_KEY=\"my-secure-val-with-\\\"quotes\\\"\"")
}

func TestOrchestrator_InspectServices(t *testing.T) {
	mockCli := &mockDockerClient{
		containers: []types.Container{
			{
				ID:      "cid-web-123",
				State:   "running",
				Status:  "Up 10 minutes (healthy)",
				Created: time.Now().Unix(),
				Labels: map[string]string{
					"com.docker.compose.project": "my-project",
					"com.docker.compose.service": "web",
				},
			},
		},
		logOutput: "Server started on :80\n",
	}

	orch := compose.NewOrchestrator("", mockCli)

	projName := "my-project"
	app := &models.ComposeApplication{
		ID:                 "comp-inspect-1",
		Slug:               "my-project",
		ComposeProjectName: &projName,
		ParsedServices: []models.ComposeServiceDef{
			{Name: "web", Image: "nginx:alpine"},
			{Name: "worker", Image: "worker:latest"},
		},
	}

	ctx := context.Background()
	runtimes, err := orch.InspectServices(ctx, app)
	require.NoError(t, err)
	require.Len(t, runtimes, 2)

	// Service "web" has container running & healthy
	assert.Equal(t, "web", runtimes[0].Name)
	assert.Equal(t, "cid-web-123", runtimes[0].ContainerID)
	assert.Equal(t, "running", runtimes[0].Status)
	assert.Equal(t, "healthy", runtimes[0].Health)

	// Service "worker" is not created
	assert.Equal(t, "worker", runtimes[1].Name)
	assert.Empty(t, runtimes[1].ContainerID)
	assert.Equal(t, "not_created", runtimes[1].Status)

	// Test GetServiceLogs
	logs, err := orch.GetServiceLogs(ctx, app, "web", 50)
	require.NoError(t, err)
	assert.Contains(t, logs, "Server started on :80")

	// GetServiceLogs for not created service should fail
	_, err = orch.GetServiceLogs(ctx, app, "worker", 50)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "belum dibuat")
}
