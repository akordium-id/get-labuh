package observability

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime"
	"time"
)

type Metrics struct {
	Timestamp    time.Time `json:"timestamp"`
	Uptime       string    `json:"uptime"`
	GoVersion    string    `json:"go_version"`
	NumGoroutine int       `json:"num_goroutine"`
	NumGC        uint32    `json:"num_gc"`
	ContainerCount int64   `json:"container_count"`
	DeployDuration float64 `json:"deploy_duration_seconds"`
	BuildDuration float64  `json:"build_duration_seconds"`
	RequestDuration float64 `json:"request_duration_seconds"`
}

var startTime = time.Now()

func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metrics := Metrics{
		Timestamp:       time.Now(),
		Uptime:          time.Since(startTime).String(),
		GoVersion:       runtime.Version(),
		NumGoroutine:    runtime.NumGoroutine(),
		NumGC:           m.NumGC,
		ContainerCount:  containerCount.Value(),
		DeployDuration:  0,
		BuildDuration:   0,
		RequestDuration: 0,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		slog.Error("failed to encode metrics", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	slog.Info("metrics requested")
}
