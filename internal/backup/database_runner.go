package backup

import (
	"compress/gzip"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
)

type EventNotifier interface {
	Dispatch(event models.NotificationEvent)
}

type DatabaseBackupRunner struct {
	dockerClient *docker.Client
	backupRepo   *repo.DatabaseBackupRepo
	storageRepo  *repo.StorageDestinationRepo
	dbRepo       *repo.DatabaseRepo
	projectRepo  *repo.ProjectRepo
	notifier     EventNotifier
	localBaseDir string
}

func NewDatabaseBackupRunner(
	dockerClient *docker.Client,
	backupRepo *repo.DatabaseBackupRepo,
	storageRepo *repo.StorageDestinationRepo,
	dbRepo *repo.DatabaseRepo,
	localBaseDir string,
) *DatabaseBackupRunner {
	if localBaseDir == "" {
		localBaseDir = "/var/lib/labuh/backups/databases"
	}
	_ = os.MkdirAll(localBaseDir, 0755)

	return &DatabaseBackupRunner{
		dockerClient: dockerClient,
		backupRepo:   backupRepo,
		storageRepo:  storageRepo,
		dbRepo:       dbRepo,
		localBaseDir: localBaseDir,
	}
}

func (r *DatabaseBackupRunner) SetNotifier(n EventNotifier, projectRepo *repo.ProjectRepo) {
	r.notifier = n
	r.projectRepo = projectRepo
}

