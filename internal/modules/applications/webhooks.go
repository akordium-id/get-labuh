package applications

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/caddy"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/encryption"
	"github.com/akordium-id/get-labuh/internal/gitbot"
	"github.com/akordium-id/get-labuh/internal/models"
)

// WebhooksHandler menangani request webhook dari GitHub, GitLab, atau Git generic.
type WebhooksHandler struct {
	appRepo      *Repo
	deployRepo   DeploymentCreator
	envVarRepo   *EnvVarRepo
	deployWorker DeployEnqueuer
	dockerClient *docker.Client
	caddyClient  *caddy.Client
	settingRepo  SettingProvider
	gitbotClient *gitbot.Client
}

// NewWebhooksHandler membuat instance baru WebhooksHandler.
func NewWebhooksHandler(appRepo *Repo, deployRepo DeploymentCreator, envVarRepo *EnvVarRepo) *WebhooksHandler {
	return &WebhooksHandler{
		appRepo:      appRepo,
		deployRepo:   deployRepo,
		envVarRepo:   envVarRepo,
		gitbotClient: gitbot.NewClient(),
	}
}

// SetSettingRepo menyetel setting provider untuk membaca global bot tokens.
func (h *WebhooksHandler) SetSettingRepo(s SettingProvider) {
	h.settingRepo = s
}

// SetGitBotClient menyetel gitbot client (berguna untuk testing/mock).
func (h *WebhooksHandler) SetGitBotClient(c *gitbot.Client) {
	h.gitbotClient = c
}

// SetDeployWorker menyetel worker antrean deployment.
func (h *WebhooksHandler) SetDeployWorker(w DeployEnqueuer) {
	h.deployWorker = w
}

// SetClients menyetel docker dan caddy client untuk kebutuhan teardown ephemeral preview.
func (h *WebhooksHandler) SetClients(dockerClient *docker.Client, caddyClient *caddy.Client) {
	h.dockerClient = dockerClient
	h.caddyClient = caddyClient
}

type githubWebhookPayload struct {
	Action      string `json:"action"`
	Number      int    `json:"number"`
	PullRequest *struct {
		Title string `json:"title"`
		Head  struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Merged bool `json:"merged"`
	} `json:"pull_request"`
	Ref        string `json:"ref"`
	Before     string `json:"before"`
	After      string `json:"after"`
	HeadCommit *struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"head_commit"`
	Commits []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
	} `json:"commits"`
}

type gitlabWebhookPayload struct {
	ObjectKind       string `json:"object_kind"`
	Ref              string `json:"ref"`
	CheckoutSHA      string `json:"checkout_sha"`
	ObjectAttributes *struct {
		IID          int    `json:"iid"`
		Title        string `json:"title"`
		SourceBranch string `json:"source_branch"`
		LastCommit   struct {
			ID      string `json:"id"`
			Message string `json:"message"`
		} `json:"last_commit"`
		Action string `json:"action"`
		State  string `json:"state"`
	} `json:"object_attributes"`
	Commits []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"commits"`
}

type genericWebhookPayload struct {
	Branch        string `json:"branch"`
	CommitHash    string `json:"commit_hash"`
	CommitMessage string `json:"commit_message"`
}

