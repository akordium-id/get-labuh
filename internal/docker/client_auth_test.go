package docker_test

import (
	"encoding/base64"
	"encoding/json"
	stdtesting "testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/docker"
)

func TestRegistryAuthEncoding(t *stdtesting.T) {
	auth := &docker.RegistryAuth{
		Username:      "octocat",
		Password:      "ghp_secrettoken",
		ServerAddress: "ghcr.io",
		Email:         "octo@github.com",
	}

	encoded, err := auth.EncodeBase64()
	require.NoError(t, err)
	require.NotEmpty(t, encoded)

	// Decode base64 URL
	decodedBytes, err := base64.URLEncoding.DecodeString(encoded)
	require.NoError(t, err)

	var parsed map[string]string
	err = json.Unmarshal(decodedBytes, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "octocat", parsed["username"])
	assert.Equal(t, "ghp_secrettoken", parsed["password"])
	assert.Equal(t, "ghcr.io", parsed["serveraddress"])
	assert.Equal(t, "octo@github.com", parsed["email"])
}

func TestNilRegistryAuthEncoding(t *stdtesting.T) {
	var auth *docker.RegistryAuth
	encoded, err := auth.EncodeBase64()
	require.NoError(t, err)
	assert.Empty(t, encoded)
}
