package repo

import (
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

type StorageDestinationRepo struct {
	db *sql.DB
}

func NewStorageDestinationRepo(db *sql.DB) *StorageDestinationRepo {
	return &StorageDestinationRepo{db: db}
}

func (r *StorageDestinationRepo) Create(input models.CreateStorageDestinationInput) (*models.StorageDestination, error) {
	id := uuid.New().String()
	now := time.Now()

	if input.IsDefault {
		_, _ = r.db.Exec("UPDATE storage_destinations SET is_default = 0")
	}

	_, err := r.db.Exec(
		`INSERT INTO storage_destinations (id, name, is_default, endpoint, bucket, region, access_key_id, secret_access_key, prefix, use_ssl, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, input.Name, input.IsDefault, input.Endpoint, input.Bucket, input.Region, input.AccessKeyID, input.SecretAccessKey, input.Prefix, input.UseSSL, now, now,
	)
	if err != nil {
		return nil, err
	}

	return r.GetByID(id)
}

func (r *StorageDestinationRepo) GetByID(id string) (*models.StorageDestination, error) {
	dest := &models.StorageDestination{}
	err := r.db.QueryRow(
		`SELECT id, name, is_default, endpoint, bucket, region, access_key_id, secret_access_key, prefix, use_ssl, created_at, updated_at
		 FROM storage_destinations WHERE id = ?`,
		id,
	).Scan(&dest.ID, &dest.Name, &dest.IsDefault, &dest.Endpoint, &dest.Bucket, &dest.Region, &dest.AccessKeyID, &dest.SecretAccessKey, &dest.Prefix, &dest.UseSSL, &dest.CreatedAt, &dest.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return dest, nil
}

func (r *StorageDestinationRepo) GetDefault() (*models.StorageDestination, error) {
	dest := &models.StorageDestination{}
	err := r.db.QueryRow(
		`SELECT id, name, is_default, endpoint, bucket, region, access_key_id, secret_access_key, prefix, use_ssl, created_at, updated_at
		 FROM storage_destinations WHERE is_default = 1 LIMIT 1`,
	).Scan(&dest.ID, &dest.Name, &dest.IsDefault, &dest.Endpoint, &dest.Bucket, &dest.Region, &dest.AccessKeyID, &dest.SecretAccessKey, &dest.Prefix, &dest.UseSSL, &dest.CreatedAt, &dest.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return dest, nil
}

func (r *StorageDestinationRepo) List() ([]*models.StorageDestination, error) {
	rows, err := r.db.Query(
		`SELECT id, name, is_default, endpoint, bucket, region, access_key_id, secret_access_key, prefix, use_ssl, created_at, updated_at
		 FROM storage_destinations ORDER BY is_default DESC, name ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.StorageDestination
	for rows.Next() {
		dest := &models.StorageDestination{}
		if err := rows.Scan(&dest.ID, &dest.Name, &dest.IsDefault, &dest.Endpoint, &dest.Bucket, &dest.Region, &dest.AccessKeyID, &dest.SecretAccessKey, &dest.Prefix, &dest.UseSSL, &dest.CreatedAt, &dest.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, dest)
	}
	return list, nil
}

func (r *StorageDestinationRepo) SetDefault(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("UPDATE storage_destinations SET is_default = 0"); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE storage_destinations SET is_default = 1, updated_at = ? WHERE id = ?", time.Now(), id); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *StorageDestinationRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM storage_destinations WHERE id = ?", id)
	return err
}

type DatabaseBackupRepo struct {
	db *sql.DB
}

func NewDatabaseBackupRepo(db *sql.DB) *DatabaseBackupRepo {
	return &DatabaseBackupRepo{db: db}
}

func (r *DatabaseBackupRepo) GetConfig(databaseID string) (*models.DatabaseBackupConfig, error) {
	cfg := &models.DatabaseBackupConfig{}
	err := r.db.QueryRow(
		`SELECT id, database_id, storage_destination_id, enabled, cron_expression, retention_count, custom_prefix, created_at, updated_at
		 FROM database_backup_configs WHERE database_id = ?`,
		databaseID,
	).Scan(&cfg.ID, &cfg.DatabaseID, &cfg.StorageDestinationID, &cfg.Enabled, &cfg.CronExpression, &cfg.RetentionCount, &cfg.CustomPrefix, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (r *DatabaseBackupRepo) UpsertConfig(cfg *models.DatabaseBackupConfig) error {
	now := time.Now()
	if cfg.ID == "" {
		cfg.ID = uuid.New().String()
	}
	if cfg.RetentionCount <= 0 {
		cfg.RetentionCount = 7
	}
	if cfg.CronExpression == "" {
		cfg.CronExpression = "0 2 * * *"
	}

	_, err := r.db.Exec(
		`INSERT INTO database_backup_configs (id, database_id, storage_destination_id, enabled, cron_expression, retention_count, custom_prefix, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(database_id) DO UPDATE SET
		 storage_destination_id = excluded.storage_destination_id,
		 enabled = excluded.enabled,
		 cron_expression = excluded.cron_expression,
		 retention_count = excluded.retention_count,
		 custom_prefix = excluded.custom_prefix,
		 updated_at = excluded.updated_at`,
		cfg.ID, cfg.DatabaseID, cfg.StorageDestinationID, cfg.Enabled, cfg.CronExpression, cfg.RetentionCount, cfg.CustomPrefix, now, now,
	)
	return err
}

func (r *DatabaseBackupRepo) ListActiveConfigs() ([]*models.DatabaseBackupConfig, error) {
	rows, err := r.db.Query(
		`SELECT id, database_id, storage_destination_id, enabled, cron_expression, retention_count, custom_prefix, created_at, updated_at
		 FROM database_backup_configs WHERE enabled = 1`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.DatabaseBackupConfig
	for rows.Next() {
		cfg := &models.DatabaseBackupConfig{}
		if err := rows.Scan(&cfg.ID, &cfg.DatabaseID, &cfg.StorageDestinationID, &cfg.Enabled, &cfg.CronExpression, &cfg.RetentionCount, &cfg.CustomPrefix, &cfg.CreatedAt, &cfg.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, cfg)
	}
	return list, nil
}

func (r *DatabaseBackupRepo) CreateBackup(backup *models.DatabaseBackup) error {
	if backup.ID == "" {
		backup.ID = uuid.New().String()
	}
	backup.CreatedAt = time.Now()

	_, err := r.db.Exec(
		`INSERT INTO database_backups (id, database_id, storage_destination_id, filename, file_size, storage_type, s3_key, status, error_message, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		backup.ID, backup.DatabaseID, backup.StorageDestinationID, backup.Filename, backup.FileSize, backup.StorageType, backup.S3Key, backup.Status, backup.ErrorMessage, backup.CreatedAt,
	)
	return err
}

func (r *DatabaseBackupRepo) UpdateStatus(id, status string, fileSize int64, s3Key *string, errMsg *string) error {
	_, err := r.db.Exec(
		`UPDATE database_backups SET status = ?, file_size = ?, s3_key = ?, error_message = ? WHERE id = ?`,
		status, fileSize, s3Key, errMsg, id,
	)
	return err
}

func (r *DatabaseBackupRepo) GetByID(id string) (*models.DatabaseBackup, error) {
	b := &models.DatabaseBackup{}
	err := r.db.QueryRow(
		`SELECT id, database_id, storage_destination_id, filename, file_size, storage_type, s3_key, status, error_message, created_at
		 FROM database_backups WHERE id = ?`,
		id,
	).Scan(&b.ID, &b.DatabaseID, &b.StorageDestinationID, &b.Filename, &b.FileSize, &b.StorageType, &b.S3Key, &b.Status, &b.ErrorMessage, &b.CreatedAt)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (r *DatabaseBackupRepo) ListByDatabaseID(databaseID string) ([]*models.DatabaseBackup, error) {
	rows, err := r.db.Query(
		`SELECT id, database_id, storage_destination_id, filename, file_size, storage_type, s3_key, status, error_message, created_at
		 FROM database_backups WHERE database_id = ? ORDER BY created_at DESC`,
		databaseID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.DatabaseBackup
	for rows.Next() {
		b := &models.DatabaseBackup{}
		if err := rows.Scan(&b.ID, &b.DatabaseID, &b.StorageDestinationID, &b.Filename, &b.FileSize, &b.StorageType, &b.S3Key, &b.Status, &b.ErrorMessage, &b.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, nil
}

func (r *DatabaseBackupRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM database_backups WHERE id = ?", id)
	return err
}

// GetOldBackupsForPruning mengembalikan backup sukses yang melebihi jumlah retentionCount
func (r *DatabaseBackupRepo) GetOldBackupsForPruning(databaseID string, keepCount int) ([]*models.DatabaseBackup, error) {
	if keepCount <= 0 {
		keepCount = 7
	}

	rows, err := r.db.Query(
		`SELECT id, database_id, storage_destination_id, filename, file_size, storage_type, s3_key, status, error_message, created_at
		 FROM database_backups
		 WHERE database_id = ? AND status = 'completed'
		 ORDER BY created_at DESC
		 LIMIT -1 OFFSET ?`,
		databaseID, keepCount,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.DatabaseBackup
	for rows.Next() {
		b := &models.DatabaseBackup{}
		if err := rows.Scan(&b.ID, &b.DatabaseID, &b.StorageDestinationID, &b.Filename, &b.FileSize, &b.StorageType, &b.S3Key, &b.Status, &b.ErrorMessage, &b.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, b)
	}
	return list, nil
}