func (r *DatabaseBackupRunner) RunBackup(ctx context.Context, databaseID string) (*models.DatabaseBackup, error) {
	db, err := r.dbRepo.GetByID(databaseID)
	if err != nil {
		return nil, fmt.Errorf("database not found: %w", err)
	}

	if db.ContainerID == nil || *db.ContainerID == "" {
		return nil, fmt.Errorf("database container is not running (no container ID)")
	}

	// 1. Dapatkan config backup
	cfg, _ := r.backupRepo.GetConfig(databaseID)

	var storageDest *models.StorageDestination
	if cfg != nil && cfg.StorageDestinationID != nil && *cfg.StorageDestinationID != "" {
		storageDest, _ = r.storageRepo.GetByID(*cfg.StorageDestinationID)
	}
	if storageDest == nil {
		// Fallback ke default storage jika ada
		storageDest, _ = r.storageRepo.GetDefault()
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	ext := "sql.gz"
	switch db.Engine {
	case models.DatabaseEngineRedis:
		ext = "rdb.gz"
	case models.DatabaseEngineMongoDB:
		ext = "archive.gz"
	}
	filename := fmt.Sprintf("%s-%s-%s.%s", db.Engine, db.Slug, timestamp, ext)

	// Pastikan direktori lokal ada
	dbDir := filepath.Join(r.localBaseDir, db.ID)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create local backup dir: %w", err)
	}
	localFilePath := filepath.Join(dbDir, filename)

	var storageDestID *string
	storageType := "local"
	if storageDest != nil {
		storageDestID = &storageDest.ID
		storageType = "s3"
	}

	// Record awal status pending
	backupRecord := &models.DatabaseBackup{
		DatabaseID:           db.ID,
		StorageDestinationID: storageDestID,
		Filename:             filename,
		StorageType:          storageType,
		Status:               "pending",
	}
	if err := r.backupRepo.CreateBackup(backupRecord); err != nil {
		return nil, fmt.Errorf("failed to create backup record: %w", err)
	}

	// 2. Siapkan command dump
	cmd, envs, err := getDumpCommand(db)
	if err != nil {
		errMsg := err.Error()
		_ = r.backupRepo.UpdateStatus(backupRecord.ID, "failed", 0, nil, &errMsg)
		return nil, err
	}

	// 3. Eksekusi dump streaming langsung ke gzip & file lokal
	outFile, err := os.Create(localFilePath)
	if err != nil {
		errMsg := fmt.Sprintf("cannot create local backup file: %v", err)
		_ = r.backupRepo.UpdateStatus(backupRecord.ID, "failed", 0, nil, &errMsg)
		return nil, err
	}

	gzWriter := gzip.NewWriter(outFile)
	execErr := r.dockerClient.ExecStream(ctx, *db.ContainerID, cmd, envs, gzWriter)
	_ = gzWriter.Close()
	_ = outFile.Close()

	if execErr != nil {
		_ = os.Remove(localFilePath)
		errMsg := fmt.Sprintf("dump execution failed: %v", execErr)
		_ = r.backupRepo.UpdateStatus(backupRecord.ID, "failed", 0, nil, &errMsg)
		return nil, execErr
	}

	// Cek ukuran file hasil dump
	fileInfo, err := os.Stat(localFilePath)
	if err != nil || fileInfo.Size() == 0 {
		_ = os.Remove(localFilePath)
		errMsg := "backup output is empty"
		_ = r.backupRepo.UpdateStatus(backupRecord.ID, "failed", 0, nil, &errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}
	fileSize := fileInfo.Size()

	var s3KeyPtr *string
	// 4. Jika storage destination S3 aktif, upload ke remote
	if storageDest != nil {
		s3Client, err := NewS3ClientFromDestination(storageDest)
		if err != nil {
			slog.Error("failed to initialize S3 client", "error", err)
		} else {
			prefix := strings.Trim(storageDest.Prefix, "/")
			if cfg != nil && cfg.CustomPrefix != nil && *cfg.CustomPrefix != "" {
				prefix = strings.Trim(*cfg.CustomPrefix, "/")
			}
			s3Key := fmt.Sprintf("%s/%s/%s", prefix, db.Slug, filename)

			f, err := os.Open(localFilePath)
			if err == nil {
				uploadErr := s3Client.PutObject(ctx, s3Key, f, fileSize, "application/gzip")
				_ = f.Close()
				if uploadErr != nil {
					slog.Error("failed to upload backup to S3", "error", uploadErr, "key", s3Key)
				} else {
					s3KeyPtr = &s3Key
				}
			}
		}
	}

	// 5. Update status sukses
	_ = r.backupRepo.UpdateStatus(backupRecord.ID, "completed", fileSize, s3KeyPtr, nil)
	backupRecord.Status = "completed"
	backupRecord.FileSize = fileSize
	backupRecord.S3Key = s3KeyPtr

	// 6. Jalankan Retention Pruning
	keepCount := 7
	if cfg != nil && cfg.RetentionCount > 0 {
		keepCount = cfg.RetentionCount
	}
	r.pruneOldBackups(ctx, db.ID, keepCount, storageDest)

	if r.notifier != nil {
		var projectID *string
		if r.projectRepo != nil {
			if env, envErr := r.projectRepo.GetEnvironmentByID(db.EnvironmentID); envErr == nil && env != nil {
				projectID = &env.ProjectID
			}
		}

		storageLabel := "Local Disk"
		if storageDest != nil {
			storageLabel = fmt.Sprintf("S3 (%s)", storageDest.Name)
		}

		r.notifier.Dispatch(models.NotificationEvent{
			Title:       fmt.Sprintf("Backup Database Sukses: %s", db.Name),
			Description: fmt.Sprintf("Backup untuk database %s berhasil dibuat dan disimpan ke %s.", db.Name, storageLabel),
			Status:      "success",
			EventType:   "backup_success",
			ProjectID:   projectID,
			Fields: map[string]string{
				"Database": db.Name,
				"Engine":   string(db.Engine),
				"Filename": filename,
				"Size":     fmt.Sprintf("%d KB", fileSize/1024),
				"Storage":  storageLabel,
			},
		})
	}

	return backupRecord, nil
}

func (r *DatabaseBackupRunner) pruneOldBackups(ctx context.Context, databaseID string, keepCount int, dest *models.StorageDestination) {
	oldBackups, err := r.backupRepo.GetOldBackupsForPruning(databaseID, keepCount)
	if err != nil {
		slog.Error("failed to query old backups for pruning", "error", err)
		return
	}

	var s3Client *S3Client
	if dest != nil {
		s3Client, _ = NewS3ClientFromDestination(dest)
	}

	for _, b := range oldBackups {
		// Hapus file lokal
		localPath := filepath.Join(r.localBaseDir, b.DatabaseID, b.Filename)
		_ = os.Remove(localPath)

		// Hapus remote S3 jika ada
		if b.S3Key != nil && s3Client != nil {
			_ = s3Client.DeleteObject(ctx, *b.S3Key)
		}

		// Hapus record dari database
		_ = r.backupRepo.Delete(b.ID)
		slog.Info("pruned old backup", "filename", b.Filename, "database_id", databaseID)
	}
}

func getDumpCommand(db *models.DatabaseService) ([]string, []string, error) {
	switch db.Engine {
	case models.DatabaseEnginePostgres:
		cmd := []string{"pg_dump", "-U", db.Username, "-d", db.DatabaseName}
		envs := []string{"PGPASSWORD=" + db.Password}
		return cmd, envs, nil

	case models.DatabaseEngineMySQL:
		cmd := []string{"mysqldump", "-u" + db.Username, "-p" + db.Password, db.DatabaseName}
		return cmd, nil, nil

	case models.DatabaseEngineMongoDB:
		cmd := []string{
			"mongodump",
			"--username", db.Username,
			"--password", db.Password,
			"--authenticationDatabase", "admin",
			"--db", db.DatabaseName,
			"--archive",
		}
		return cmd, nil, nil

	case models.DatabaseEngineRedis:
		cmd := []string{"redis-cli", "-a", db.Password, "--rdb", "/dev/stdout"}
		return cmd, nil, nil

	default:
		return nil, nil, fmt.Errorf("unsupported database engine for backup: %s", db.Engine)
	}
}
