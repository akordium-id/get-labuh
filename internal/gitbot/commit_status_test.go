package gitbot_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/gitbot"
)

func TestGitHubCommitStatus(t *testing.T) {
	var receivedPath string
	var receivedAuth string
	var receivedBody map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := gitbot.NewClientWithBaseURLs(server.URL, server.URL)
	ctx := context.Background()

	err := client.SetGitHubCommitStatus(
		ctx,
		"https://github.com/akordium-id/get-labuh.git",
		"a1b2c3d4e5f6",
		"ghp_testtoken",
		"success",
		"https://labuh.app/deployments/dep-1",
		"Deployment live and healthy!",
		"labuh/deploy",
	)
	require.NoError(t, err)

	assert.Equal(t, "/repos/akordium-id/get-labuh/statuses/a1b2c3d4e5f6", receivedPath)
	assert.Equal(t, "Bearer ghp_testtoken", receivedAuth)
	assert.Equal(t, "success", receivedBody["state"])
	assert.Equal(t, "https://labuh.app/deployments/dep-1", receivedBody["target_url"])
	assert.Equal(t, "Deployment live and healthy!", receivedBody["description"])
	assert.Equal(t, "labuh/deploy", receivedBody["context"])
}

func TestGitLabCommitStatus(t *testing.T) {
	var receivedPath string
	var receivedToken string
	var receivedBody map[string]string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		receivedToken = r.Header.Get("PRIVATE-TOKEN")
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := gitbot.NewClientWithBaseURLs(server.URL, server.URL)
	ctx := context.Background()

	err := client.SetGitLabCommitStatus(
		ctx,
		"https://gitlab.com/akordium-id/subgroup/get-labuh.git",
		"a1b2c3d4e5f6",
		"glpat_testtoken",
		"success",
		"https://labuh.app/deployments/dep-1",
		"Deployment live and healthy!",
		"labuh/deploy",
	)
	require.NoError(t, err)

	assert.Contains(t, receivedPath, "/api/v4/projects/")
	assert.Contains(t, receivedPath, "/statuses/a1b2c3d4e5f6")
	assert.Equal(t, "glpat_testtoken", receivedToken)
	assert.Equal(t, "success", receivedBody["state"])
	assert.Equal(t, "https://labuh.app/deployments/dep-1", receivedBody["target_url"])
	assert.Equal(t, "Deployment live and healthy!", receivedBody["description"])
	assert.Equal(t, "labuh/deploy", receivedBody["name"])
}
