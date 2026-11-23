package docker

import (
	"bytes"
	"context"
	"strings"
	stdtesting "testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecStreamWithStdin(t *stdtesting.T) {
	client, err := NewClient()
	require.NoError(t, err)

	ctx := context.Background()
	if err := client.Ping(ctx); err != nil {
		t.Skip("Docker daemon not available, skipping live exec test")
	}

	// 1. Jalankan container nginx:alpine sementara (running foreground service)
	_ = client.RemoveContainer(ctx, "labuh-test-exec-stdin")
	cID, err := client.RunContainerWithCustomName(ctx, "test-app", "labuh-test-exec-stdin", "nginx:alpine", 80, nil, ContainerResourceLimits{})
	if err != nil {
		t.Skipf("Cannot run test container: %v", err)
	}
	defer func() {
		_ = client.RemoveContainer(ctx, cID)
	}()

	// 2. Test ExecStreamWithStdin dengan perintah 'cat'
	inputData := "Hello from ExecStreamWithStdin test!\nLine 2 of piped data\n"
	stdin := strings.NewReader(inputData)
	var stdout bytes.Buffer

	cmd := []string{"cat"}
	err = client.ExecStreamWithStdin(ctx, cID, cmd, nil, stdin, &stdout)
	require.NoError(t, err)
	assert.Equal(t, inputData, stdout.String())
}
