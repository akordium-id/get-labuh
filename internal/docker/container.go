package docker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type ContainerStatus string

const (
	ContainerStatusRunning  ContainerStatus = "running"
	ContainerStatusStopped  ContainerStatus = "stopped"
	ContainerStatusBuilding ContainerStatus = "building"
	ContainerStatusFailed   ContainerStatus = "failed"
)

type ContainerNotFoundError struct {
	ContainerID string
}

func (e *ContainerNotFoundError) Error() string {
	return fmt.Sprintf("container not found: %s", e.ContainerID)
}

type DockerUnavailableError struct {
	Cause error
}

func (e *DockerUnavailableError) Error() string {
	return fmt.Sprintf("docker daemon unavailable: %v", e.Cause)
}

func (c *Client) GenerateContainerName(envSlug, appSlug, appID string) string {
	shortID := appID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	cleanEnv := strings.ReplaceAll(strings.ToLower(envSlug), "_", "-")
	cleanApp := strings.ReplaceAll(strings.ToLower(appSlug), "_", "-")
	return fmt.Sprintf("labuh-%s-%s-%s", cleanEnv, cleanApp, shortID)
}

func (c *Client) StopContainer(ctx context.Context, containerID string, timeout int) error {
	if containerID == "" {
		return fmt.Errorf("empty container ID")
	}

	if c.cli == nil {
		slog.Warn("docker client not initialized, skipping stop", "container_id", containerID)
		return nil
	}

	running, err := c.IsContainerRunning(ctx, containerID)
	if err != nil {
		if _, ok := err.(*ContainerNotFoundError); ok {
			return nil
		}
		return &DockerUnavailableError{Cause: err}
	}

	if !running {
		slog.Info("container already stopped", "container_id", containerID)
		return nil
	}

	if timeout <= 0 {
		timeout = 10
	}

	slog.Info("stopping container", "container_id", containerID, "timeout_s", timeout)
	stopTimeout := timeout
	return c.cli.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &stopTimeout,
	})
}

func (c *Client) RestartContainer(ctx context.Context, containerID string, timeout int) error {
	if containerID == "" {
		return fmt.Errorf("empty container ID")
	}

	if err := c.StopContainer(ctx, containerID, timeout); err != nil {
		return err
	}

	if err := c.StartContainer(ctx, containerID); err != nil {
		return err
	}

	return nil
}

func (c *Client) IsContainerRunning(ctx context.Context, containerID string) (bool, error) {
	if containerID == "" {
		return false, fmt.Errorf("empty container ID")
	}

	if c.cli == nil {
		return false, fmt.Errorf("docker client not initialized")
	}

	insp, err := c.InspectContainer(ctx, containerID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return false, &ContainerNotFoundError{ContainerID: containerID}
		}
		return false, err
	}

	return insp.State != nil && insp.State.Running, nil
}

func (c *Client) GetContainerStatus(ctx context.Context, containerID string) (string, error) {
	if containerID == "" {
		return "", fmt.Errorf("empty container ID")
	}

	if c.cli == nil {
		return string(ContainerStatusStopped), nil
	}

	insp, err := c.InspectContainer(ctx, containerID)
	if err != nil {
		if client.IsErrNotFound(err) {
			return string(ContainerStatusStopped), nil
		}
		return string(ContainerStatusFailed), err
	}

	if insp.State != nil && insp.State.Running {
		return string(ContainerStatusRunning), nil
	}

	return string(ContainerStatusStopped), nil
}

func (c *Client) RunContainer(ctx context.Context, appID, envSlug, appSlug, imageName string, appPort int, envVars []struct{ Key, Value string }) (string, error) {
	if c.cli == nil {
		return "", fmt.Errorf("docker client not initialized")
	}

	networkName := "labuh-network"
	if err := c.EnsureNetwork(ctx, networkName); err != nil {
		slog.Warn("failed to ensure labuh network, proceeding with default", "error", err)
	}

	containerName := c.GenerateContainerName(envSlug, appSlug, appID)

	// Build environment variable slice
	envs := make([]string, 0, len(envVars))
	for _, ev := range envVars {
		envs = append(envs, fmt.Sprintf("%s=%s", ev.Key, ev.Value))
	}

	cfg := &container.Config{
		Image: imageName,
		Env:   envs,
	}

	hostCfg := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
	}

	if appPort > 0 {
		portProto := nat.Port(fmt.Sprintf("%d/tcp", appPort))
		cfg.ExposedPorts = nat.PortSet{
			portProto: struct{}{},
		}
	}

	netCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkName: {},
		},
	}

	// If container already exists with same name, remove it first
	_ = c.RemoveContainer(ctx, containerName)

	containerID, err := c.CreateContainer(ctx, cfg, hostCfg, netCfg, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	if err := c.StartContainer(ctx, containerID); err != nil {
		_ = c.RemoveContainer(ctx, containerID)
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	return containerID, nil
}

func (c *Client) RunDatabaseContainer(ctx context.Context, envSlug, dbSlug, imageName string, envVars map[string]string, ports []string, volumeName string) (string, error) {
	if c.cli == nil {
		return "", fmt.Errorf("docker client not initialized")
	}

	networkName := "labuh-network"
	_ = c.EnsureNetwork(ctx, networkName)

	containerName := fmt.Sprintf("labuh-db-%s-%s", envSlug, dbSlug)

	envs := make([]string, 0, len(envVars))
	for k, v := range envVars {
		envs = append(envs, fmt.Sprintf("%s=%s", k, v))
	}

	cfg := &container.Config{
		Image: imageName,
		Env:   envs,
	}

	hostCfg := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{
			Name: "unless-stopped",
		},
	}

	if volumeName != "" {
		hostCfg.Binds = []string{fmt.Sprintf("%s:/var/lib/data", volumeName)}
	}

	_ = c.RemoveContainer(ctx, containerName)

	containerID, err := c.CreateContainer(ctx, cfg, hostCfg, &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkName: {},
		},
	}, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create database container: %w", err)
	}

	if err := c.StartContainer(ctx, containerID); err != nil {
		_ = c.RemoveContainer(ctx, containerID)
		return "", fmt.Errorf("failed to start database container: %w", err)
	}

	return containerID, nil
}
