package settings

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/docker"
)

func TestCleanupScheduler_RunCleanup(t *testing.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, database.RunAllMigrations(db))

	settingRepo := NewSettingRepo(db)
	auditRepo := NewAuditLogRepo(db)
	dockerClient, err := docker.NewClient()
	require.NoError(t, err)

	scheduler := NewCleanupScheduler(dockerClient, settingRepo, auditRepo)
	require.NotNil(t, scheduler)

	ctx := context.Background()
	report, err := scheduler.RunCleanup(ctx)
	require.NoError(t, err)
	require.NotNil(t, report)

	lastRun, err := settingRepo.Get(ctx, "docker_cleanup_last_run")
	require.NoError(t, err)
	assert.NotEmpty(t, lastRun)

	lastReclaimed, err := settingRepo.Get(ctx, "docker_cleanup_last_reclaimed")
	require.NoError(t, err)
	assert.NotEmpty(t, lastReclaimed)

	auditLogs, err := auditRepo.GetAll(ctx, 10)
	require.NoError(t, err)
	require.NotEmpty(t, auditLogs)
	assert.Equal(t, "docker_cleanup", auditLogs[0].Action)
}
