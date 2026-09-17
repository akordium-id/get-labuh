package docker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type Client struct {
	cli *client.Client
}

func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		slog.Warn("failed to initialize docker client from env, falling back to dormant client", "error", err)
		return &Client{cli: nil}, nil
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	if c.cli == nil {
		return fmt.Errorf("docker client not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := c.cli.Ping(ctx)
	return err
}

func (c *Client) PullImage(ctx context.Context, imageRef string) error {
	if c.cli == nil {
		return fmt.Errorf("docker client not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	slog.Info("pulling docker image", "image", imageRef)
	reader, err := c.cli.ImagePull(ctx, imageRef, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image error: %w", err)
	}
	defer reader.Close()

	// Drain reader to ensure pull completes
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

func (c *Client) BuildImage(ctx context.Context, buildContext io.Reader, dockerfilePath string, tags []string) (string, error) {
	if c.cli == nil {
		return "", fmt.Errorf("docker client not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if dockerfilePath == "" {
		dockerfilePath = "Dockerfile"
	}

	opts := types.ImageBuildOptions{
		Dockerfile: dockerfilePath,
		Tags:       tags,
		Remove:     true,
	}

	resp, err := c.cli.ImageBuild(ctx, buildContext, opts)
	if err != nil {
		return "", fmt.Errorf("image build error: %w", err)
	}
	defer resp.Body.Close()

	// Drain output
	_, _ = io.Copy(io.Discard, resp.Body)

	if len(tags) > 0 {
		return tags[0], nil
	}
	return "", nil
}

func (c *Client) CreateContainer(ctx context.Context, cfg *container.Config, hostConfig *container.HostConfig, netConfig *network.NetworkingConfig, name string) (string, error) {
	if c.cli == nil {
		return "", fmt.Errorf("docker client not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	resp, err := c.cli.ContainerCreate(ctx, cfg, hostConfig, netConfig, nil, name)
	if err != nil {
		return "", fmt.Errorf("container create error: %w", err)
	}
	return resp.ID, nil
}

func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	if c.cli == nil {
		return fmt.Errorf("docker client not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return c.cli.ContainerStart(ctx, containerID, container.StartOptions{})
}

func (c *Client) RemoveContainer(ctx context.Context, containerID string) error {
	if c.cli == nil {
		return fmt.Errorf("docker client not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return c.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
}

func (c *Client) InspectContainer(ctx context.Context, containerID string) (*types.ContainerJSON, error) {
	if c.cli == nil {
		return nil, fmt.Errorf("docker client not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	insp, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, err
	}
	return &insp, nil
}

func (c *Client) ContainerLogs(ctx context.Context, containerID string, opts container.LogsOptions) (io.ReadCloser, error) {
	if c.cli == nil {
		return nil, fmt.Errorf("docker client not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return c.cli.ContainerLogs(ctx, containerID, opts)
}

func (c *Client) ListContainers(ctx context.Context, opts container.ListOptions) ([]types.Container, error) {
	if c.cli == nil {
		return nil, fmt.Errorf("docker client not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return c.cli.ContainerList(ctx, opts)
}

func (c *Client) EnsureNetwork(ctx context.Context, networkName string) error {
	if c.cli == nil {
		return fmt.Errorf("docker client not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(networkName) == "" {
		networkName = "labuh-network"
	}

	nets, err := c.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list networks: %w", err)
	}

	for _, n := range nets {
		if n.Name == networkName {
			return nil
		}
	}

	_, err = c.cli.NetworkCreate(ctx, networkName, network.CreateOptions{
		Driver: "bridge",
	})
	if err != nil {
		return fmt.Errorf("failed to create network %s: %w", networkName, err)
	}
	return nil
}

func (c *Client) GetServerVersion(ctx context.Context) (string, error) {
	if c.cli == nil {
		return "", fmt.Errorf("docker client not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ver, err := c.cli.ServerVersion(ctx)
	if err != nil {
		return "", err
	}
	return ver.Version, nil
}

func (c *Client) CountRunningContainers(ctx context.Context) (int, error) {
	if c.cli == nil {
		return 0, fmt.Errorf("docker client not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return 0, err
	}
	return len(containers), nil
}

func DemultiplexLogs(dst io.Writer, src io.Reader) error {
	_, err := stdcopy.StdCopy(dst, dst, src)
	return err
}

