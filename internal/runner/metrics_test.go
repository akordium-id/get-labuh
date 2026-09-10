package runner

import (
	"context"
	"testing"

	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsCollector(t *testing.T) {
	dockerClient, err := docker.NewClient()
	require.NoError(t, err)

	collector := NewMetricsCollector(dockerClient)
	require.NotNil(t, collector)

	hb := collector.Collect(context.Background())
	assert.True(t, hb.Timestamp > 0)
	assert.True(t, hb.MemoryTotalMB > 0)
	assert.True(t, hb.DiskUsedPercent >= 0)
}
