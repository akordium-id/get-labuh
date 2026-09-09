package observability

import (
	"expvar"
	"log/slog"
	"net/http"
	"runtime"
	"strconv"
	"time"
)

var (
	deployDuration  *expvar.Map
	buildDuration   *expvar.Map
	containerCount  expvar.Int
	requestDuration *expvar.Map
)

func init() {
	deployDuration = expvar.NewMap("deploy_duration_seconds")
	buildDuration = expvar.NewMap("build_duration_seconds")
	containerCount = expvar.Int{}
	requestDuration = expvar.NewMap("request_duration_seconds")
}

func ObserveDeployDuration(d time.Duration) {
	deployDuration.Add("total", int64(d.Seconds()))
}

func ObserveBuildDuration(d time.Duration) {
	buildDuration.Add("total", int64(d.Seconds()))
}

func SetContainerCount(n int64) {
	containerCount.Set(n)
}

func ObserveRequestDuration(d time.Duration) {
	requestDuration.Add("total", int64(d.Seconds()))
}

func ProfilingHandler(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte("{\n"))
	_, _ = w.Write([]byte("  \"goroutines\": " + strconv.Itoa(runtime.NumGoroutine()) + ",\n"))
	_, _ = w.Write([]byte("  \"memory_alloc\": " + strconv.FormatUint(uint64(memStats.Alloc), 10) + ",\n"))
	_, _ = w.Write([]byte("  \"memory_total_alloc\": " + strconv.FormatUint(uint64(memStats.TotalAlloc), 10) + ",\n"))
	_, _ = w.Write([]byte("  \"memory_sys\": " + strconv.FormatUint(uint64(memStats.Sys), 10) + ",\n"))
	_, _ = w.Write([]byte("  \"num_gc\": " + strconv.FormatUint(uint64(memStats.NumGC), 10) + ",\n"))
	_, _ = w.Write([]byte("  \"num_goroutine\": " + strconv.Itoa(runtime.NumGoroutine()) + ",\n"))
	_, _ = w.Write([]byte("  \"container_count\": " + strconv.FormatInt(containerCount.Value(), 10) + "\n"))
	_, _ = w.Write([]byte("}\n"))

	slog.Info("profiling data requested")
}
