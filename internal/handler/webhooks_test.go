package handler

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	stdtesting "testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/worker"
)

func setupWebhooksTest(t *stdtesting.T) (*WebhooksHandler, *repo.ApplicationRepo, *repo.DeploymentRepo, *models.Application) {
	t.Helper()
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	t.Cleanup(func() { _ = db.Close() })

	projectRepo := repo.NewProjectRepo(db)
	appRepo := repo.NewApplicationRepo(db)
	deployRepo := repo.NewDeploymentRepo(db)

	project, err := projectRepo.Create(models.CreateProjectInput{Name: "Webhook App Project", Slug: "webhook-project"})
	require.NoError(t, err)
	env, err := projectRepo.CreateEnvironment(project.ID, "Production", "prod")
	require.NoError(t, err)

	secret := "super-secure-webhook-secret-123"
	branch := "main"
	app, err := appRepo.Create(models.CreateApplicationInput{
		EnvironmentID: env.ID,
		Name:          "Web Service",
		Slug:          "web-service",
		SourceType:    models.SourceTypeGit,
		Branch:        &branch,
		WebhookSecret: &secret,
	})
	require.NoError(t, err)

	h := NewWebhooksHandler(appRepo, deployRepo)
	deployWorker := worker.NewDeployWorker(10)
	h.SetDeployWorker(deployWorker)

	return h, appRepo, deployRepo, app
}

func calculateHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhooksHandler_GitHub_Ping(t *stdtesting.T) {
	h, _, _, app := setupWebhooksTest(t)

	r := chi.NewRouter()
	r.Post("/webhooks/github/{application_id}", h.GitHub)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/github/"+app.ID, bytes.NewBufferString(`{}`))
	req.Header.Set("X-GitHub-Event", "ping")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "pong", resp["status"])
}

func TestWebhooksHandler_GitHub_Push_Success(t *stdtesting.T) {
	h, _, deployRepo, app := setupWebhooksTest(t)

	r := chi.NewRouter()
	r.Post("/webhooks/github/{application_id}", h.GitHub)

	payload := `{
		"ref": "refs/heads/main",
		"after": "c0ffee1234567890",
		"head_commit": {
			"id": "c0ffee1234567890",
			"message": "feat: auto-deploy working seamlessly",
			"author": { "name": "Faiq", "email": "faiq@example.com" }
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/webhooks/github/"+app.ID, bytes.NewBufferString(payload))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", calculateHMAC([]byte(payload), *app.WebhookSecret))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "queued", resp["status"])
	assert.Equal(t, "c0ffee1234567890", resp["commit_hash"])

	// Verifikasi deployment tercatat di DB
	deployID, ok := resp["deployment_id"].(string)
	require.True(t, ok)
	dep, err := deployRepo.GetByID(deployID)
	require.NoError(t, err)
	assert.Equal(t, "c0ffee1234567890", *dep.CommitHash)
	assert.Equal(t, "feat: auto-deploy working seamlessly", *dep.CommitMessage)
}

func TestWebhooksHandler_GitHub_InvalidSignature(t *stdtesting.T) {
	h, _, _, app := setupWebhooksTest(t)

	r := chi.NewRouter()
	r.Post("/webhooks/github/{application_id}", h.GitHub)

	payload := `{"ref": "refs/heads/main"}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github/"+app.ID, bytes.NewBufferString(payload))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid-signature")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestWebhooksHandler_GitHub_BranchMismatch_Ignored(t *stdtesting.T) {
	h, _, deployRepo, app := setupWebhooksTest(t)

	r := chi.NewRouter()
	r.Post("/webhooks/github/{application_id}", h.GitHub)

	payload := `{
		"ref": "refs/heads/feature/awesome-ui",
		"after": "dev123456"
	}`

	req := httptest.NewRequest(http.MethodPost, "/webhooks/github/"+app.ID, bytes.NewBufferString(payload))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("X-Hub-Signature-256", calculateHMAC([]byte(payload), *app.WebhookSecret))
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Status harus 200 OK tetapi diabaikan (ignored)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "ignored", resp["status"])
	assert.Equal(t, "branch mismatch", resp["reason"])

	// Tidak ada deployment yang dibuat
	deployments, err := deployRepo.GetByApplicationID(app.ID)
	require.NoError(t, err)
	assert.Empty(t, deployments)
}

func TestWebhooksHandler_GitLab_Push_Success(t *stdtesting.T) {
	h, _, deployRepo, app := setupWebhooksTest(t)

	r := chi.NewRouter()
	r.Post("/webhooks/gitlab/{application_id}", h.GitLab)

	payload := `{
		"object_kind": "push",
		"ref": "refs/heads/main",
		"checkout_sha": "gitlabsha123456",
		"commits": [
			{
				"id": "gitlabsha123456",
				"message": "fix: resolve production crash",
				"author": { "name": "GitLab CI", "email": "ci@gitlab.com" }
			}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/webhooks/gitlab/"+app.ID, bytes.NewBufferString(payload))
	req.Header.Set("X-Gitlab-Event", "Push Hook")
	req.Header.Set("X-Gitlab-Token", *app.WebhookSecret)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "queued", resp["status"])
	assert.Equal(t, "gitlabsha123456", resp["commit_hash"])

	deployID, ok := resp["deployment_id"].(string)
	require.True(t, ok)
	dep, err := deployRepo.GetByID(deployID)
	require.NoError(t, err)
	assert.Equal(t, "gitlabsha123456", *dep.CommitHash)
	assert.Equal(t, "fix: resolve production crash", *dep.CommitMessage)
}

func TestWebhooksHandler_GitLab_InvalidToken(t *stdtesting.T) {
	h, _, _, app := setupWebhooksTest(t)

	r := chi.NewRouter()
	r.Post("/webhooks/gitlab/{application_id}", h.GitLab)

	payload := `{"object_kind": "push", "ref": "refs/heads/main"}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/gitlab/"+app.ID, bytes.NewBufferString(payload))
	req.Header.Set("X-Gitlab-Event", "Push Hook")
	req.Header.Set("X-Gitlab-Token", "wrong-token")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestWebhooksHandler_Deploy_AutoDetect(t *stdtesting.T) {
	h, _, _, app := setupWebhooksTest(t)

	r := chi.NewRouter()
	r.Post("/webhooks/deploy/{application_id}", h.Deploy)

	// Uji generic cURL deployment dengan token di Authorization Header
	payload := `{"branch": "main", "commit_hash": "curlsha123", "commit_message": "manual deploy via curl"}`
	req := httptest.NewRequest(http.MethodPost, "/webhooks/deploy/"+app.ID, bytes.NewBufferString(payload))
	req.Header.Set("Authorization", "Bearer "+*app.WebhookSecret)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "queued", resp["status"])
	assert.Equal(t, "curlsha123", resp["commit_hash"])
}
