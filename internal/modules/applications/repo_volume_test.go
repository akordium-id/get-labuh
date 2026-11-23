package applications

import (
	"context"
	stdtesting "testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/models"
)

func TestVolumeRepo(t *stdtesting.T) {
	db, err := database.Connect(":memory:")
	require.NoError(t, err)
	defer db.Close()

	err = database.RunAllMigrations(db)
	require.NoError(t, err)

	repo := NewRepo(db)
	ctx := context.Background()

	// 1. Create Application first
	app, err := repo.CreateWithContext(ctx, models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "App Storage Test",
		SourceType:    models.SourceTypeDockerImage,
		AppPort:       8080,
	})
	require.NoError(t, err)

	// 2. Test Volumes
	volInput := models.CreateAppVolumeInput{
		ApplicationID: app.ID,
		Name:          "uploads",
		VolumeType:    models.VolumeTypeNamed,
		MountPath:     "/app/uploads",
		IsReadOnly:    false,
	}
	vol, err := repo.CreateVolume(ctx, volInput)
	require.NoError(t, err)
	assert.NotEmpty(t, vol.ID)
	assert.Equal(t, "uploads", vol.Name)
	assert.Equal(t, models.VolumeTypeNamed, vol.VolumeType)
	assert.Equal(t, "/app/uploads", vol.MountPath)

	vols, err := repo.GetVolumesByAppID(ctx, app.ID)
	require.NoError(t, err)
	assert.Len(t, vols, 1)

	gotVol, err := repo.GetVolumeByID(ctx, vol.ID)
	require.NoError(t, err)
	assert.Equal(t, vol.ID, gotVol.ID)

	err = repo.DeleteVolume(ctx, vol.ID)
	require.NoError(t, err)

	volsAfterDelete, err := repo.GetVolumesByAppID(ctx, app.ID)
	require.NoError(t, err)
	assert.Len(t, volsAfterDelete, 0)

	// 3. Test File Mounts
	fmInput := models.CreateAppFileMountInput{
		ApplicationID: app.ID,
		MountPath:     "/etc/nginx/nginx.conf",
		Content:       "events {}",
		FileMode:      "0644",
		IsReadOnly:    true,
	}
	fm, err := repo.CreateFileMount(ctx, fmInput)
	require.NoError(t, err)
	assert.NotEmpty(t, fm.ID)
	assert.Equal(t, "/etc/nginx/nginx.conf", fm.MountPath)
	assert.Equal(t, "events {}", fm.Content)

	fms, err := repo.GetFileMountsByAppID(ctx, app.ID)
	require.NoError(t, err)
	assert.Len(t, fms, 1)

	gotFm, err := repo.GetFileMountByID(ctx, fm.ID)
	require.NoError(t, err)
	assert.Equal(t, fm.ID, gotFm.ID)

	err = repo.UpdateFileMount(ctx, fm.ID, "events { worker_connections 1024; }")
	require.NoError(t, err)

	gotFmUpdated, err := repo.GetFileMountByID(ctx, fm.ID)
	require.NoError(t, err)
	assert.Equal(t, "events { worker_connections 1024; }", gotFmUpdated.Content)

	err = repo.DeleteFileMount(ctx, fm.ID)
	require.NoError(t, err)

	fmsAfterDelete, err := repo.GetFileMountsByAppID(ctx, app.ID)
	require.NoError(t, err)
	assert.Len(t, fmsAfterDelete, 0)
}