// Deploy menangani webhook universal dengan auto deteksi provider.
func (h *WebhooksHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "application_id")
	if appID == "" {
		http.Error(w, `{"error":"missing application_id"}`, http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByIDWithContext(r.Context(), appID)
	if err != nil {
		http.Error(w, `{"error":"application not found"}`, http.StatusNotFound)
		return
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		http.Error(w, `{"error":"cannot read request body"}`, http.StatusBadRequest)
		return
	}

	ghEvent := r.Header.Get("X-GitHub-Event")
	glEvent := r.Header.Get("X-Gitlab-Event")

	if ghEvent != "" {
		h.handleGitHub(w, r, app, ghEvent, bodyBytes)
		return
	}

	if glEvent != "" {
		h.handleGitLab(w, r, app, glEvent, bodyBytes)
		return
	}

	h.handleGeneric(w, r, app, bodyBytes)
}

// GitHub menangani webhook spesifik endpoint GitHub.
func (h *WebhooksHandler) GitHub(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "application_id")
	if appID == "" {
		http.Error(w, `{"error":"missing application_id"}`, http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByIDWithContext(r.Context(), appID)
	if err != nil {
		http.Error(w, `{"error":"application not found"}`, http.StatusNotFound)
		return
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		http.Error(w, `{"error":"cannot read request body"}`, http.StatusBadRequest)
		return
	}

	ghEvent := r.Header.Get("X-GitHub-Event")
	if ghEvent == "" {
		ghEvent = "push"
	}

	h.handleGitHub(w, r, app, ghEvent, bodyBytes)
}

// GitLab menangani webhook spesifik endpoint GitLab.
func (h *WebhooksHandler) GitLab(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "application_id")
	if appID == "" {
		http.Error(w, `{"error":"missing application_id"}`, http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByIDWithContext(r.Context(), appID)
	if err != nil {
		http.Error(w, `{"error":"application not found"}`, http.StatusNotFound)
		return
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		http.Error(w, `{"error":"cannot read request body"}`, http.StatusBadRequest)
		return
	}

	glEvent := r.Header.Get("X-Gitlab-Event")
	if glEvent == "" {
		glEvent = "Push Hook"
	}

	h.handleGitLab(w, r, app, glEvent, bodyBytes)
}

func (h *WebhooksHandler) handleGitHub(w http.ResponseWriter, r *http.Request, app *models.Application, event string, bodyBytes []byte) {
	if event == "ping" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "pong",
			"message": "GitHub webhook configured successfully on Labuh",
		})
		return
	}

	if app.WebhookSecret != nil && *app.WebhookSecret != "" {
		signature := r.Header.Get("X-Hub-Signature-256")
		if signature == "" {
			http.Error(w, `{"error":"unauthorized: missing X-Hub-Signature-256"}`, http.StatusUnauthorized)
			return
		}

		mac := hmac.New(sha256.New, []byte(*app.WebhookSecret))
		mac.Write(bodyBytes)
		expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			http.Error(w, `{"error":"unauthorized: invalid signature"}`, http.StatusUnauthorized)
			return
		}
	}

	var payload githubWebhookPayload
	if len(bodyBytes) > 0 {
		_ = json.Unmarshal(bodyBytes, &payload)
	}

	if event == "pull_request" {
		h.handleGitHubPullRequest(w, r, app, payload)
		return
	}

	targetBranch := "main"
	if app.Branch != nil && *app.Branch != "" {
		targetBranch = *app.Branch
	}

	if payload.Ref != "" {
		pushedBranch := strings.TrimPrefix(payload.Ref, "refs/heads/")
		if pushedBranch != targetBranch {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":        "ignored",
				"reason":        "branch mismatch",
				"pushed_branch": pushedBranch,
				"target_branch": targetBranch,
			})
			return
		}
	}

	commitHash := payload.After
	var commitMsg string
	if payload.HeadCommit != nil {
		if commitHash == "" {
			commitHash = payload.HeadCommit.ID
		}
		commitMsg = payload.HeadCommit.Message
	} else if len(payload.Commits) > 0 {
		if commitHash == "" {
			commitHash = payload.Commits[len(payload.Commits)-1].ID
		}
		commitMsg = payload.Commits[len(payload.Commits)-1].Message
	}

	h.dispatchDeployment(w, app, targetBranch, commitHash, commitMsg)
}

