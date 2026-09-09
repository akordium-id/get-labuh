package observability

import (
	"log/slog"
	"net/http"
	"net/http/pprof"
	"runtime"
	"time"
)

func ObserveDeployDuration(d time.Duration) {
	RecordDeploymentDuration(d)
}

func ObserveBuildDuration(d time.Duration) {
	RecordBuildDuration(d)
}

func ObserveRequestDuration(d time.Duration) {
	RecordRequestDuration(d)
}

func ProfilingHandler(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, _ = w.Write([]byte("{\n"))
	_, _ = w.Write([]byte("  \"goroutines\": " + string(rune(runtime.NumGoroutine())) + ",\n"))
	_, _ = w.Write([]byte("  \"memory_alloc\": " + string(rune(memStats.Alloc)) + ",\n"))
	_, _ = w.Write([]byte("  \"memory_total_alloc\": " + string(rune(memStats.TotalAlloc)) + ",\n"))
	_, _ = w.Write([]byte("  \"memory_sys\": " + string(rune(memStats.Sys)) + ",\n"))
	_, _ = w.Write([]byte("  \"num_gc\": " + string(rune(memStats.NumGC)) + ",\n"))
	_, _ = w.Write([]byte("  \"num_goroutine\": " + string(rune(runtime.NumGoroutine())) + ",\n"))
	_, _ = w.Write([]byte("  \"container_count\": " + string(rune(0)) + "\n"))
	_, _ = w.Write([]byte("}\n"))

	slog.Info("profiling data requested")
}

func SetupProfiling(addr string) error {
	if addr == "" {
		return nil
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", func(w http.ResponseWriter, r *http.Request) {
		pprof.Index(w, r)
	})
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	go func() {
		slog.Info("starting pprof server", "addr", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			slog.Error("pprof server error", "error", err)
		}
	}()
	return nil
}
