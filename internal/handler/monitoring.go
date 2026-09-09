package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
)

type MonitoringHandler struct {
	appRepo   *repo.ApplicationRepo
	dbRepo    *repo.DatabaseRepo
	dockerCli any
}

func NewMonitoringHandler(appRepo *repo.ApplicationRepo, dbRepo *repo.DatabaseRepo, dockerCli any) *MonitoringHandler {
	return &MonitoringHandler{
		appRepo:   appRepo,
		dbRepo:    dbRepo,
		dockerCli: dockerCli,
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

	if app.ContainerID != nil && *app.ContainerID != "" {
		_ = h.dockerCli
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
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

	if dbService.ContainerID != nil && *dbService.ContainerID != "" {
		_ = h.dockerCli
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
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
