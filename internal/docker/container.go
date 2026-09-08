package docker

import (
	"context"
	"fmt"
)

func (c *Client) GenerateContainerName(envSlug, appSlug, appID string) string {
	shortID := appID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	return fmt.Sprintf("labuh-%s-%s-%s", envSlug, appSlug, shortID)
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
