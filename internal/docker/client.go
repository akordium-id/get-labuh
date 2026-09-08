package docker

import (
	"context"
	"io"

	"github.com/moby/moby/api/pkg/stdcopy"
)

type Client struct {
	cli interface{}
}

func NewClient() (*Client, error) {
	return &Client{cli: nil}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	return nil
}

func (c *Client) PullImage(ctx context.Context, imageRef string) error {
	return nil
}

func (c *Client) BuildImage(ctx context.Context, buildContext io.Reader, dockerfilePath string, tags []string) (string, error) {
	return "", nil
}

func (c *Client) CreateContainer(ctx context.Context, config interface{}, hostConfig interface{}, networkingConfig interface{}, name string) (string, error) {
	return "", nil
}

func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	return nil
}

func (c *Client) StopContainer(ctx context.Context, containerID string) error {
	return nil
}

func (c *Client) RemoveContainer(ctx context.Context, containerID string) error {
	return nil
}

func (c *Client) InspectContainer(ctx context.Context, containerID string) error {
	return nil
}

func (c *Client) ContainerLogs(ctx context.Context, containerID string, opts interface{}) (io.ReadCloser, error) {
	return nil, nil
}

func (c *Client) ListContainers(ctx context.Context, opts interface{}) error {
	return nil
}

func DemultiplexLogs(dst io.Writer, src io.Reader) error {
	_, err := stdcopy.StdCopy(dst, dst, src)
	return err
}
