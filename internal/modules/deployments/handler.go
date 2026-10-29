package deployments

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/docker/docker/api/types/container"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	webdeploy "github.com/akordium-id/get-labuh/internal/web/pages/deployments"
)

// ApplicationProvider mengabstraksi akses aplikasi yang dibutuhkan oleh Handler deployments.
type ApplicationProvider interface {
	GetByID(id string) (*models.Application, error)
	UpdateStatus(id string, status models.AppStatus) error
}

// Handler menangani request HTTP dan SSE untuk domain Deployments.
type Handler struct {
	repo         *Repo
	appProvider  ApplicationProvider
	dockerClient *docker.Client
}

// NewHandler membuat instance baru Handler deployments.
func NewHandler(repo *Repo, appProvider ApplicationProvider) *Handler {
	return &Handler{
		repo:        repo,
		appProvider: appProvider,
	}
}

// SetDockerClient mengatur docker client untuk streaming container logs.
func (h *Handler) SetDockerClient(client *docker.Client) {
	h.dockerClient = client
}

// Get merender halaman detail deployment.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	deployment, err := h.repo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	var app *models.Application
	if h.appProvider != nil {
		app, _ = h.appProvider.GetByID(deployment.ApplicationID)
	}

	templ.Handler(layouts.AppLayout(webdeploy.DeploymentDetailPage(deployment, app))).ServeHTTP(w, r)
}

// Logs merender halaman log deployment.
func (h *Handler) Logs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	deployment, err := h.repo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	logPath := "/tmp/labuh-logs/" + deployment.ID + ".log"
	if deployment.LogPath != nil && *deployment.LogPath != "" {
		logPath = *deployment.LogPath
	}

	var logs []string
	content, err := readLogFile(logPath)
	if err == nil {
		logs = content
	}

	templ.Handler(layouts.AppLayout(webdeploy.DeploymentLogsPage(deployment, logs))).ServeHTTP(w, r)
}

// Rollback memulai deployment baru mengacu pada deployment sukses sebelumnya.
func (h *Handler) Rollback(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	deployment, err := h.repo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	prevDeployment, err := h.repo.GetPreviousSuccessfulWithContext(r.Context(), deployment.ApplicationID)
	if err != nil || prevDeployment == nil {
		http.Error(w, "No previous successful deployment found", http.StatusBadRequest)
		return
	}

	rollbackDeployment, err := h.repo.CreateWithContext(r.Context(), models.CreateDeploymentInput{
		ApplicationID: deployment.ApplicationID,
		CommitHash:    prevDeployment.CommitHash,
		CommitMessage: prevDeployment.CommitMessage,
		LogPath:       prevDeployment.LogPath,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	_ = h.repo.MarkStartedWithContext(r.Context(), rollbackDeployment.ID)
	_ = h.repo.UpdateStatusWithContext(r.Context(), rollbackDeployment.ID, models.DeployStatusDeploying)
	if h.appProvider != nil {
		_ = h.appProvider.UpdateStatus(deployment.ApplicationID, models.AppStatusRunning)
	}

	w.Header().Set("HX-Redirect", "/deployments/"+rollbackDeployment.ID)
	w.WriteHeader(http.StatusOK)
}

// Start memulai ulang/menjalankan deployment.
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	deployment, err := h.repo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	_ = h.repo.MarkStartedWithContext(r.Context(), deployment.ID)
	_ = h.repo.UpdateStatusWithContext(r.Context(), deployment.ID, models.DeployStatusCloning)

	w.Header().Set("HX-Redirect", "/deployments/"+id)
	w.WriteHeader(http.StatusOK)
}

// StreamDeploymentLogs streaming log deployment realtime via SSE.
func (h *Handler) StreamDeploymentLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	deployment, err := h.repo.GetByIDWithContext(r.Context(), id)
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
	var lastStep = ""
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
					if sErr := scanner.Err(); sErr != nil {
						slog.Debug("scanner error while reading deploy logs", "error", sErr)
					}
					lastOffset = stat.Size()
				}
				_ = file.Close()
			}

			latestDeploy, dErr := h.repo.GetByIDWithContext(r.Context(), id)
			if dErr == nil && latestDeploy != nil {
				currentStep := ""
				if latestDeploy.Step != nil {
					currentStep = *latestDeploy.Step
				}
				if currentStep != "" && currentStep != lastStep {
					lastStep = currentStep
					sendEvent("step-changed", fmt.Sprintf(`{"step": "%s", "status": "%s"}`, currentStep, latestDeploy.Status))
				}

				if latestDeploy.Status == models.DeployStatusSuccess || latestDeploy.Status == models.DeployStatusFailed || latestDeploy.Status == models.DeployStatusCancelled {
					// Final log drain
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
							if sErr := scanner.Err(); sErr != nil {
								slog.Debug("scanner error while final draining deploy logs", "error", sErr)
							}
						}
						_ = file.Close()
					}
					errMsg := ""
					if latestDeploy.ErrorMessage != nil {
						errMsg = *latestDeploy.ErrorMessage
					}
					sendEvent("done", fmt.Sprintf(`{"status": "%s", "error": "%s"}`, latestDeploy.Status, escapeJSON(errMsg)))
					return
				}
			}
		}
	}
}

// StreamContainerLogs streaming container log realtime via SSE.
func (h *Handler) StreamContainerLogs(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if h.appProvider == nil {
		http.Error(w, "Application provider not configured", http.StatusInternalServerError)
		return
	}

	app, err := h.appProvider.GetByID(id)
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

	reader, err := h.dockerClient.ContainerLogs(r.Context(), *app.ContainerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "100",
	})
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

	if err := scanner.Err(); err != nil {
		slog.Debug("scanner error while reading container logs", "error", err)
	}

	sendEvent("done", "{}")
}

func readLogFile(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	rawLines := strings.Split(string(content), "\n")
	var lines []string
	for _, l := range rawLines {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines, nil
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}