func (h *WebhooksHandler) handleGitLab(w http.ResponseWriter, r *http.Request, app *models.Application, event string, bodyBytes []byte) {
	if app.WebhookSecret != nil && *app.WebhookSecret != "" {
		token := r.Header.Get("X-Gitlab-Token")
		if subtle.ConstantTimeCompare([]byte(token), []byte(*app.WebhookSecret)) != 1 {
			http.Error(w, `{"error":"unauthorized: invalid X-Gitlab-Token"}`, http.StatusUnauthorized)
			return
		}
	}

	var payload gitlabWebhookPayload
	if len(bodyBytes) > 0 {
		_ = json.Unmarshal(bodyBytes, &payload)
	}

	if event == "Merge Request Hook" || payload.ObjectKind == "merge_request" {
		h.handleGitLabMergeRequest(w, r, app, payload)
		return
	}

	targetBranch := "main"
	if app.Branch != nil && *app.Branch != "" {
		targetBranch = *app.Branch
	}

	if payload.Ref != "" {
		pushedBranch := strings.TrimPrefix(payload.Ref, "refs/heads/")
		if pushedBranch != targetBranch {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status":        "ignored",
				"reason":        "branch mismatch",
				"pushed_branch": pushedBranch,
				"target_branch": targetBranch,
			})
			return
		}
	}

	commitHash := payload.CheckoutSHA
	var commitMsg string
	if len(payload.Commits) > 0 {
		if commitHash == "" {
			commitHash = payload.Commits[len(payload.Commits)-1].ID
		}
		commitMsg = payload.Commits[len(payload.Commits)-1].Message
	}

	h.dispatchDeployment(w, app, targetBranch, commitHash, commitMsg)
}

func (h *WebhooksHandler) handleGeneric(w http.ResponseWriter, r *http.Request, app *models.Application, bodyBytes []byte) {
	if app.WebhookSecret != nil && *app.WebhookSecret != "" {
		sigHeader := r.Header.Get("X-Hub-Signature-256")
		labuhHeader := r.Header.Get("X-Labuh-Signature")
		authHeader := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		queryToken := r.URL.Query().Get("token")

		authorized := false
		if sigHeader != "" {
			mac := hmac.New(sha256.New, []byte(*app.WebhookSecret))
			mac.Write(bodyBytes)
			expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
			authorized = hmac.Equal([]byte(sigHeader), []byte(expectedSig))
		} else if labuhHeader != "" && subtle.ConstantTimeCompare([]byte(labuhHeader), []byte(*app.WebhookSecret)) == 1 {
			authorized = true
		} else if authHeader != "" && subtle.ConstantTimeCompare([]byte(authHeader), []byte(*app.WebhookSecret)) == 1 {
			authorized = true
		} else if queryToken != "" && subtle.ConstantTimeCompare([]byte(queryToken), []byte(*app.WebhookSecret)) == 1 {
			authorized = true
		}

		if !authorized {
			http.Error(w, `{"error":"unauthorized: invalid signature or token"}`, http.StatusUnauthorized)
			return
		}
	}

	var payload genericWebhookPayload
	if len(bodyBytes) > 0 {
		_ = json.Unmarshal(bodyBytes, &payload)
	}

	targetBranch := "main"
	if app.Branch != nil && *app.Branch != "" {
		targetBranch = *app.Branch
	}
	if payload.Branch != "" && payload.Branch != targetBranch {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":        "ignored",
			"reason":        "branch mismatch",
			"pushed_branch": payload.Branch,
			"target_branch": targetBranch,
		})
		return
	}

	h.dispatchDeployment(w, app, targetBranch, payload.CommitHash, payload.CommitMessage)
}

