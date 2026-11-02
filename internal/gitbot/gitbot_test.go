package gitbot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRepo(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		wantProvider string
		wantOwner    string
		wantRepo     string
		wantErr      bool
	}{
		{
			name:         "GitHub HTTPS with .git",
			url:          "https://github.com/akordium-id/get-labuh.git",
			wantProvider: "github",
			wantOwner:    "akordium-id",
			wantRepo:     "get-labuh",
		},
		{
			name:         "GitHub SSH",
			url:          "git@github.com:akordium-id/get-labuh.git",
			wantProvider: "github",
			wantOwner:    "akordium-id",
			wantRepo:     "get-labuh",
		},
		{
			name:         "GitLab HTTPS nested",
			url:          "https://gitlab.com/group/subgroup/project.git",
			wantProvider: "gitlab",
			wantOwner:    "group/subgroup",
			wantRepo:     "project",
		},
		{
			name:         "GitLab SSH",
			url:          "git@gitlab.com:group/project.git",
			wantProvider: "gitlab",
			wantOwner:    "group",
			wantRepo:     "project",
		},
		{
			name:    "Invalid URL",
			url:     "invalid-url",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, ownerOrPath, repo, err := ParseRepo(tt.url)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantProvider, provider)
			assert.Equal(t, tt.wantOwner, ownerOrPath)
			assert.Equal(t, tt.wantRepo, repo)
		})
	}
}

func TestGitHub_CreateAndPatchComment(t *testing.T) {
	var comments []map[string]any
	var patchCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer ghp_test123", r.Header.Get("Authorization"))

		// 1. GET /repos/owner/repo/issues/42/comments
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/comments") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(comments)
			return
		}

		// 2. POST /repos/owner/repo/issues/42/comments
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/comments") {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			comment := map[string]any{
				"id":   101,
				"body": body["body"],
			}
			comments = append(comments, comment)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(comment)
			return
		}

		// 3. PATCH /repos/owner/repo/issues/comments/101
		if r.Method == http.MethodPatch && strings.HasSuffix(r.URL.Path, "/comments/101") {
			patchCalled = true
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			comments[0]["body"] = body["body"]
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(comments[0])
			return
		}

		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClientWithBaseURLs(server.URL, server.URL)

	// Step A: First call should CREATE (POST) comment with marker
	body1 := FormatComment(CommentParams{
		Status:     StatusBuilding,
		CommitHash: "abc1234",
		Branch:     "feature/pr-bot",
	})
	err := client.PostOrUpdateGitHubComment(context.Background(), "https://github.com/owner/repo", 42, "ghp_test123", body1)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Contains(t, comments[0]["body"].(string), Marker)
	assert.Contains(t, comments[0]["body"].(string), "Building & Deploying...")

	// Step B: Second call should UPDATE (PATCH) the existing comment
	body2 := FormatComment(CommentParams{
		Status:     StatusLive,
		PreviewURL: "https://pr-42.preview.test",
		CommitHash: "abc1234",
	})
	err = client.PostOrUpdateGitHubComment(context.Background(), "https://github.com/owner/repo", 42, "ghp_test123", body2)
	require.NoError(t, err)
	assert.True(t, patchCalled)
	require.Len(t, comments, 1) // Masih tetap 1 komentar (anti-spam)
	assert.Contains(t, comments[0]["body"].(string), "https://pr-42.preview.test")
	assert.Contains(t, comments[0]["body"].(string), "Live & Healthy")
}

func TestGitLab_CreateAndPatchNote(t *testing.T) {
	var notes []map[string]any
	var putCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "glpat_test123", r.Header.Get("PRIVATE-TOKEN"))

		// 1. GET /api/v4/projects/.../merge_requests/12/notes
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/notes") {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(notes)
			return
		}

		// 2. POST /api/v4/projects/.../merge_requests/12/notes
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/notes") {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			note := map[string]any{
				"id":   505,
				"body": body["body"],
			}
			notes = append(notes, note)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(note)
			return
		}

		// 3. PUT /api/v4/projects/.../merge_requests/12/notes/505
		if r.Method == http.MethodPut && strings.HasSuffix(r.URL.Path, "/notes/505") {
			putCalled = true
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			notes[0]["body"] = body["body"]
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(notes[0])
			return
		}

		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer server.Close()

	client := NewClientWithBaseURLs(server.URL, server.URL)

	// Step A: First call should CREATE (POST) note
	body1 := FormatComment(CommentParams{
		Status:     StatusBuilding,
		CommitHash: "gitlab123",
		Branch:     "feature/gitlab-pr",
	})
	err := client.PostOrUpdateGitLabNote(context.Background(), "https://gitlab.com/group/repo", 12, "glpat_test123", body1)
	require.NoError(t, err)
	require.Len(t, notes, 1)

	// Step B: Second call should UPDATE (PUT) existing note
	body2 := FormatComment(CommentParams{
		Status:     StatusLive,
		PreviewURL: "https://gl-12.preview.test",
	})
	err = client.PostOrUpdateGitLabNote(context.Background(), "https://gitlab.com/group/repo", 12, "glpat_test123", body2)
	require.NoError(t, err)
	assert.True(t, putCalled)
	require.Len(t, notes, 1)
	assert.Contains(t, notes[0]["body"].(string), "https://gl-12.preview.test")
}
