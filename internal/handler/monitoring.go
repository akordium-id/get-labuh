package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/components"
)

type MonitoringHandler struct {
	appRepo      *repo.ApplicationRepo
	dbRepo       *repo.DatabaseRepo
	dockerClient *docker.Client
}

func NewMonitoringHandler(appRepo *repo.ApplicationRepo, dbRepo *repo.DatabaseRepo, dockerCli any) *MonitoringHandler {
	var dc *docker.Client
	if cli, ok := dockerCli.(*docker.Client); ok {
		dc = cli
	}
	return &MonitoringHandler{
		appRepo:      appRepo,
		dbRepo:       dbRepo,
		dockerClient: dc,
	}
}

func (h *MonitoringHandler) ApplicationStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	app, err := h.appRepo.GetByID(id)
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

func (h *MonitoringHandler) DatabaseStats(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	dbService, err := h.dbRepo.GetByID(id)
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

	if h.dockerClient != nil && dbService.ContainerID != nil && *dbService.ContainerID != "" {
		cpu, mem, limit, rx, tx, err := h.dockerClient.GetContainerStats(r.Context(), *dbService.ContainerID)
		if err == nil {
			stats.CPUPercent = cpu
			stats.MemoryUsage = mem
			stats.MemoryLimit = limit
			stats.NetworkRx = rx
			stats.NetworkTx = tx
		}
	}

	if r.Header.Get("HX-Request") == "true" || strings.Contains(r.Header.Get("Accept"), "text/html") {
		templ.Handler(components.MetricsCard(stats, "/databases/"+id+"/stats")).ServeHTTP(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

func FormatBytes(b uint64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	}
	if b < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(b)/(1024*1024*1024))
}

func FormatPercent(p float64) string {
	return fmt.Sprintf("%.1f%%", p)
}
