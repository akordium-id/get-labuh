package docker

import (
	stdtesting "testing"
	"errors"
	"os"

	"github.com/stretchr/testify/assert"
)

func init() {
	_ = os.Setenv("LABUH_DOCKER_MOCK", "true")
}

func TestClient_Ping(t *stdtesting.T) {
	client, err := NewClient()
	assert.NoError(t, err)

	err = client.Ping(nil)
	assert.NoError(t, err)
}

func TestClient_PullImage(t *stdtesting.T) {
	client, err := NewClient()
	assert.NoError(t, err)

	err = client.PullImage(nil, "nginx:latest")
	assert.NoError(t, err)
}

func TestClient_BuildImage(t *stdtesting.T) {
	client, err := NewClient()
	assert.NoError(t, err)

	_, err = client.BuildImage(nil, nil, "Dockerfile", []string{"test:latest"})
	assert.NoError(t, err)
}

func TestClient_CreateContainer(t *stdtesting.T) {
	client, err := NewClient()
	assert.NoError(t, err)

	id, err := client.CreateContainer(nil, nil, nil, nil, "test-container")
	assert.NoError(t, err)
	assert.Empty(t, id)
}

func TestClient_StartContainer(t *stdtesting.T) {
	client, err := NewClient()
	assert.NoError(t, err)

	err = client.StartContainer(nil, "container-123")
	assert.NoError(t, err)
}

func TestClient_RemoveContainer(t *stdtesting.T) {
	client, err := NewClient()
	assert.NoError(t, err)

	err = client.RemoveContainer(nil, "container-123")
	assert.NoError(t, err)
}

func TestClient_InspectContainer(t *stdtesting.T) {
	client, err := NewClient()
	assert.NoError(t, err)

	err = client.InspectContainer(nil, "container-123")
	assert.NoError(t, err)
}

func TestClient_ContainerLogs(t *stdtesting.T) {
	client, err := NewClient()
	assert.NoError(t, err)

	reader, err := client.ContainerLogs(nil, "container-123", nil)
	assert.NoError(t, err)
	assert.Nil(t, reader)
}

func TestClient_ListContainers(t *stdtesting.T) {
	client, err := NewClient()
	assert.NoError(t, err)

	err = client.ListContainers(nil, nil)
	assert.NoError(t, err)
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
