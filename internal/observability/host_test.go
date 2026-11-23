package observability

import (
	stdtesting "testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetHostStats(t *stdtesting.T) {
	stats := GetHostStats("/")

	assert.Greater(t, stats.DiskTotal, uint64(0), "DiskTotal must be greater than 0")
	assert.Greater(t, stats.DiskFree, uint64(0), "DiskFree must be greater than 0")
	assert.GreaterOrEqual(t, stats.DiskUsedPercent, float64(0))
	assert.LessOrEqual(t, stats.DiskUsedPercent, float64(100))

	assert.Greater(t, stats.MemTotal, uint64(0), "MemTotal must be greater than 0")
	assert.GreaterOrEqual(t, stats.MemUsedPercent, float64(0))
	assert.LessOrEqual(t, stats.MemUsedPercent, float64(100))

	assert.Greater(t, stats.CPUNum, 0, "CPUNum must be at least 1")
}

func TestGetDiskStats_NonExistentPath(t *stdtesting.T) {
	_, _, _, _, err := GetDiskStats("/path/that/does/not/exist/at/all/12345")
	require.Error(t, err)
}
