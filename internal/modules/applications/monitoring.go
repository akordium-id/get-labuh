package applications

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/components"
)

// MonitoringHandler menangani pembacaan metrik resource container aplikasi.
type MonitoringHandler struct {
	appRepo      *Repo
	dockerClient *docker.Client
}

// NewMonitoringHandler membuat instance baru MonitoringHandler.
func NewMonitoringHandler(appRepo *Repo, dockerClient *docker.Client) *MonitoringHandler {
	return &MonitoringHandler{
		appRepo:      appRepo,
		dockerClient: dockerClient,
	}
}

// SetDockerClient menyetel docker client.
func (h *MonitoringHandler) SetDockerClient(c *docker.Client) {
	h.dockerClient = c
}

// Stats mengembalikan metrik realtime CPU, Memory, dan Network container aplikasi.
func (h *MonitoringHandler) Stats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByIDWithContext(r.Context(), id)
	if err != nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	stats := models.ContainerStats{
		CPUPercent:  0,
		MemoryUsage: 0,
		MemoryLimit: 0,
		NetworkRx:   0,
		NetworkTx:   0,
	}

	if h.dockerClient != nil && app.ContainerID != nil && *app.ContainerID != "" {
		cpu, mem, limit, rx, tx, err := h.dockerClient.GetContainerStats(r.Context(), *app.ContainerID)
		if err == nil {
			stats.CPUPercent = cpu
			stats.MemoryUsage = mem
			stats.MemoryLimit = limit
			stats.NetworkRx = rx
			stats.NetworkTx = tx
		}
	}

	if r.Header.Get("HX-Request") == "true" || strings.Contains(r.Header.Get("Accept"), "text/html") {
		templ.Handler(components.MetricsCard(stats, "/applications/"+id+"/stats")).ServeHTTP(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}
