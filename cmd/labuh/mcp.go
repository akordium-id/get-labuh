package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/akordium-id/get-labuh/internal/app"
	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/mcp"
)

func runMCPMode(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	dbPath := fs.String("db", os.Getenv("DATABASE_URL"), "Path to SQLite database file (e.g. labuh.db)")
	_ = fs.Parse(args)

	// Direct all operational logs to stderr so stdout is reserved exclusively for MCP JSON-RPC protocol
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelWarn,
	}))
	slog.SetDefault(logger)

	db, err := database.Connect(*dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := database.RunAllMigrations(db); err != nil {
		fmt.Fprintf(os.Stderr, "Error running migrations: %v\n", err)
		os.Exit(1)
	}

	cfg := app.LoadConfig()
	container, err := app.BuildContainer(db, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error building container: %v\n", err)
		os.Exit(1)
	}

	mcpServer := container.MCPServer
	if mcpServer == nil {
		mcpServer = mcp.NewServer(mcp.ServerDeps{
			AppRepo:       container.AppRepo,
			DeployRepo:    container.DeployRepo,
			SharedEnvRepo: container.SharedEnvRepo,
			DeployWorker:  container.DeployWorker,
			DockerPool:    container.DockerPool,
			EnvResolver:   container.EnvResolver,
		})
	}

	if err := mcpServer.ServeStdio(); err != nil {
		fmt.Fprintf(os.Stderr, "MCP stdio server terminated with error: %v\n", err)
		os.Exit(1)
	}
}
