package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/akordium-id/get-labuh/internal/caddy"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/runner"
)

func runRunnerMode(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("runner", flag.ExitOnError)
	hubURL := fs.String("hub-url", os.Getenv("LABUH_HUB_URL"), "WebSocket URL of the SaaS Control Plane Hub (e.g. wss://hub.labuh.id/v1/tunnel)")
	token := fs.String("token", os.Getenv("LABUH_NODE_TOKEN"), "Secret authentication token for this node")
	heartbeatSec := fs.Int("heartbeat", 15, "Heartbeat interval in seconds")
	caddyURL := fs.String("caddy-url", os.Getenv("CADDY_API_URL"), "Caddy Admin API URL")
	caddyKey := fs.String("caddy-key", os.Getenv("CADDY_API_KEY"), "Caddy Admin API Key")
	_ = fs.Parse(args)

	if *hubURL == "" {
		slog.Error("missing required hub-url (specify --hub-url flag or LABUH_HUB_URL env var)")
		os.Exit(1)
	}
	if *token == "" {
		slog.Error("missing required node token (specify --token flag or LABUH_NODE_TOKEN env var)")
		os.Exit(1)
	}

	slog.Info("starting Labuh in Runner Agent mode (BYOS)")

	dockerClient, err := docker.NewClient()
	if err != nil {
		slog.Error("failed to connect to Docker", "error", err)
		os.Exit(1)
	}

	var caddyClient *caddy.Client
	if *caddyURL != "" {
		caddyClient = caddy.NewClient(*caddyURL, *caddyKey)
	}

	executor := runner.NewExecutor(dockerClient, caddyClient)
	collector := runner.NewMetricsCollector(dockerClient)

	cfg := runner.Config{
		HubURL:            *hubURL,
		NodeToken:         *token,
		Version:           "1.0.0",
		HeartbeatInterval: time.Duration(*heartbeatSec) * time.Second,
	}

	client := runner.NewClient(cfg, executor, collector)

	if err := client.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("runner stopped with error", "error", err)
		os.Exit(1)
	}
	slog.Info("runner shut down gracefully")
}
