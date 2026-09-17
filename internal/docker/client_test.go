package docker

import (
	"context"
	"errors"
	stdtesting "testing"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
)

func TestClient_Ping(t *stdtesting.T) {
	client, err := NewClient()
	assert.NoError(t, err)

	_ = client.Ping(context.Background())
}

func TestClient_InspectContainer(t *stdtesting.T) {
	client, err := NewClient()
	assert.NoError(t, err)

	_, _ = client.InspectContainer(context.Background(), "non-existent-container-123")
}

func TestClient_ContainerLogs(t *stdtesting.T) {
	client, err := NewClient()
	assert.NoError(t, err)

	_, _ = client.ContainerLogs(context.Background(), "container-123", container.LogsOptions{})
}

func TestClient_ListContainers(t *stdtesting.T) {
	client, err := NewClient()
	assert.NoError(t, err)

	_, _ = client.ListContainers(context.Background(), container.ListOptions{})
}

func TestValidateRepositoryURL(t *stdtesting.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"empty", "", true},
		{"https", "https://github.com/test/repo", false},
		{"http", "http://github.com/test/repo", false},
		{"git ssh", "git@github.com:test/repo.git", false},
		{"git protocol", "git://github.com/test/repo.git", false},
		{"invalid", "ftp://github.com/test/repo", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *stdtesting.T) {
			err := ValidateRepositoryURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSanitizeBranch(t *stdtesting.T) {
	assert.Equal(t, "main", SanitizeBranch(""))
	assert.Equal(t, "main", SanitizeBranch("   "))
	assert.Equal(t, "feature/test", SanitizeBranch("feature/test"))
	assert.Equal(t, "main", SanitizeBranch(" main "))
}

func TestGenerateContainerName(t *stdtesting.T) {
	client, _ := NewClient()
	name := client.GenerateContainerName("prod", "myapp", "12345678-1234-1234-1234-123456789abc")
	assert.Contains(t, name, "labuh-prod-myapp-")
}

func TestContainerStatusConstants(t *stdtesting.T) {
	assert.Equal(t, ContainerStatus("running"), ContainerStatusRunning)
	assert.Equal(t, ContainerStatus("stopped"), ContainerStatusStopped)
	assert.Equal(t, ContainerStatus("building"), ContainerStatusBuilding)
	assert.Equal(t, ContainerStatus("failed"), ContainerStatusFailed)
}

func TestContainerNotFoundError(t *stdtesting.T) {
	err := &ContainerNotFoundError{ContainerID: "abc123"}
	assert.Equal(t, "container not found: abc123", err.Error())
}

func TestDockerUnavailableError(t *stdtesting.T) {
	err := &DockerUnavailableError{Cause: errors.New("connection refused")}
	assert.Contains(t, err.Error(), "docker daemon unavailable")
}
