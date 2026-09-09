package handler

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
)

type LogsHandler struct {
	deployRepo   *repo.DeploymentRepo
	appRepo      *repo.ApplicationRepo
	dockerClient *docker.Client
}

func NewLogsHandler(deployRepo *repo.DeploymentRepo, appRepo *repo.ApplicationRepo) *LogsHandler {
	return &LogsHandler{
		deployRepo: deployRepo,
		appRepo:    appRepo,
	}
}

func (h *LogsHandler) SetDockerClient(client *docker.Client) {
	h.dockerClient = client
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
	if deployment.LogPath != nil && *deployment.LogPath != "" {
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
			_, _ = w.Write([]byte("event: " + event + "\n"))
		}
		_, _ = w.Write([]byte("data: " + data + "\n\n"))
		flusher.Flush()
	}

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	var lastOffset int64 = 0
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if file, err := os.Open(logPath); err == nil {
				stat, sErr := file.Stat()
				if sErr == nil && stat.Size() > lastOffset {
					_, _ = file.Seek(lastOffset, io.SeekStart)
					scanner := bufio.NewScanner(file)
					for scanner.Scan() {
						line := scanner.Text()
						if line != "" {
							sendEvent("log-message", fmt.Sprintf(`{"output": "%s", "timestamp": "%s"}`, escapeJSON(line), time.Now().Format(time.RFC3339)))
						}
					}
					lastOffset = stat.Size()
				}
				_ = file.Close()
			}

			latestDeploy, dErr := h.deployRepo.GetByID(id)
			if dErr == nil && latestDeploy != nil {
				if latestDeploy.Status == models.DeployStatusSuccess || latestDeploy.Status == models.DeployStatusFailed {
					sendEvent("done", fmt.Sprintf(`{"status": "%s"}`, latestDeploy.Status))
					return
				}
			}
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
	if err != nil || app.ContainerID == nil || *app.ContainerID == "" {
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
			_, _ = w.Write([]byte("event: " + event + "\n"))
		}
		_, _ = w.Write([]byte("data: " + data + "\n\n"))
		flusher.Flush()
	}

	if h.dockerClient == nil {
		sendEvent("error", `{"message": "docker client not configured"}`)
		sendEvent("done", "{}")
		return
	}

	reader, err := h.dockerClient.ContainerLogs(r.Context(), *app.ContainerID, nil)
	if err != nil {
		sendEvent("error", fmt.Sprintf(`{"message": "%s"}`, escapeJSON(err.Error())))
		sendEvent("done", "{}")
		return
	}
	if reader == nil {
		sendEvent("log-message", fmt.Sprintf(`{"output": "Container is not emitting logs or running in simulated mode", "timestamp": "%s"}`, time.Now().Format(time.RFC3339)))
		sendEvent("done", "{}")
		return
	}
	defer reader.Close()

	pr, pw := io.Pipe()
	go func() {
		_ = docker.DemultiplexLogs(pw, reader)
		_ = pw.Close()
	}()

	scanner := bufio.NewScanner(pr)
	for scanner.Scan() {
		select {
		case <-r.Context().Done():
			return
		default:
			line := scanner.Text()
			if line != "" {
				sendEvent("log-message", fmt.Sprintf(`{"output": "%s", "timestamp": "%s"}`, escapeJSON(line), time.Now().Format(time.RFC3339)))
			}
		}
	}

	sendEvent("done", "{}")
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}