func (h *WebhooksHandler) dispatchDeployment(w http.ResponseWriter, app *models.Application, branch, commitHash, commitMsg string) {
	var hashPtr, msgPtr *string
	if commitHash != "" {
		hashPtr = &commitHash
	}
	if commitMsg != "" {
		msgPtr = &commitMsg
	}

	if h.deployRepo == nil {
		http.Error(w, `{"error":"deploy repository not configured"}`, http.StatusInternalServerError)
		return
	}

	deployment, err := h.deployRepo.Create(models.CreateDeploymentInput{
		ApplicationID: app.ID,
		CommitHash:    hashPtr,
		CommitMessage: msgPtr,
	})
	if err != nil {
		http.Error(w, `{"error":"failed to create deployment"}`, http.StatusInternalServerError)
		return
	}

	logPath := "/tmp/labuh-logs/" + deployment.ID + ".log"
	_ = h.appRepo.UpdateStatus(app.ID, models.AppStatusBuilding)

	if h.deployWorker != nil {
		var envVars []*models.AppEnvVar
		if h.envVarRepo != nil {
			envVars, _ = h.envVarRepo.GetByApplicationID(app.ID)
		}
		job := BuildDeploymentJob(app, deployment.ID, envVars)
		job.LogPath = logPath
		if branch != "" {
			job.Branch = branch
		}
		_ = h.deployWorker.Enqueue(job)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":         "queued",
		"deployment_id":  deployment.ID,
		"application_id": app.ID,
		"branch":         branch,
		"commit_hash":    commitHash,
		"commit_message": commitMsg,
	})
}

func (h *WebhooksHandler) handleGitHubPullRequest(w http.ResponseWriter, r *http.Request, app *models.Application, payload githubWebhookPayload) {
	if payload.PullRequest == nil || payload.Number <= 0 {
		http.Error(w, `{"error":"invalid pull_request payload"}`, http.StatusBadRequest)
		return
	}

	action := strings.ToLower(payload.Action)
	prNum := payload.Number
	branch := payload.PullRequest.Head.Ref
	commitHash := payload.PullRequest.Head.SHA
	commitMsg := payload.PullRequest.Title

	switch action {
	case "opened", "reopened", "synchronize":
		if !app.PRPreviewsEnabled {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "ignored",
				"reason": "pr previews disabled for this application",
			})
			return
		}

		previewApp, err := h.spawnOrUpdatePRPreview(r.Context(), app, prNum, branch)
		if err != nil {
			slog.Error("failed to spawn or update pr preview", "error", err, "pr_number", prNum)
			http.Error(w, `{"error":"failed to prepare pr preview environment"}`, http.StatusInternalServerError)
			return
		}

		h.dispatchDeployment(w, previewApp, branch, commitHash, commitMsg)

		h.postPRComment(r.Context(), app, prNum, gitbot.CommentParams{
			Status:     gitbot.StatusBuilding,
			CommitHash: commitHash,
			Branch:     branch,
		})

	case "closed":
		existing, err := h.appRepo.GetByParentAndPRWithContext(r.Context(), app.ID, prNum)
		if err != nil || existing == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "ignored",
				"reason": "no active preview found for this pr",
			})
			return
		}

		if err := h.teardownPreview(r.Context(), existing); err != nil {
			slog.Error("failed to teardown preview", "error", err, "preview_id", existing.ID)
			http.Error(w, `{"error":"failed to teardown preview"}`, http.StatusInternalServerError)
			return
		}

		h.postPRComment(r.Context(), app, prNum, gitbot.CommentParams{
			Status: gitbot.StatusDestroyed,
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":     "deleted",
			"preview_id": existing.ID,
			"pr_number":  prNum,
		})

	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ignored",
			"action": action,
		})
	}
}

