package backup

import (
	"context"
	stdtesting "testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	apprepo "github.com/akordium-id/get-labuh/internal/modules/applications"
	"github.com/akordium-id/get-labuh/internal/models"
)

func TestAppVolumeRunner_EmptyVolumesError(t *stdtesting.T) {
	db, err := database.Connect(":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, database.RunAllMigrations(db))

	ar := apprepo.NewRepo(db)
	sr := repo.NewStorageDestinationRepo(db)

	runner := NewAppVolumeBackupRunner(nil, ar, sr, t.TempDir())

	ctx := context.Background()
	app, err := ar.CreateWithContext(ctx, models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "App Without Volumes",
		SourceType:    models.SourceTypeDockerImage,
		AppPort:       80,
	})
	require.NoError(t, err)

	// App with no volumes and no container should return error
	_, err = runner.RunBackup(ctx, app.ID)
	assert.Error(t, err)
}

func TestAppVolumeRunner_RestoreNotFound(t *stdtesting.T) {
	db, err := database.Connect(":memory:")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, database.RunAllMigrations(db))

	ar := apprepo.NewRepo(db)
	sr := repo.NewStorageDestinationRepo(db)
	runner := NewAppVolumeBackupRunner(nil, ar, sr, t.TempDir())

	ctx := context.Background()
	err = runner.RestoreAppVolume(ctx, "non-existent-backup-id")
	assert.Error(t, err)
}
