package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/worker"
)

type WebhooksHandler struct {
	appRepo      *repo.ApplicationRepo
	deployRepo   *repo.DeploymentRepo
	envVarRepo   *repo.EnvVarRepo
	deployWorker *worker.DeployWorker
}

func NewWebhooksHandler(appRepo *repo.ApplicationRepo, deployRepo *repo.DeploymentRepo) *WebhooksHandler {
	return &WebhooksHandler{
		appRepo:    appRepo,
		deployRepo: deployRepo,
	}
}

func (h *WebhooksHandler) SetDeployWorker(w *worker.DeployWorker) {
	h.deployWorker = w
}

func (h *WebhooksHandler) SetEnvVarRepo(r *repo.EnvVarRepo) {
	h.envVarRepo = r
}

type githubWebhookPayload struct {
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
	ObjectKind  string `json:"object_kind"`
	Ref         string `json:"ref"`
	CheckoutSHA string `json:"checkout_sha"`
	Commits     []struct {
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

// Deploy adalah handler universal yang otomatis mendeteksi webhook GitHub, GitLab, atau Generic
func (h *WebhooksHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "application_id")
	if appID == "" {
		http.Error(w, `{"error":"missing application_id"}`, http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByID(appID)
	if err != nil {
		http.Error(w, `{"error":"application not found"}`, http.StatusNotFound)
		return
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
	if err != nil {
		http.Error(w, `{"error":"cannot read request body"}`, http.StatusBadRequest)
		return
	}

	// Deteksi provider berdasarkan request headers
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

	// Fallback ke Generic Webhook
	h.handleGeneric(w, r, app, bodyBytes)
}

// GitHub dedicated handler
func (h *WebhooksHandler) GitHub(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "application_id")
	if appID == "" {
		http.Error(w, `{"error":"missing application_id"}`, http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByID(appID)
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

// GitLab dedicated handler
func (h *WebhooksHandler) GitLab(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "application_id")
	if appID == "" {
		http.Error(w, `{"error":"missing application_id"}`, http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByID(appID)
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
	// Ping event dari GitHub untuk verifikasi webhook endpoint
	if event == "ping" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "pong",
			"message": "GitHub webhook configured successfully on Labuh",
		})
		return
	}

	// Validasi HMAC Signature
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

	// Branch verification
	targetBranch := "main"
	if app.Branch != nil && *app.Branch != "" {
		targetBranch = *app.Branch
	}

	if payload.Ref != "" {
		pushedBranch := strings.TrimPrefix(payload.Ref, "refs/heads/")
		if pushedBranch != targetBranch {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
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
	// Validasi Token GitLab
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

	targetBranch := "main"
	if app.Branch != nil && *app.Branch != "" {
		targetBranch = *app.Branch
	}

	if payload.Ref != "" {
		pushedBranch := strings.TrimPrefix(payload.Ref, "refs/heads/")
		if pushedBranch != targetBranch {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
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
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
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
	_ = h.deployRepo.MarkStarted(deployment.ID)
	_ = h.deployRepo.UpdateStatus(deployment.ID, models.DeployStatusQueued)

	if h.deployWorker != nil {
		var envVars []*models.AppEnvVar
		if h.envVarRepo != nil {
			envVars, _ = h.envVarRepo.GetByApplicationID(app.ID)
		}
		job := worker.BuildDeploymentJob(app, deployment.ID, envVars)
		job.LogPath = logPath
		if branch != "" {
			job.Branch = branch
		}
		_ = h.deployWorker.Enqueue(job)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "queued",
		"deployment_id":  deployment.ID,
		"application_id": app.ID,
		"branch":         branch,
		"commit_hash":    commitHash,
		"commit_message": commitMsg,
	})
}
