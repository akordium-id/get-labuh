package applications

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/akordium-id/get-labuh/internal/models"
)

// GetBackupConfigWithContext mengambil konfigurasi backup volume untuk sebuah aplikasi.
func (r *Repo) GetBackupConfigWithContext(ctx context.Context, appID string) (*models.AppBackupConfig, error) {
	query := `
		SELECT id, application_id, storage_destination_id, enabled, cron_expression, retention_count, custom_prefix, created_at, updated_at
		FROM app_backup_configs
		WHERE application_id = ?
	`
	var cfg models.AppBackupConfig
	err := r.db.QueryRowContext(ctx, query, appID).Scan(
		&cfg.ID,
		&cfg.ApplicationID,
		&cfg.StorageDestinationID,
		&cfg.Enabled,
		&cfg.CronExpression,
		&cfg.RetentionCount,
		&cfg.CustomPrefix,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get app backup config: %w", err)
	}
	return &cfg, nil
}

// UpsertBackupConfigWithContext menyimpan atau memperbarui konfigurasi backup volume aplikasi.
func (r *Repo) UpsertBackupConfigWithContext(ctx context.Context, input models.UpdateAppBackupConfigInput) error {
	now := time.Now().UTC()
	id := uuid.New().String()

	query := `
		INSERT INTO app_backup_configs (id, application_id, storage_destination_id, enabled, cron_expression, retention_count, custom_prefix, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(application_id) DO UPDATE SET
			storage_destination_id = excluded.storage_destination_id,
			enabled = excluded.enabled,
			cron_expression = excluded.cron_expression,
			retention_count = excluded.retention_count,
			custom_prefix = excluded.custom_prefix,
			updated_at = excluded.updated_at
	`
	_, err := r.db.ExecContext(ctx, query,
		id,
		input.ApplicationID,
		input.StorageDestinationID,
		input.Enabled,
		input.CronExpression,
		input.RetentionCount,
		input.CustomPrefix,
		now,
		now,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert app backup config: %w", err)
	}
	return nil
}

// CreateBackupWithContext mencatat snapshot backup baru ke database.
func (r *Repo) CreateBackupWithContext(ctx context.Context, input models.CreateAppBackupInput) (*models.AppBackup, error) {
	b := &models.AppBackup{
		ID:                   uuid.New().String(),
		ApplicationID:        input.ApplicationID,
		StorageDestinationID: input.StorageDestinationID,
		Filename:             input.Filename,
		FileSize:             input.FileSize,
		StorageType:          input.StorageType,
		S3Key:                input.S3Key,
		Status:               input.Status,
		ErrorMessage:         input.ErrorMessage,
		CreatedAt:            time.Now().UTC(),
	}

	query := `
		INSERT INTO app_backups (id, application_id, storage_destination_id, filename, file_size, storage_type, s3_key, status, error_message, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		b.ID,
		b.ApplicationID,
		b.StorageDestinationID,
		b.Filename,
		b.FileSize,
		b.StorageType,
		b.S3Key,
		b.Status,
		b.ErrorMessage,
		b.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create app backup record: %w", err)
	}
	return b, nil
}

// UpdateBackupStatusWithContext memperbarui status, ukuran file, s3 key, dan pesan error backup.
func (r *Repo) UpdateBackupStatusWithContext(ctx context.Context, id, status string, size int64, s3Key *string, errMsg *string) error {
	query := `
		UPDATE app_backups
		SET status = ?, file_size = ?, s3_key = ?, error_message = ?
		WHERE id = ?
	`
	_, err := r.db.ExecContext(ctx, query, status, size, s3Key, errMsg, id)
	if err != nil {
		return fmt.Errorf("failed to update app backup status: %w", err)
	}
	return nil
}

// GetBackupByIDWithContext mengambil record backup berdasarkan ID.
func (r *Repo) GetBackupByIDWithContext(ctx context.Context, id string) (*models.AppBackup, error) {
	query := `
		SELECT id, application_id, storage_destination_id, filename, file_size, storage_type, s3_key, status, error_message, created_at
		FROM app_backups
		WHERE id = ?
	`
	var b models.AppBackup
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&b.ID,
		&b.ApplicationID,
		&b.StorageDestinationID,
		&b.Filename,
		&b.FileSize,
		&b.StorageType,
		&b.S3Key,
		&b.Status,
		&b.ErrorMessage,
		&b.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("app backup not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query app backup: %w", err)
	}
	return &b, nil
}

// ListBackupsByAppIDWithContext mengambil seluruh riwayat backup volume aplikasi (terurut terbaru).
func (r *Repo) ListBackupsByAppIDWithContext(ctx context.Context, appID string) ([]*models.AppBackup, error) {
	query := `
		SELECT id, application_id, storage_destination_id, filename, file_size, storage_type, s3_key, status, error_message, created_at
		FROM app_backups
		WHERE application_id = ?
		ORDER BY created_at DESC
	`
	rows, err := r.db.QueryContext(ctx, query, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to query app backups: %w", err)
	}
	defer rows.Close()

	var backups []*models.AppBackup
	for rows.Next() {
		var b models.AppBackup
		if err := rows.Scan(
			&b.ID,
			&b.ApplicationID,
			&b.StorageDestinationID,
			&b.Filename,
			&b.FileSize,
			&b.StorageType,
			&b.S3Key,
			&b.Status,
			&b.ErrorMessage,
			&b.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan app backup: %w", err)
		}
		backups = append(backups, &b)
	}
	return backups, nil
}

// DeleteBackupWithContext menghapus record backup dari database.
func (r *Repo) DeleteBackupWithContext(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM app_backups WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete app backup: %w", err)
	}
	return nil
}

// GetOldBackupsForPruning mengembalikan list backup sukses yang melebihi batas keepCount.
func (r *Repo) GetOldBackupsForPruning(ctx context.Context, appID string, keepCount int) ([]*models.AppBackup, error) {
	query := `
		SELECT id, application_id, storage_destination_id, filename, file_size, storage_type, s3_key, status, error_message, created_at
		FROM app_backups
		WHERE application_id = ? AND status = 'completed'
		ORDER BY created_at DESC
		LIMIT -1 OFFSET ?
	`
	rows, err := r.db.QueryContext(ctx, query, appID, keepCount)
	if err != nil {
		return nil, fmt.Errorf("failed to query old app backups for pruning: %w", err)
	}
	defer rows.Close()

	var oldBackups []*models.AppBackup
	for rows.Next() {
		var b models.AppBackup
		if err := rows.Scan(
			&b.ID,
			&b.ApplicationID,
			&b.StorageDestinationID,
			&b.Filename,
			&b.FileSize,
			&b.StorageType,
			&b.S3Key,
			&b.Status,
			&b.ErrorMessage,
			&b.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan old app backup: %w", err)
		}
		oldBackups = append(oldBackups, &b)
	}
	return oldBackups, nil
}