func (h *WebhooksHandler) handleGitLabMergeRequest(w http.ResponseWriter, r *http.Request, app *models.Application, payload gitlabWebhookPayload) {
	if payload.ObjectAttributes == nil || payload.ObjectAttributes.IID <= 0 {
		http.Error(w, `{"error":"invalid merge_request payload"}`, http.StatusBadRequest)
		return
	}

	mrNum := payload.ObjectAttributes.IID
	action := strings.ToLower(payload.ObjectAttributes.Action)
	if action == "" {
		action = strings.ToLower(payload.ObjectAttributes.State)
	}

	branch := payload.ObjectAttributes.SourceBranch
	commitHash := payload.ObjectAttributes.LastCommit.ID
	commitMsg := payload.ObjectAttributes.LastCommit.Message
	if commitMsg == "" {
		commitMsg = payload.ObjectAttributes.Title
	}

	switch action {
	case "open", "reopen", "update", "opened", "reopened":
		if !app.PRPreviewsEnabled {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "ignored",
				"reason": "pr previews disabled for this application",
			})
			return
		}

		previewApp, err := h.spawnOrUpdatePRPreview(r.Context(), app, mrNum, branch)
		if err != nil {
			slog.Error("failed to spawn or update pr preview", "error", err, "mr_number", mrNum)
			http.Error(w, `{"error":"failed to prepare pr preview environment"}`, http.StatusInternalServerError)
			return
		}

		h.dispatchDeployment(w, previewApp, branch, commitHash, commitMsg)

		h.postPRComment(r.Context(), app, mrNum, gitbot.CommentParams{
			Status:     gitbot.StatusBuilding,
			CommitHash: commitHash,
			Branch:     branch,
		})

	case "close", "closed", "merge", "merged":
		existing, err := h.appRepo.GetByParentAndPRWithContext(r.Context(), app.ID, mrNum)
		if err != nil || existing == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "ignored",
				"reason": "no active preview found for this mr",
			})
			return
		}

		if err := h.teardownPreview(r.Context(), existing); err != nil {
			slog.Error("failed to teardown preview", "error", err, "preview_id", existing.ID)
			http.Error(w, `{"error":"failed to teardown preview"}`, http.StatusInternalServerError)
			return
		}

		h.postPRComment(r.Context(), app, mrNum, gitbot.CommentParams{
			Status: gitbot.StatusDestroyed,
		})

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":     "deleted",
			"preview_id": existing.ID,
			"mr_number":  mrNum,
		})

	default:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "ignored",
			"action": action,
		})
	}
}

func (h *WebhooksHandler) spawnOrUpdatePRPreview(ctx context.Context, parent *models.Application, prNum int, branch string) (*models.Application, error) {
	existing, err := h.appRepo.GetByParentAndPRWithContext(ctx, parent.ID, prNum)
	if err == nil && existing != nil {
		// Update branch if changed
		if branch != "" && (existing.Branch == nil || *existing.Branch != branch) {
			existing.Branch = &branch
		}
		return existing, nil
	}

	slug := fmt.Sprintf("%s-pr-%d", parent.Slug, prNum)
	name := fmt.Sprintf("%s (PR #%d)", parent.Name, prNum)

	var previewDomain *string
	if parent.PreviewTemplateDomain != nil && *parent.PreviewTemplateDomain != "" {
		resolved := strings.ReplaceAll(*parent.PreviewTemplateDomain, "{{pr}}", fmt.Sprintf("%d", prNum))
		resolved = strings.ReplaceAll(resolved, "{{app}}", parent.Slug)
		resolved = strings.TrimSpace(resolved)
		if resolved != "" {
			previewDomain = &resolved
		}
	}

	input := models.CreateApplicationInput{
		EnvironmentID:         parent.EnvironmentID,
		ServerID:              parent.ServerID,
		Name:                  name,
		Slug:                  slug,
		SourceType:            parent.SourceType,
		RepositoryURL:         parent.RepositoryURL,
		Branch:                &branch,
		BuildPath:             parent.BuildPath,
		DockerfilePath:        parent.DockerfilePath,
		DockerImage:           parent.DockerImage,
		AppPort:               parent.AppPort,
		CustomDomain:          previewDomain,
		HealthcheckPath:       parent.HealthcheckPath,
		HealthcheckTimeout:    parent.HealthcheckTimeout,
		CPULimit:              parent.CPULimit,
		MemoryLimit:           parent.MemoryLimit,
		MemoryReservation:     parent.MemoryReservation,
		BasicAuthUser:         parent.BasicAuthUser,
		BasicAuthPassword:     parent.BasicAuthPassword,
		ParentAppID:           &parent.ID,
		PRNumber:              &prNum,
		IsPreview:             true,
		PreviewTemplateDomain: parent.PreviewTemplateDomain,
		PRPreviewsEnabled:     false,
	}

	previewApp, err := h.appRepo.CreateWithContext(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("create preview app error: %w", err)
	}

	// Kloning environment variables dari induk
	if h.envVarRepo != nil {
		parentVars, err := h.envVarRepo.GetByApplicationIDWithContext(ctx, parent.ID)
		if err == nil {
			for _, v := range parentVars {
				_, _ = h.envVarRepo.CreateWithContext(ctx, models.CreateAppEnvVarInput{
					ApplicationID: previewApp.ID,
					Key:           v.Key,
					Value:         v.Value,
					IsSecret:      v.IsSecret,
					IsEncrypted:   v.IsEncrypted,
				})
			}
		}
	}

	return previewApp, nil
}

