package docker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
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

	if timeout <= 0 {
		timeout = 10
	}

	stopTimeout := timeout
	return c.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &stopTimeout})
}

func (c *Client) RestartContainer(ctx context.Context, containerID string, timeout int) error {
	if containerID == "" {
		return fmt.Errorf("empty container ID")
	}

	if c.cli == nil {
		return nil
	}

	restartTimeout := timeout
	if restartTimeout <= 0 {
		restartTimeout = 10
	}

	return c.cli.ContainerRestart(ctx, containerID, container.StopOptions{Timeout: &restartTimeout})
}

func (c *Client) IsContainerRunning(ctx context.Context, containerID string) (bool, error) {
	if containerID == "" {
		return false, fmt.Errorf("empty container ID")
	}

	if c.cli == nil {
		slog.Warn("docker client not initialized, assuming not running", "container_id", containerID)
		return false, nil
	}

	inspect, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, err
	}

	return inspect.State != nil && inspect.State.Running, nil
}

func (c *Client) GetContainerStatus(ctx context.Context, containerID string) (string, error) {
	if containerID == "" {
		return "", fmt.Errorf("empty container ID")
	}

	if c.cli == nil {
		return string(ContainerStatusStopped), nil
	}

	inspect, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return string(ContainerStatusStopped), err
	}

	if inspect.State != nil {
		if inspect.State.Running {
			return string(ContainerStatusRunning), nil
		}
		if inspect.State.Dead || inspect.State.OOMKilled {
			return string(ContainerStatusFailed), nil
		}
	}
	return string(ContainerStatusStopped), nil
}

func (c *Client) RunContainer(ctx context.Context, appID, envSlug, appSlug, imageName string, appPort int, envVars []struct{ Key, Value string }) (string, error) {
	if c.cli == nil {
		slog.Warn("docker client unavailable, returning simulated container ID", "app_id", appID)
		return fmt.Sprintf("sim-container-%s", appID), nil
	}

	if ctx == nil {
		ctx = context.Background()
	}

	networkName := "labuh-network"
	networks, err := c.cli.NetworkList(ctx, network.ListOptions{})
	if err == nil {
		found := false
		for _, netItem := range networks {
			if netItem.Name == networkName {
				found = true
				break
			}
		}
		if !found {
			_, _ = c.cli.NetworkCreate(ctx, networkName, network.CreateOptions{Driver: "bridge"})
		}
	}

	var envList []string
	for _, ev := range envVars {
		envList = append(envList, fmt.Sprintf("%s=%s", ev.Key, ev.Value))
	}

	containerName := c.GenerateContainerName(envSlug, appSlug, appID)
	_ = c.cli.ContainerRemove(ctx, containerName, container.RemoveOptions{Force: true})

	portStr := fmt.Sprintf("%d/tcp", appPort)
	cfg := &container.Config{
		Image: imageName,
		Env:   envList,
		ExposedPorts: nat.PortSet{
			nat.Port(portStr): struct{}{},
		},
	}
	hostCfg := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
	}
	netCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			networkName: {},
		},
	}

	resp, err := c.cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	if err := c.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	return resp.ID, nil
}

func (c *Client) RunDatabaseContainer(ctx context.Context, envSlug, dbSlug, imageName string, envVars map[string]string, ports []string, volumeName string) (string, error) {
	if c.cli == nil {
		return fmt.Sprintf("sim-db-%s", dbSlug), nil
	}

	if ctx == nil {
		ctx = context.Background()
	}

	var envList []string
	for k, v := range envVars {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}

	containerName := fmt.Sprintf("labuh-db-%s-%s", envSlug, dbSlug)
	_ = c.cli.ContainerRemove(ctx, containerName, container.RemoveOptions{Force: true})

	cfg := &container.Config{
		Image: imageName,
		Env:   envList,
	}
	hostCfg := &container.HostConfig{
		RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
	}
	netCfg := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"labuh-network": {},
		},
	}

	resp, err := c.cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create db container: %w", err)
	}

	if err := c.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("failed to start db container: %w", err)
	}

	return resp.ID, nil
}
