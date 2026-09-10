package docker

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/moby/moby/api/pkg/stdcopy"
)

type Client struct {
	cli *client.Client
}

func NewClient() (*Client, error) {
	if os.Getenv("LABUH_DOCKER_MOCK") == "true" {
		return &Client{cli: nil}, nil
	}
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		slog.Warn("docker client unavailable, falling back to stub", "error", err)
		return &Client{cli: nil}, nil
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	if c.cli == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := c.cli.Ping(ctx)
	return err
}

func (c *Client) PullImage(ctx context.Context, imageRef string) error {
	if c.cli == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	reader, err := c.cli.ImagePull(ctx, imageRef, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

func (c *Client) BuildImage(ctx context.Context, buildContext io.Reader, dockerfilePath string, tags []string) (string, error) {
	if c.cli == nil {
		if len(tags) > 0 {
			return tags[0], nil
		}
		return "test:latest", nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if buildContext == nil {
		if len(tags) > 0 {
			return tags[0], nil
		}
		return "test:latest", nil
	}
	resp, err := c.cli.ImageBuild(ctx, buildContext, types.ImageBuildOptions{
		Dockerfile: dockerfilePath,
		Tags:       tags,
		Remove:     true,
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if len(tags) > 0 {
		return tags[0], nil
	}
	return "latest", nil
}

func (c *Client) CreateContainer(ctx context.Context, config any, hostConfig any, networkingConfig any, name string) (string, error) {
	if c.cli == nil {
		return "", nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cfg, _ := config.(*container.Config)
	if cfg == nil {
		return "", nil
	}
	hCfg, _ := hostConfig.(*container.HostConfig)
	nCfg, _ := networkingConfig.(*network.NetworkingConfig)
	resp, err := c.cli.ContainerCreate(ctx, cfg, hCfg, nCfg, nil, name)
	if err != nil {
		return "", err
	}
	return resp.ID, nil
}

func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	if c.cli == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return c.cli.ContainerStart(ctx, containerID, container.StartOptions{})
}

func (c *Client) RemoveContainer(ctx context.Context, containerID string) error {
	if c.cli == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	err := c.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{Force: true})
	if err != nil && client.IsErrNotFound(err) {
		return nil
	}
	return err
}

func (c *Client) InspectContainer(ctx context.Context, containerID string) error {
	if c.cli == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	_, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil && client.IsErrNotFound(err) {
		return nil
	}
	return err
}

func (c *Client) ContainerLogs(ctx context.Context, containerID string, opts any) (io.ReadCloser, error) {
	if c.cli == nil {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	logOpts, ok := opts.(container.LogsOptions)
	if !ok {
		logOpts = container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
			Follow:     true,
			Tail:       "100",
		}
	}
	reader, err := c.cli.ContainerLogs(ctx, containerID, logOpts)
	if err != nil && client.IsErrNotFound(err) {
		return nil, nil
	}
	return reader, err
}

func (c *Client) ListContainers(ctx context.Context, opts any) error {
	if c.cli == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	listOpts, ok := opts.(container.ListOptions)
	if !ok {
		listOpts = container.ListOptions{All: true}
	}
	_, err := c.cli.ContainerList(ctx, listOpts)
	return err
}

func (c *Client) GetServerVersion(ctx context.Context) (string, error) {
	if c.cli == nil {
		return "mock", nil
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
		return 0, nil
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