func (h *WebhooksHandler) teardownPreview(ctx context.Context, previewApp *models.Application) error {
	// 1. Remove Docker Container
	if h.dockerClient != nil && previewApp.ContainerID != nil && *previewApp.ContainerID != "" {
		if err := h.dockerClient.RemoveContainer(ctx, *previewApp.ContainerID); err != nil {
			slog.Warn("failed to remove docker container during preview teardown", "error", err, "container_id", *previewApp.ContainerID)
		}
	}

	// 2. Remove Caddy Route if domain exists
	if h.caddyClient != nil && previewApp.CustomDomain != nil && *previewApp.CustomDomain != "" {
		if err := h.caddyClient.RemoveRoute(*previewApp.CustomDomain); err != nil {
			slog.Warn("failed to remove caddy route during preview teardown", "error", err, "domain", *previewApp.CustomDomain)
		}
	}

	// 3. Delete database record
	return h.appRepo.DeleteWithContext(ctx, previewApp.ID)
}

// TeardownPRPreview mengeksekusi teardown preview aplikasi secara publik/handler.
func (h *WebhooksHandler) TeardownPRPreview(ctx context.Context, previewApp *models.Application) error {
	return h.teardownPreview(ctx, previewApp)
}

func (h *WebhooksHandler) resolveGitToken(ctx context.Context, app *models.Application, provider string) string {
	if app.GitToken != nil && *app.GitToken != "" {
		// Coba decrypt jika terenkripsi
		if decrypted, err := encryption.Decrypt(*app.GitToken); err == nil && decrypted != "" {
			return decrypted
		}
		return *app.GitToken
	}

	if h.settingRepo != nil {
		if provider == "github" {
			val, _ := h.settingRepo.Get(ctx, "github_bot_token")
			return val
		} else if provider == "gitlab" {
			val, _ := h.settingRepo.Get(ctx, "gitlab_bot_token")
			return val
		}
	}
	return ""
}

func (h *WebhooksHandler) postPRComment(ctx context.Context, app *models.Application, prNumber int, params gitbot.CommentParams) {
	if h.gitbotClient == nil || app.RepositoryURL == nil || *app.RepositoryURL == "" {
		return
	}

	provider, _, _, err := gitbot.ParseRepo(*app.RepositoryURL)
	if err != nil {
		return
	}

	token := h.resolveGitToken(ctx, app, provider)
	if token == "" {
		return
	}

	body := gitbot.FormatComment(params)
	go func() {
		reqCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var postErr error
		if provider == "github" {
			postErr = h.gitbotClient.PostOrUpdateGitHubComment(reqCtx, *app.RepositoryURL, prNumber, token, body)
		} else if provider == "gitlab" {
			postErr = h.gitbotClient.PostOrUpdateGitLabNote(reqCtx, *app.RepositoryURL, prNumber, token, body)
		}

		if postErr != nil {
			slog.Warn("failed to post pr preview comment", "provider", provider, "pr_number", prNumber, "error", postErr)
		}
	}()
}
