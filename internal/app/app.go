package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/observability"
	"github.com/akordium-id/get-labuh/internal/server"
)

// App merepresentasikan server runtime aplikasi get-labuh.
type App struct {
	cfg       Config
	container *Container
	server    *http.Server
	db        *sql.DB
}

// New menginisialisasi database, migrasi, container dependensi, dan router server.
func New(cfg Config) (*App, error) {
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if err := database.RunAllMigrations(db); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	container, err := BuildContainer(db, cfg)
	if err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("failed to build container: %w", err)
	}

	router := SetupRoutes(container)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
	}

	return &App{
		cfg:       cfg,
		container: container,
		server:    srv,
		db:        db,
	}, nil
}

// Run menjalankan HTTP/TLS server dan menunggu signal termination untuk graceful shutdown.
func (a *App) Run(ctx context.Context) error {
	defer a.container.BackupScheduler.Stop()
	defer a.container.DeployWorker.Stop()
	defer func() {
		if err := database.Close(); err != nil {
			slog.Warn("Failed to close database", "error", err)
		}
	}()

	_ = observability.SetupProfiling(a.cfg.ProfilingAddr)

	serverErr := make(chan error, 1)
	go func() {
		if a.cfg.TLSEnabled && a.cfg.TLSCertPath != "" && a.cfg.TLSKeyPath != "" {
			slog.Info("Labuh server starting with TLS", "port", a.cfg.Port)
			if err := a.server.ListenAndServeTLS(a.cfg.TLSCertPath, a.cfg.TLSKeyPath); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serverErr <- err
			}
		} else {
			slog.Info("Labuh server starting", "port", a.cfg.Port)
			if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				serverErr <- err
			}
		}
	}()

	select {
	case err := <-serverErr:
		return err
	case <-ctx.Done():
		slog.Info("Shutting down server gracefully...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return server.GracefulShutdown(shutdownCtx, a.server, a.container.DeployWorker, a.db)
	}
}
