package docker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{15728640, "15.00 MB"},
		{1073741824, "1.00 GB"},
		{3758096384, "3.50 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, FormatBytes(tt.input))
		})
	}
}

func TestPruneSystem_DormantClient(t *testing.T) {
	client := &Client{cli: nil}
	report, err := client.PruneSystem(context.Background(), DefaultPruneOptions())
	require.Error(t, err)
	assert.Nil(t, report)
}

func TestPruneSystem_LiveOrMock(t *testing.T) {
	client, err := NewClient()
	require.NoError(t, err)
	if client.cli == nil {
		t.Skip("Docker client is dormant, skipping live prune test")
	}

	report, err := client.PruneSystem(context.Background(), DefaultPruneOptions())
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.NotEmpty(t, report.SpaceFormatted)
}
