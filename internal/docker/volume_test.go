package docker

import (
	"os"
	stdtesting "testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/models"
)

func TestPrepareFileMounts(t *stdtesting.T) {
	tempDir := t.TempDir()
	appID := "test-app-1"

	mounts := []*models.AppFileMount{
		{
			ID:            "fm-1",
			ApplicationID: appID,
			MountPath:     "/etc/nginx/nginx.conf",
			Content:       "worker_processes auto;",
			FileMode:      "0644",
			IsReadOnly:    true,
		},
		{
			ID:            "fm-2",
			ApplicationID: appID,
			MountPath:     "/app/config.json",
			Content:       `{"env":"production"}`,
			FileMode:      "0600",
			IsReadOnly:    false,
		},
	}

	bindings, err := PrepareFileMounts(tempDir, appID, mounts)
	require.NoError(t, err)
	require.Len(t, bindings, 2)

	// Verify first mount
	assert.Equal(t, "/etc/nginx/nginx.conf", bindings[0].Target)
	assert.True(t, bindings[0].IsReadOnly)
	content1, err := os.ReadFile(bindings[0].Source)
	require.NoError(t, err)
	assert.Equal(t, "worker_processes auto;", string(content1))

	info1, err := os.Stat(bindings[0].Source)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info1.Mode().Perm())

	// Verify second mount
	assert.Equal(t, "/app/config.json", bindings[1].Target)
	assert.False(t, bindings[1].IsReadOnly)
	content2, err := os.ReadFile(bindings[1].Source)
	require.NoError(t, err)
	assert.Equal(t, `{"env":"production"}`, string(content2))

	info2, err := os.Stat(bindings[1].Source)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info2.Mode().Perm())
}

func TestVolumeBindingToBindString(t *stdtesting.T) {
	b1 := VolumeBinding{
		Source:     "my-vol",
		Target:     "/app/storage",
		IsReadOnly: false,
	}
	assert.Equal(t, "my-vol:/app/storage:rw", b1.ToBindString())

	b2 := VolumeBinding{
		Source:     "/var/data/custom.conf",
		Target:     "/etc/custom.conf",
		IsReadOnly: true,
	}
	assert.Equal(t, "/var/data/custom.conf:/etc/custom.conf:ro", b2.ToBindString())
}
