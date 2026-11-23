package docker_test

import (
	"context"
	stdtesting "testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/docker"
)

func TestWaitForHealthCustom_ExitedContainerFailsFast(t *stdtesting.T) {
	client, err := docker.NewClient()
	require.NoError(t, err)

	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		t.Skip("Docker daemon not available, skipping test")
	}

	// Run container that exits immediately
	_ = client.RemoveContainer(ctx, "labuh-test-health-exited")
	cID, err := client.RunContainerWithCustomName(ctx, "test-app", "labuh-test-health-exited", "alpine:latest", 80, nil, docker.ContainerResourceLimits{})
	if err != nil {
		t.Skipf("Cannot run alpine container: %v", err)
	}
	defer func() {
		_ = client.RemoveContainer(ctx, cID)
	}()

	// Wait 1s for it to exit
	time.Sleep(1 * time.Second)

	// WaitForHealthCustom should fail fast because container is stopped
	start := time.Now()
	err = client.WaitForHealthCustom(ctx, cID, 80, "/", 10*time.Second)
	duration := time.Since(start)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exited")
	assert.Less(t, duration, 5*time.Second, "Should fail fast instead of waiting 10s")
}

func TestWaitForHealthCustom_HealthyContainer(t *stdtesting.T) {
	client, err := docker.NewClient()
	require.NoError(t, err)

	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		t.Skip("Docker daemon not available, skipping test")
	}

	_ = client.RemoveContainer(ctx, "labuh-test-health-ok")
	cID, err := client.RunContainerWithCustomName(ctx, "test-app", "labuh-test-health-ok", "nginx:alpine", 80, nil, docker.ContainerResourceLimits{})
	if err != nil {
		t.Skipf("Cannot run nginx container: %v", err)
	}
	defer func() {
		_ = client.RemoveContainer(ctx, cID)
	}()

	err = client.WaitForHealthCustom(ctx, cID, 80, "/", 15*time.Second)
	assert.NoError(t, err)
}
