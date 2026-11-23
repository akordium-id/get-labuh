package backup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
)

type AppRepository interface {
	GetByIDWithContext(ctx context.Context, id string) (*models.Application, error)
	GetVolumesByAppID(ctx context.Context, appID string) ([]*models.AppVolume, error)
	GetFileMountsByAppID(ctx context.Context, appID string) ([]*models.AppFileMount, error)
	GetBackupConfigWithContext(ctx context.Context, appID string) (*models.AppBackupConfig, error)
	CreateBackupWithContext(ctx context.Context, input models.CreateAppBackupInput) (*models.AppBackup, error)
	UpdateBackupStatusWithContext(ctx context.Context, id, status string, size int64, s3Key *string, errMsg *string) error
	GetBackupByIDWithContext(ctx context.Context, id string) (*models.AppBackup, error)
	ListBackupsByAppIDWithContext(ctx context.Context, appID string) ([]*models.AppBackup, error)
	DeleteBackupWithContext(ctx context.Context, id string) error
	GetOldBackupsForPruning(ctx context.Context, appID string, keepCount int) ([]*models.AppBackup, error)
}

type AppVolumeBackupRunner struct {
	dockerClient *docker.Client
	appRepo      AppRepository
	storageRepo  *repo.StorageDestinationRepo
	projectRepo  *repo.ProjectRepo
	notifier     EventNotifier
	localBaseDir string
}

func NewAppVolumeBackupRunner(
	dockerClient *docker.Client,
	appRepo AppRepository,
	storageRepo *repo.StorageDestinationRepo,
	localBaseDir string,
) *AppVolumeBackupRunner {
	if localBaseDir == "" {
		if os.Geteuid() == 0 {
			localBaseDir = "/var/lib/labuh/backups/apps"
		} else {
			homeDir, err := os.UserHomeDir()
			if err == nil && homeDir != "" {
				localBaseDir = filepath.Join(homeDir, ".labuh", "backups", "apps")
			} else {
				localBaseDir = "./.labuh/backups/apps"
			}
		}
	}
	_ = os.MkdirAll(localBaseDir, 0755)

	return &AppVolumeBackupRunner{
		dockerClient: dockerClient,
		appRepo:      appRepo,
		storageRepo:  storageRepo,
		localBaseDir: localBaseDir,
	}
}

func (r *AppVolumeBackupRunner) SetNotifier(n EventNotifier, projectRepo *repo.ProjectRepo) {
	r.notifier = n
	r.projectRepo = projectRepo
}

