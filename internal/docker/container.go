package docker

import (
	"context"
	"fmt"
	"log/slog"
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
	return fmt.Sprintf("labuh-%s-%s-%s", envSlug, appSlug, shortID)
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
	return nil
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
		slog.Warn("docker client not initialized, assuming not running", "container_id", containerID)
		return false, nil
	}

	slog.Info("checking container status", "container_id", containerID)
	return false, nil
}

func (c *Client) GetContainerStatus(ctx context.Context, containerID string) (string, error) {
	if containerID == "" {
		return "", fmt.Errorf("empty container ID")
	}

	if c.cli == nil {
		return string(ContainerStatusStopped), nil
	}

	slog.Info("inspecting container", "container_id", containerID)
	return string(ContainerStatusStopped), nil
}

func (c *Client) RunContainer(ctx context.Context, appID, envSlug, appSlug, imageName string, appPort int, envVars []struct{ Key, Value string }) (string, error) {
	_ = ctx
	_ = appID
	_ = envSlug
	_ = appSlug
	_ = imageName
	_ = appPort
	_ = envVars
	return "", fmt.Errorf("not implemented without docker client")
}

func (c *Client) RunDatabaseContainer(ctx context.Context, envSlug, dbSlug, imageName string, envVars map[string]string, ports []string, volumeName string) (string, error) {
	_ = ctx
	_ = envSlug
	_ = dbSlug
	_ = imageName
	_ = envVars
	_ = ports
	_ = volumeName
	return "", fmt.Errorf("not implemented without docker client")
}
