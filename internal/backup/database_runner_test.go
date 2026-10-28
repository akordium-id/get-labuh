package backup

import (
	"time"
	stdtesting "testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
)

func TestGetDumpCommand(t *stdtesting.T) {
	// 1. Postgres
	pg := &models.DatabaseService{
		Engine:       models.DatabaseEnginePostgres,
		Username:     "postgres",
		Password:     "secretpass",
		DatabaseName: "production",
	}
	cmd, envs, err := getDumpCommand(pg)
	require.NoError(t, err)
	assert.Equal(t, []string{"pg_dump", "-U", "postgres", "-d", "production"}, cmd)
	assert.Equal(t, []string{"PGPASSWORD=secretpass"}, envs)

	// 2. MySQL
	my := &models.DatabaseService{
		Engine:       models.DatabaseEngineMySQL,
		Username:     "root",
		Password:     "dbpass",
		DatabaseName: "shop",
	}
	cmd, envs, err = getDumpCommand(my)
	require.NoError(t, err)
	assert.Equal(t, []string{"mysqldump", "-uroot", "-pdbpass", "shop"}, cmd)
	assert.Nil(t, envs)

	// 3. Mongo
	mg := &models.DatabaseService{
		Engine:       models.DatabaseEngineMongoDB,
		Username:     "admin",
		Password:     "mongopass",
		DatabaseName: "analytics",
	}
	cmd, _, err = getDumpCommand(mg)
	require.NoError(t, err)
	assert.Equal(t, []string{"mongodump", "--username", "admin", "--password", "mongopass", "--authenticationDatabase", "admin", "--db", "analytics", "--archive"}, cmd)

	// 4. Redis
	rd := &models.DatabaseService{
		Engine:   models.DatabaseEngineRedis,
		Password: "redispassword",
	}
	cmd, _, err = getDumpCommand(rd)
	require.NoError(t, err)
	assert.Equal(t, []string{"redis-cli", "-a", "redispassword", "--rdb", "/dev/stdout"}, cmd)
}

func TestScheduler_ShouldRunNow(t *stdtesting.T) {
	dailyTime := time.Date(2026, 10, 28, 2, 0, 0, 0, time.UTC)
	assert.True(t, shouldRunNow("0 2 * * *", dailyTime))
	assert.True(t, shouldRunNow("@daily", dailyTime))

	otherTime := time.Date(2026, 10, 28, 5, 30, 0, 0, time.UTC)
	assert.False(t, shouldRunNow("0 2 * * *", otherTime))

	hourlyTime := time.Date(2026, 10, 28, 14, 0, 0, 0, time.UTC)
	assert.True(t, shouldRunNow("@hourly", hourlyTime))
}

func TestDatabaseBackupRepo_RetentionPruning(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	t.Cleanup(func() { _ = db.Close() })

	projectRepo := repo.NewProjectRepo(db)
	dbRepo := repo.NewDatabaseRepo(db)
	backupRepo := repo.NewDatabaseBackupRepo(db)

	proj, err := projectRepo.Create(models.CreateProjectInput{Name: "DB Project", Slug: "db-proj"})
	require.NoError(t, err)
	env, err := projectRepo.CreateEnvironment(proj.ID, "Production", "prod")
	require.NoError(t, err)

	dbService, err := dbRepo.Create(models.CreateDatabaseServiceInput{
		EnvironmentID: env.ID,
		Name:          "Primary Postgres",
		Slug:          "primary-pg",
		Engine:        models.DatabaseEnginePostgres,
		Username:      "postgres",
		Password:      "pass",
		DatabaseName:  "appdb",
		Port:          5432,
	})
	require.NoError(t, err)

	// Buat 5 backup berstatus completed
	for i := 1; i <= 5; i++ {
		b := &models.DatabaseBackup{
			DatabaseID:  dbService.ID,
			Filename:    "backup-" + string(rune('0'+i)) + ".sql.gz",
			FileSize:    1024,
			StorageType: "local",
			Status:      "completed",
		}
		require.NoError(t, backupRepo.CreateBackup(b))
		time.Sleep(10 * time.Millisecond)
	}

	// Jika keepCount = 3, maka 2 backup terlama harus masuk list pruning
	pruneList, err := backupRepo.GetOldBackupsForPruning(dbService.ID, 3)
	require.NoError(t, err)
	assert.Len(t, pruneList, 2)
}
