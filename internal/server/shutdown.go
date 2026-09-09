package server

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"time"
)

func GracefulShutdown(ctx context.Context, server *http.Server, worker interface{ Stop() }, db *sql.DB) error {
	slog.Info("initiating graceful shutdown")

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("http server shutdown error", "error", err)
	}

	slog.Info("stopping deploy worker")
	if worker != nil {
		worker.Stop()
	}

	if db != nil {
		slog.Info("closing database connection")
		if err := db.Close(); err != nil {
			slog.Error("database close error", "error", err)
		}
	}

	slog.Info("shutdown complete")
	return nil
}

func WaitForShutdownSignal() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		cancel()
	}()

	return ctx
}