func (r *AppVolumeBackupRunner) RunBackup(ctx context.Context, appID string) (*models.AppBackup, error) {
	app, err := r.appRepo.GetByIDWithContext(ctx, appID)
	if err != nil {
		return nil, fmt.Errorf("application not found: %w", err)
	}

	if app.ContainerID == nil || *app.ContainerID == "" {
		return nil, fmt.Errorf("application container is not running (no container ID)")
	}

	volumes, _ := r.appRepo.GetVolumesByAppID(ctx, appID)
	fileMounts, _ := r.appRepo.GetFileMountsByAppID(ctx, appID)
	if len(volumes) == 0 && len(fileMounts) == 0 {
		return nil, fmt.Errorf("no persistent volumes or file mounts configured for this application")
	}

	cfg, _ := r.appRepo.GetBackupConfigWithContext(ctx, appID)
	var storageDest *models.StorageDestination
	if cfg != nil && cfg.StorageDestinationID != nil && *cfg.StorageDestinationID != "" && r.storageRepo != nil {
		storageDest, _ = r.storageRepo.GetByID(*cfg.StorageDestinationID)
	}
	if storageDest == nil && r.storageRepo != nil {
		storageDest, _ = r.storageRepo.GetDefault()
	}

	timestamp := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("app-volume-%s-%s.tar.gz", app.Slug, timestamp)

	appDir := filepath.Join(r.localBaseDir, app.ID)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		return nil, fmt.Errorf("cannot create local backup dir: %w", err)
	}
	localFilePath := filepath.Join(appDir, filename)

	var storageDestID *string
	storageType := "local"
	if storageDest != nil {
		storageDestID = &storageDest.ID
		storageType = "s3"
	}

	backupRecord, err := r.appRepo.CreateBackupWithContext(ctx, models.CreateAppBackupInput{
		ApplicationID:        app.ID,
		StorageDestinationID: storageDestID,
		Filename:             filename,
		StorageType:          storageType,
		Status:               "pending",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create backup record: %w", err)
	}

	outFile, err := os.Create(localFilePath)
	if err != nil {
		errMsg := err.Error()
		_ = r.appRepo.UpdateBackupStatusWithContext(ctx, backupRecord.ID, "failed", 0, nil, &errMsg)
		return nil, err
	}

	gw := gzip.NewWriter(outFile)
	tw := tar.NewWriter(gw)

	var mountPaths []string
	for _, v := range volumes {
		mountPaths = append(mountPaths, v.MountPath)
	}
	for _, fm := range fileMounts {
		mountPaths = append(mountPaths, fm.MountPath)
	}

	var archiveErr error
	archivedFilesCount := 0

	for _, mp := range mountPaths {
		if r.dockerClient == nil {
			archiveErr = fmt.Errorf("docker client not available")
			break
		}
		stream, copyErr := r.dockerClient.CopyFromContainer(ctx, *app.ContainerID, mp)
		if copyErr != nil {
			slog.Warn("cannot copy path from container, skipping", "path", mp, "error", copyErr)
			continue
		}
		subTr := tar.NewReader(stream)
		for {
			hdr, tErr := subTr.Next()
			if tErr == io.EOF {
				break
			}
			if tErr != nil {
				slog.Warn("tar stream read error", "error", tErr)
				break
			}
			parentDir := strings.TrimPrefix(filepath.Dir(filepath.Clean(mp)), "/")
			var entryName string
			if parentDir == "." || parentDir == "" {
				entryName = strings.TrimPrefix(hdr.Name, "./")
			} else {
				entryName = parentDir + "/" + strings.TrimPrefix(hdr.Name, "./")
			}
			hdr.Name = filepath.Clean(entryName)
			if err := tw.WriteHeader(hdr); err != nil {
				continue
			}
			archivedFilesCount++
			if hdr.Size > 0 {
				_, _ = io.Copy(tw, subTr)
			}
		}
		_ = stream.Close()
	}

	_ = tw.Close()
	_ = gw.Close()
	_ = outFile.Close()

	if archivedFilesCount == 0 {
		_ = os.Remove(localFilePath)
		errMsg := "no volume data could be archived from container"
		_ = r.appRepo.UpdateBackupStatusWithContext(ctx, backupRecord.ID, "failed", 0, nil, &errMsg)
		if r.notifier != nil {
			r.notifier.Dispatch(models.NotificationEvent{
				Title:       fmt.Sprintf("Backup Volume Gagal: %s", app.Name),
				Description: fmt.Sprintf("Gagal membuat snapshot volume untuk %s: tidak ada data volume/file mount yang ditemukan.", app.Name),
				Status:      "failed",
				EventType:   "backup_failed",
			})
		}
		return nil, errors.New(errMsg)
	}

	fileInfo, err := os.Stat(localFilePath)
	if err != nil || fileInfo.Size() == 0 {
		_ = os.Remove(localFilePath)
		errMsg := "backup output is empty"
		if archiveErr != nil {
			errMsg = archiveErr.Error()
		}
		_ = r.appRepo.UpdateBackupStatusWithContext(ctx, backupRecord.ID, "failed", 0, nil, &errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}
	fileSize := fileInfo.Size()

	var s3KeyPtr *string
	if storageDest != nil {
		s3Client, err := NewS3ClientFromDestination(storageDest)
		if err == nil {
			prefix := strings.Trim(storageDest.Prefix, "/")
			if cfg != nil && cfg.CustomPrefix != nil && *cfg.CustomPrefix != "" {
				prefix = strings.Trim(*cfg.CustomPrefix, "/")
			}
			s3Key := fmt.Sprintf("%s/%s/%s", prefix, app.Slug, filename)
			f, oErr := os.Open(localFilePath)
			if oErr == nil {
				if uErr := s3Client.PutObject(ctx, s3Key, f, fileSize, "application/gzip"); uErr != nil {
					slog.Error("failed to upload app volume backup to S3", "s3_key", s3Key, "error", uErr)
				} else {
					s3KeyPtr = &s3Key
				}
				_ = f.Close()
			}
		}
	}

	_ = r.appRepo.UpdateBackupStatusWithContext(ctx, backupRecord.ID, "completed", fileSize, s3KeyPtr, nil)
	backupRecord.Status = "completed"
	backupRecord.FileSize = fileSize
	backupRecord.S3Key = s3KeyPtr

	keepCount := 7
	if cfg != nil && cfg.RetentionCount > 0 {
		keepCount = cfg.RetentionCount
	}
	r.pruneOldBackups(ctx, app.ID, keepCount, storageDest)

	if r.notifier != nil {
		storageLabel := "Local Disk"
		if storageDest != nil {
			storageLabel = fmt.Sprintf("S3 (%s)", storageDest.Name)
		}
		r.notifier.Dispatch(models.NotificationEvent{
			Title:       fmt.Sprintf("Backup Volume Aplikasi Sukses: %s", app.Name),
			Description: fmt.Sprintf("Backup volume untuk aplikasi %s berhasil dibuat dan disimpan ke %s.", app.Name, storageLabel),
			Status:      "success",
			EventType:   "app_volume_backup_success",
			Fields: map[string]string{
				"Application": app.Name,
				"Filename":    filename,
				"Size":        fmt.Sprintf("%d KB", fileSize/1024),
				"Storage":     storageLabel,
			},
		})
	}

	return backupRecord, nil
}

func (r *AppVolumeBackupRunner) RestoreAppVolume(ctx context.Context, backupID string) error {
	backupRecord, err := r.appRepo.GetBackupByIDWithContext(ctx, backupID)
	if err != nil {
		return fmt.Errorf("backup record not found: %w", err)
	}

	app, err := r.appRepo.GetByIDWithContext(ctx, backupRecord.ApplicationID)
	if err != nil {
		return fmt.Errorf("application not found: %w", err)
	}
	if app.ContainerID == nil || *app.ContainerID == "" {
		return fmt.Errorf("application container is not running (no container ID)")
	}

	localPath := filepath.Join(r.localBaseDir, backupRecord.ApplicationID, backupRecord.Filename)
	var rawReader io.ReadCloser

	if f, err := os.Open(localPath); err == nil {
		rawReader = f
	} else if backupRecord.S3Key != nil && backupRecord.StorageDestinationID != nil && r.storageRepo != nil {
		dest, err := r.storageRepo.GetByID(*backupRecord.StorageDestinationID)
		if err != nil {
			return fmt.Errorf("storage destination not found: %w", err)
		}
		s3Client, err := NewS3ClientFromDestination(dest)
		if err != nil {
			return fmt.Errorf("failed to init S3 client for volume restore: %w", err)
		}
		s3Stream, err := s3Client.GetObject(ctx, *backupRecord.S3Key)
		if err != nil {
			return fmt.Errorf("failed to download volume backup from S3: %w", err)
		}
		rawReader = s3Stream
	} else {
		return fmt.Errorf("backup file %s not found on disk or S3", backupRecord.Filename)
	}
	defer rawReader.Close()

	if r.dockerClient == nil {
		return fmt.Errorf("docker client not available")
	}

	// Safe Restore: Stop container sementara
	_ = r.dockerClient.StopContainer(ctx, *app.ContainerID, 10)
	// Garansi pemulihan: Container selalu dinyalakan kembali menggunakan detached context agar tidak gagal saat caller context canceled/timeout
	defer func() {
		restartCtx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		if startErr := r.dockerClient.StartContainer(restartCtx, *app.ContainerID); startErr != nil {
			slog.Error("CRITICAL: failed to restart container after volume restore",
				"app_id", app.ID,
				"container_id", *app.ContainerID,
				"error", startErr)
		}
	}()

	gzReader, err := gzip.NewReader(rawReader)
	if err != nil {
		return fmt.Errorf("failed to initialize gzip reader: %w", err)
	}
	defer gzReader.Close()

	err = r.dockerClient.CopyToContainer(ctx, *app.ContainerID, "/", gzReader)
	if err != nil {
		if r.notifier != nil {
			r.notifier.Dispatch(models.NotificationEvent{
				Title:       fmt.Sprintf("Restore Volume Aplikasi Gagal: %s", app.Name),
				Description: fmt.Sprintf("Gagal mengekstrak volume backup %s ke container: %v", backupRecord.Filename, err),
				Status:      "failed",
				EventType:   "app_volume_restore_failed",
			})
		}
		return fmt.Errorf("failed to extract volume archive into container: %w", err)
	}

	if r.notifier != nil {
		r.notifier.Dispatch(models.NotificationEvent{
			Title:       fmt.Sprintf("Restore Volume Aplikasi Sukses: %s", app.Name),
			Description: fmt.Sprintf("Berhasil me-restore volume snapshot %s ke aplikasi %s.", backupRecord.Filename, app.Name),
			Status:      "success",
			EventType:   "app_volume_restore_success",
		})
	}

	return nil
}

func (r *AppVolumeBackupRunner) pruneOldBackups(ctx context.Context, appID string, keepCount int, dest *models.StorageDestination) {
	oldBackups, err := r.appRepo.GetOldBackupsForPruning(ctx, appID, keepCount)
	if err != nil {
		slog.Error("failed to query old app backups for pruning", "error", err)
		return
	}

	var s3Client *S3Client
	if dest != nil {
		s3Client, _ = NewS3ClientFromDestination(dest)
	}

	for _, b := range oldBackups {
		localPath := filepath.Join(r.localBaseDir, b.ApplicationID, b.Filename)
		_ = os.Remove(localPath)

		if b.S3Key != nil && s3Client != nil {
			_ = s3Client.DeleteObject(ctx, *b.S3Key)
		}

		_ = r.appRepo.DeleteBackupWithContext(ctx, b.ID)
		slog.Info("pruned old app volume backup", "filename", b.Filename, "app_id", appID)
	}
}
