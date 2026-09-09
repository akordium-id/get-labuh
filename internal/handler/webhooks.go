package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

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

func (h *WebhooksHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	appID := chi.URLParam(r, "application_id")
	if appID == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByID(appID)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if app.WebhookSecret != nil && *app.WebhookSecret != "" {
		signature := r.Header.Get("X-Hub-Signature-256")
		if signature == "" {
			signature = r.Header.Get("X-Labuh-Signature")
		}
		if signature == "" {
			http.Error(w, "Unauthorized: missing signature header", http.StatusUnauthorized)
			return
		}

		bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, 10<<20))
		if err != nil {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		mac := hmac.New(sha256.New, []byte(*app.WebhookSecret))
		mac.Write(bodyBytes)
		expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
			http.Error(w, "Unauthorized: invalid signature", http.StatusUnauthorized)
			return
		}
	}

	deployment, err := h.deployRepo.Create(models.CreateDeploymentInput{
		ApplicationID: app.ID,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
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
		_ = h.deployWorker.Enqueue(job)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"queued","deployment_id":"` + deployment.ID + `"}`))
}
