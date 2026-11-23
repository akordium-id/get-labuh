package models

import (
	"encoding/json"
	stdtesting "testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerRegistryModel(t *stdtesting.T) {
	now := time.Now()
	reg := &DockerRegistry{
		ID:            "reg-123",
		Name:          "GitHub Packages",
		ServerAddress: "ghcr.io",
		Username:      "octocat",
		Password:      "super-secret-token",
		Email:         "octo@github.com",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	assert.Equal(t, "reg-123", reg.ID)
	assert.Equal(t, "GitHub Packages", reg.Name)
	assert.Equal(t, "ghcr.io", reg.ServerAddress)
	assert.Equal(t, "octocat", reg.Username)
	assert.Equal(t, "super-secret-token", reg.Password)
	assert.Equal(t, "octo@github.com", reg.Email)

	// Password must NOT be serialized in JSON
	bytes, err := json.Marshal(reg)
	require.NoError(t, err)
	assert.NotContains(t, string(bytes), "super-secret-token")
	assert.Contains(t, string(bytes), "ghcr.io")

	// Inputs test
	createInput := CreateDockerRegistryInput{
		Name:          "Docker Hub Private",
		ServerAddress: "docker.io",
		Username:      "devuser",
		Password:      "secretpass",
		Email:         "dev@example.com",
	}
	assert.Equal(t, "Docker Hub Private", createInput.Name)

	newPass := "updatedpass"
	updateInput := UpdateDockerRegistryInput{
		ID:            "reg-123",
		Name:          "Updated Registry",
		ServerAddress: "index.docker.io/v1/",
		Username:      "devuser2",
		Password:      &newPass,
		Email:         "dev2@example.com",
	}
	assert.Equal(t, "reg-123", updateInput.ID)
	assert.Equal(t, "updatedpass", *updateInput.Password)

	// Application DockerRegistryID field test
	regID := "reg-123"
	app := &Application{
		ID:               "app-1",
		DockerRegistryID: &regID,
	}
	assert.Equal(t, "reg-123", *app.DockerRegistryID)
}
