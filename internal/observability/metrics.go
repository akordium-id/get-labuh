package observability

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime"
	"sync"
	"time"
)

type Metrics struct {
	Timestamp         time.Time      `json:"timestamp"`
	Uptime            string         `json:"uptime"`
	GoVersion         string         `json:"go_version"`
	NumGoroutine      int            `json:"num_goroutine"`
	NumGC             uint32         `json:"num_gc"`
	Runtime           RuntimeMetrics `json:"runtime"`
	DeploymentDurations []float64    `json:"deployment_durations"`
	BuildDurations      []float64    `json:"build_durations"`
	ContainerCount      int          `json:"container_count"`
	RequestDurations    []float64    `json:"request_durations"`
}

var (
	startTime           = time.Now()
	deploymentDurations []float64
	buildDurations      []float64
	containerCount      int
	requestDurations    []float64
	metricsMu           sync.RWMutex
)

func RecordDeploymentDuration(d time.Duration) {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	deploymentDurations = append(deploymentDurations, d.Seconds())
}

func RecordBuildDuration(d time.Duration) {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	buildDurations = append(buildDurations, d.Seconds())
}

func SetContainerCount(count int) {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	containerCount = count
}

func RecordRequestDuration(d time.Duration) {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	requestDurations = append(requestDurations, d.Seconds())
}

func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	metricsMu.RLock()
	deployDurs := make([]float64, len(deploymentDurations))
	copy(deployDurs, deploymentDurations)
	buildDurs := make([]float64, len(buildDurations))
	copy(buildDurs, buildDurations)
	containerC := containerCount
	reqDurs := make([]float64, len(requestDurations))
	copy(reqDurs, requestDurations)
	metricsMu.RUnlock()

	metrics := Metrics{
		Timestamp:         time.Now(),
		Uptime:            time.Since(startTime).String(),
		GoVersion:         runtime.Version(),
		NumGoroutine:      runtime.NumGoroutine(),
		NumGC:             m.NumGC,
		Runtime:           GetRuntimeMetrics(),
		DeploymentDurations: deployDurs,
		BuildDurations:      buildDurs,
		ContainerCount:      containerC,
		RequestDurations:    reqDurs,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		slog.Error("failed to encode metrics", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	slog.Info("metrics requested")
}
