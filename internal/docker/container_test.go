package docker

import (
	"bytes"
	"context"
	"io"
	"strings"
	stdtesting "testing"

	"github.com/docker/docker/client"
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

func TestCopyToContainer_RejectsCompressedArchives(t *stdtesting.T) {
	c := &Client{cli: &client.Client{}}

	tests := []struct {
		name    string
		payload []byte
		wantErr string
	}{
		{"gzip", []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00}, "compressed gzip archives are not allowed"},
		{"xz", []byte{0xfd, 0x37, 0x7a, 0x58, 0x5a, 0x00}, "compressed xz archives are not allowed"},
		{"bzip2", []byte{0x42, 0x5a, 0x68, 0x39}, "compressed bzip2 archives are not allowed"},
		{"zstd", []byte{0x28, 0xb5, 0x2f, 0xfd, 0x00}, "compressed zstd archives are not allowed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *stdtesting.T) {
			err := c.CopyToContainer(context.Background(), "dummy-container", "/", bytes.NewReader(tt.payload))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}

	t.Run("nil content", func(t *stdtesting.T) {
		err := c.CopyToContainer(context.Background(), "dummy-container", "/", nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "archive content cannot be nil")
	})

	t.Run("uncompressed tar stream passes inspection intact", func(t *stdtesting.T) {
		rawTar := []byte("ustar-test-payload")
		r, err := ensureUncompressedArchive(bytes.NewReader(rawTar))
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		assert.Equal(t, rawTar, got)
	})
}
