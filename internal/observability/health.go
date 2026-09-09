package observability

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

type HealthCheck struct {
	Status    HealthStatus     `json:"status"`
	Timestamp time.Time        `json:"timestamp"`
	Version   string           `json:"version,omitempty"`
	Checks    map[string]Check `json:"checks"`
	Uptime    time.Duration    `json:"uptime,omitempty"`
}

type Check struct {
	Status  HealthStatus `json:"status"`
	Message string       `json:"message,omitempty"`
	Latency string       `json:"latency,omitempty"`
}

type HealthChecker struct {
	startTime time.Time
	version   string
	db        *sql.DB
}

func NewHealthChecker(version string, db *sql.DB) *HealthChecker {
	return &HealthChecker{
		startTime: time.Now(),
		version:   version,
		db:        db,
	}
}

func (h *HealthChecker) Check(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]Check)

	dbCheck := h.checkDatabase()
	checks["database"] = dbCheck

	status := HealthStatusHealthy
	for _, check := range checks {
		if check.Status == HealthStatusUnhealthy {
			status = HealthStatusUnhealthy
			break
		}
		if check.Status == HealthStatusDegraded && status == HealthStatusHealthy {
			status = HealthStatusDegraded
		}
	}

	response := map[string]any{
		"status":    status,
		"timestamp": time.Now(),
		"version":   h.version,
		"checks":    checks,
		"uptime":    time.Since(h.startTime).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	if status == HealthStatusUnhealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	slog.Info("health check", "status", status)
	json.NewEncoder(w).Encode(response)
}

func (h *HealthChecker) checkDatabase() Check {
	start := time.Now()
	if h.db == nil {
		return Check{Status: HealthStatusUnhealthy, Message: "database not initialized"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := h.db.PingContext(ctx)
	latency := time.Since(start)

	if err != nil {
		slog.Error("database health check failed", "error", err)
		return Check{Status: HealthStatusUnhealthy, Message: err.Error(), Latency: latency.String()}
	}

	return Check{Status: HealthStatusHealthy, Message: "ok", Latency: latency.String()}
}

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}

		ctx := r.Context()
		slog.Info("request started", "request_id", requestID, "method", r.Method, "path", r.URL.Path)

		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func generateRequestID() string {
	return uuid.New().String()
}
