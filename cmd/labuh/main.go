package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/akordium-id/get-labuh/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) > 1 && os.Args[1] == "runner" {
		runRunnerMode(ctx, os.Args[2:])
		return
	}
	if os.Getenv("LABUH_MODE") == "runner" {
		runRunnerMode(ctx, os.Args[1:])
		return
	}

	cfg := app.LoadConfig()

	application, err := app.New(cfg)
	if err != nil {
		slog.Error("Failed to initialize Labuh application", "error", err)
		os.Exit(1)
	}

	if err := application.Run(ctx); err != nil {
		slog.Error("Application exited with error", "error", err)
		os.Exit(1)
	}
}
