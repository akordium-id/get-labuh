package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
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
	containerName := c.GenerateContainerName(envSlug, appSlug, appID)
	return c.RunContainerWithCustomName(ctx, appID, containerName, imageName, appPort, envVars)
}

func (c *Client) RunContainerWithCustomName(ctx context.Context, appID, containerName, imageName string, appPort int, envVars []struct{ Key, Value string }) (string, error) {
	if c.cli == nil {
		return "", fmt.Errorf("docker client not initialized")
	}

	networkName := "labuh-network"
	if err := c.EnsureNetwork(ctx, networkName); err != nil {
		slog.Warn("failed to ensure labuh network, proceeding with default", "error", err)
	}

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

	// Remove old container with exact same name if present
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

func (c *Client) GetContainerIP(ctx context.Context, containerID, networkName string) (string, error) {
	insp, err := c.InspectContainer(ctx, containerID)
	if err != nil {
		return "", err
	}

	if insp.NetworkSettings != nil {
		if networkName != "" {
			if netSettings, exists := insp.NetworkSettings.Networks[networkName]; exists && netSettings.IPAddress != "" {
				return netSettings.IPAddress, nil
			}
		}
		if insp.NetworkSettings.IPAddress != "" {
			return insp.NetworkSettings.IPAddress, nil
		}
	}

	return "", fmt.Errorf("no IP address found for container %s", containerID)
}

func (c *Client) WaitForHealth(ctx context.Context, containerID string, appPort int, maxDuration time.Duration) error {
	if maxDuration <= 0 {
		maxDuration = 30 * time.Second
	}

	deadline := time.Now().Add(maxDuration)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	httpClient := &http.Client{
		Timeout: 2 * time.Second,
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("healthcheck timed out after %v", maxDuration)
			}

			running, err := c.IsContainerRunning(ctx, containerID)
			if err != nil || !running {
				return fmt.Errorf("container stopped or exited unexpectedly: %v", err)
			}

			// Try to connect to container IP if port is provided
			if appPort > 0 {
				ip, err := c.GetContainerIP(ctx, containerID, "labuh-network")
				if err == nil && ip != "" {
					portsToTry := []int{appPort}

					// Inspect container to check actual exposed ports (e.g. 80 for nginx)
					if insp, inspErr := c.InspectContainer(ctx, containerID); inspErr == nil && insp != nil && insp.Config != nil {
						for p := range insp.Config.ExposedPorts {
							if p.Int() > 0 && p.Int() != appPort {
								portsToTry = append(portsToTry, p.Int())
							}
						}
					}

					for _, checkPort := range portsToTry {
						// 1. Try HTTP request
						url := fmt.Sprintf("http://%s:%d/", ip, checkPort)
						req, reqErr := http.NewRequestWithContext(ctx, "GET", url, nil)
						if reqErr == nil {
							resp, httpErr := httpClient.Do(req)
							if httpErr == nil {
								resp.Body.Close()
								// Any status code under 500 implies the app server is up
								if resp.StatusCode < 500 {
									return nil
								}
							}
						}

						// 2. Fallback to TCP dial
						conn, tcpErr := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", ip, checkPort), 1*time.Second)
						if tcpErr == nil {
							_ = conn.Close()
							return nil
						}
					}
				}
			}

			// If container has been continuously running for at least 3 seconds without exiting, consider healthy
			if insp, inspErr := c.InspectContainer(ctx, containerID); inspErr == nil && insp != nil && insp.State != nil && insp.State.Running {
				if startedAt, err := time.Parse(time.RFC3339Nano, insp.State.StartedAt); err == nil {
					if time.Since(startedAt) >= 3*time.Second {
						return nil
					}
				}
			}
		}
	}
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

	// Engine-specific data directory inside the container
	dataDir := "/var/lib/data"
	if strings.Contains(imageName, "postgres") {
		dataDir = "/var/lib/postgresql/data"
	} else if strings.Contains(imageName, "mysql") {
		dataDir = "/var/lib/mysql"
	} else if strings.Contains(imageName, "redis") {
		dataDir = "/data"
	} else if strings.Contains(imageName, "mongo") {
		dataDir = "/data/db"
	}

	if volumeName != "" {
		hostCfg.Binds = []string{fmt.Sprintf("%s:%s", volumeName, dataDir)}
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

// ExecStream menjalankan command di dalam container dan mengalirkan stdout secara langsung ke writer
func (c *Client) ExecStream(ctx context.Context, containerID string, cmd []string, env []string, stdout io.Writer) error {
	if c.cli == nil {
		return fmt.Errorf("docker client not initialized")
	}

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		Env:          env,
		AttachStdout: true,
		AttachStderr: true,
	}

	execCreateResp, err := c.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec instance: %w", err)
	}

	attachResp, err := c.cli.ContainerExecAttach(ctx, execCreateResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return fmt.Errorf("failed to attach exec instance: %w", err)
	}
	defer attachResp.Close()

	var stderrBuf bytes.Buffer
	_, err = stdcopy.StdCopy(stdout, &stderrBuf, attachResp.Reader)
	if err != nil {
		return fmt.Errorf("failed during exec stream copy: %w", err)
	}

	inspectResp, err := c.cli.ContainerExecInspect(ctx, execCreateResp.ID)
	if err != nil {
		return fmt.Errorf("failed to inspect exec exit code: %w", err)
	}

	if inspectResp.ExitCode != 0 {
		return fmt.Errorf("command exited with code %d: %s", inspectResp.ExitCode, strings.TrimSpace(stderrBuf.String()))
	}

	return nil
}

// ExecTTY membuat sesi exec interaktif dengan PTY untuk terminal web WebSocket
func (c *Client) ExecTTY(ctx context.Context, containerID string, cmd []string, env []string) (string, types.HijackedResponse, error) {
	if c.cli == nil {
		return "", types.HijackedResponse{}, fmt.Errorf("docker client not initialized")
	}

	execConfig := container.ExecOptions{
		Cmd:          cmd,
		Env:          env,
		Tty:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
	}

	execCreateResp, err := c.cli.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		return "", types.HijackedResponse{}, fmt.Errorf("failed to create tty exec: %w", err)
	}

	attachResp, err := c.cli.ContainerExecAttach(ctx, execCreateResp.ID, container.ExecAttachOptions{
		Tty: true,
	})
	if err != nil {
		return "", types.HijackedResponse{}, fmt.Errorf("failed to attach tty exec: %w", err)
	}

	return execCreateResp.ID, attachResp, nil
}

// ResizeExec mengubah ukuran terminal PTY pada container exec
func (c *Client) ResizeExec(ctx context.Context, execID string, height, width uint) error {
	if c.cli == nil {
		return fmt.Errorf("docker client not initialized")
	}
	return c.cli.ContainerExecResize(ctx, execID, container.ResizeOptions{
		Height: height,
		Width:  width,
	})
}
