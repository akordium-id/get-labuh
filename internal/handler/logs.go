package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/database/repo"
)

type LogsHandler struct {
	deployRepo *repo.DeploymentRepo
	appRepo    *repo.ApplicationRepo
}

func NewLogsHandler(deployRepo *repo.DeploymentRepo, appRepo *repo.ApplicationRepo) *LogsHandler {
	return &LogsHandler{
		deployRepo: deployRepo,
		appRepo:    appRepo,
	}
}

func (h *LogsHandler) StreamDeploymentLogs(w http.ResponseWriter, r *http.Request) {
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

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	sendEvent := func(event, data string) {
		if event != "" {
			w.Write([]byte("event: " + event + "\n"))
		}
		w.Write([]byte("data: " + data + "\n\n"))
		flusher.Flush()
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastSize int64
	for {
		select {
		case <-r.Context().Done():
			sendEvent("error", `{"message": "connection closed"}`)
			return
		case <-ticker.C:
			_ = logPath
			_ = lastSize
			sendEvent("log-message", `{"output": "waiting for logs...", "timestamp": "`+time.Now().Format(time.RFC3339)+`"}`)
		}
	}
}

func (h *LogsHandler) StreamContainerLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByID(id)
	if err != nil || app.ContainerID == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	sendEvent := func(event, data string) {
		if event != "" {
			w.Write([]byte("event: " + event + "\n"))
		}
		w.Write([]byte("data: " + data + "\n\n"))
		flusher.Flush()
	}

	sendEvent("error", `{"message": "docker client not implemented"}`)
	sendEvent("done", "{}")
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}
