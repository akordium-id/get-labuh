package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/deployments"
)

type DeploymentsHandler struct {
	deployRepo *repo.DeploymentRepo
	appRepo    *repo.ApplicationRepo
}

func NewDeploymentsHandler(deployRepo *repo.DeploymentRepo, appRepo *repo.ApplicationRepo) *DeploymentsHandler {
	return &DeploymentsHandler{
		deployRepo: deployRepo,
		appRepo:    appRepo,
	}
}

func (h *DeploymentsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	deployment, err := h.deployRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	app, _ := h.appRepo.GetByID(deployment.ApplicationID)

	templ.Handler(layouts.AppLayout(deployments.DeploymentDetailPage(deployment, app))).ServeHTTP(w, r)
}

func (h *DeploymentsHandler) Logs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	deployment, err := h.deployRepo.GetByID(id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	logPath := "/tmp/labuh-logs/" + deployment.ID + ".log"
	if deployment.LogPath != nil {
		logPath = *deployment.LogPath
	}

	var logs []string
	content, err := readLogFile(logPath)
	if err == nil {
		logs = content
	}

	templ.Handler(layouts.AppLayout(deployments.DeploymentLogsPage(deployment, logs))).ServeHTTP(w, r)
}

func readLogFile(path string) ([]string, error) {
	_ = path
	return nil, nil
}

func formatTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}

func calculateDuration(started, finished *time.Time) string {
	if started == nil || finished == nil {
		return "-"
	}
	d := finished.Sub(*started)
	if d < time.Minute {
		return d.String()
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", h, m)
}
