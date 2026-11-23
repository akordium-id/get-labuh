package applications

import (
	"context"
	stdtesting "testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/models"
)

func TestAppBackupRepo(t *stdtesting.T) {
	db, err := database.Connect(":memory:")
	require.NoError(t, err)
	defer db.Close()

	err = database.RunAllMigrations(db)
	require.NoError(t, err)

	repo := NewRepo(db)
	ctx := context.Background()

	// 1. Create Application
	app, err := repo.CreateWithContext(ctx, models.CreateApplicationInput{
		EnvironmentID: "env-1",
		Name:          "App Backup Test",
		SourceType:    models.SourceTypeDockerImage,
		AppPort:       8080,
	})
	require.NoError(t, err)

	// 2. Test Backup Config (Upsert & Get)
	cfgInput := models.UpdateAppBackupConfigInput{
		ApplicationID:  app.ID,
		Enabled:        true,
		CronExpression: "0 4 * * *",
		RetentionCount: 5,
	}
	err = repo.UpsertBackupConfigWithContext(ctx, cfgInput)
	require.NoError(t, err)

	cfg, err := repo.GetBackupConfigWithContext(ctx, app.ID)
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, app.ID, cfg.ApplicationID)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, "0 4 * * *", cfg.CronExpression)
	assert.Equal(t, 5, cfg.RetentionCount)

	// Update existing config
	cfgInput.CronExpression = "0 2 * * *"
	cfgInput.RetentionCount = 10
	err = repo.UpsertBackupConfigWithContext(ctx, cfgInput)
	require.NoError(t, err)

	cfgUpdated, err := repo.GetBackupConfigWithContext(ctx, app.ID)
	require.NoError(t, err)
	assert.Equal(t, "0 2 * * *", cfgUpdated.CronExpression)
	assert.Equal(t, 10, cfgUpdated.RetentionCount)

	// 3. Test Create Backup
	backup1, err := repo.CreateBackupWithContext(ctx, models.CreateAppBackupInput{
		ApplicationID: app.ID,
		Filename:      "backup-1.tar.gz",
		StorageType:   "local",
		Status:        "pending",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, backup1.ID)
	assert.Equal(t, "backup-1.tar.gz", backup1.Filename)
	assert.Equal(t, "pending", backup1.Status)

	// 4. Test Update Backup Status
	s3Key := "backups/app/backup-1.tar.gz"
	err = repo.UpdateBackupStatusWithContext(ctx, backup1.ID, "completed", 2048, &s3Key, nil)
	require.NoError(t, err)

	gotBackup, err := repo.GetBackupByIDWithContext(ctx, backup1.ID)
	require.NoError(t, err)
	assert.Equal(t, "completed", gotBackup.Status)
	assert.Equal(t, int64(2048), gotBackup.FileSize)
	require.NotNil(t, gotBackup.S3Key)
	assert.Equal(t, s3Key, *gotBackup.S3Key)

	// 5. Test Multiple Backups & Pruning Query
	time.Sleep(10 * time.Millisecond)
	backup2, err := repo.CreateBackupWithContext(ctx, models.CreateAppBackupInput{
		ApplicationID: app.ID,
		Filename:      "backup-2.tar.gz",
		StorageType:   "local",
		Status:        "completed",
		FileSize:      4096,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, backup2.ID)

	time.Sleep(10 * time.Millisecond)
	backup3, err := repo.CreateBackupWithContext(ctx, models.CreateAppBackupInput{
		ApplicationID: app.ID,
		Filename:      "backup-3.tar.gz",
		StorageType:   "local",
		Status:        "completed",
		FileSize:      8192,
	})
	require.NoError(t, err)

	list, err := repo.ListBackupsByAppIDWithContext(ctx, app.ID)
	require.NoError(t, err)
	assert.Len(t, list, 3)
	// Must be ordered by created_at DESC (newest first)
	assert.Equal(t, backup3.ID, list[0].ID)

	// Pruning test: if keepCount is 2, backup1 should be returned as old backup
	oldBackups, err := repo.GetOldBackupsForPruning(ctx, app.ID, 2)
	require.NoError(t, err)
	require.Len(t, oldBackups, 1)
	assert.Equal(t, backup1.ID, oldBackups[0].ID)

	// 6. Test Delete Backup
	err = repo.DeleteBackupWithContext(ctx, backup1.ID)
	require.NoError(t, err)

	listAfterDelete, err := repo.ListBackupsByAppIDWithContext(ctx, app.ID)
	require.NoError(t, err)
	assert.Len(t, listAfterDelete, 2)
}
