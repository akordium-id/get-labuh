package settings

import (
	"context"
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

// SettingRepo mengelola konfigurasi key-value sistem (Caddy, labuh_mode, dll)
type SettingRepo struct {
	db *sql.DB
}

func NewSettingRepo(db *sql.DB) *SettingRepo {
	return &SettingRepo{db: db}
}

func (r *SettingRepo) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (r *SettingRepo) Set(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO settings (key, value, created_at, updated_at) VALUES (?, ?, ?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at",
		key, value, time.Now(), time.Now(),
	)
	return err
}

func (r *SettingRepo) InitializeDefaults(ctx context.Context) error {
	defaults := map[string]string{
		"caddy_api_url": "http://localhost:2019",
		"caddy_api_key": "",
		"caddy_network": "labuh-network",
	}

	for key, value := range defaults {
		if _, err := r.Get(ctx, key); err == sql.ErrNoRows {
			if err := r.Set(ctx, key, value); err != nil {
				return err
			}
		}
	}

	return nil
}

// AuditLogRepo mengelola catatan audit aktivitas pengguna
type AuditLogRepo struct {
	db *sql.DB
}

func NewAuditLogRepo(db *sql.DB) *AuditLogRepo {
	return &AuditLogRepo{db: db}
}

func (r *AuditLogRepo) Create(ctx context.Context, input models.CreateAuditLogInput) (*models.AuditLog, error) {
	log := &models.AuditLog{
		ID:         uuid.New().String(),
		UserID:     input.UserID,
		Action:     input.Action,
		Resource:   input.Resource,
		ResourceID: input.ResourceID,
		IPAddress:  input.IPAddress,
		UserAgent:  input.UserAgent,
		CreatedAt:  time.Now(),
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_logs (id, user_id, action, resource, resource_id, ip_address, user_agent, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		log.ID, log.UserID, log.Action, log.Resource, log.ResourceID, log.IPAddress, log.UserAgent, log.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return log, nil
}

func (r *AuditLogRepo) GetAll(ctx context.Context, limit int) ([]*models.AuditLog, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, action, resource, resource_id, ip_address, user_agent, created_at
		 FROM audit_logs ORDER BY created_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*models.AuditLog
	for rows.Next() {
		log := &models.AuditLog{}
		err := rows.Scan(&log.ID, &log.UserID, &log.Action, &log.Resource, &log.ResourceID, &log.IPAddress, &log.UserAgent, &log.CreatedAt)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return logs, nil
}

// StorageDestinationRepo mengelola destinasi backup S3/R2/MinIO
type StorageDestinationRepo struct {
	db *sql.DB
}

func NewStorageDestinationRepo(db *sql.DB) *StorageDestinationRepo {
	return &StorageDestinationRepo{db: db}
}

func (r *StorageDestinationRepo) Create(ctx context.Context, input models.CreateStorageDestinationInput) (*models.StorageDestination, error) {
	id := uuid.New().String()
	now := time.Now()

	if input.IsDefault {
		_, _ = r.db.ExecContext(ctx, "UPDATE storage_destinations SET is_default = 0")
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO storage_destinations (id, name, is_default, endpoint, bucket, region, access_key_id, secret_access_key, prefix, use_ssl, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, input.Name, input.IsDefault, input.Endpoint, input.Bucket, input.Region, input.AccessKeyID, input.SecretAccessKey, input.Prefix, input.UseSSL, now, now,
	)
	if err != nil {
		return nil, err
	}

	return r.GetByID(ctx, id)
}

func (r *StorageDestinationRepo) GetByID(ctx context.Context, id string) (*models.StorageDestination, error) {
	dest := &models.StorageDestination{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, is_default, endpoint, bucket, region, access_key_id, secret_access_key, prefix, use_ssl, created_at, updated_at
		 FROM storage_destinations WHERE id = ?`,
		id,
	).Scan(&dest.ID, &dest.Name, &dest.IsDefault, &dest.Endpoint, &dest.Bucket, &dest.Region, &dest.AccessKeyID, &dest.SecretAccessKey, &dest.Prefix, &dest.UseSSL, &dest.CreatedAt, &dest.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return dest, nil
}

func (r *StorageDestinationRepo) List(ctx context.Context) ([]*models.StorageDestination, error) {
	rows, err := r.db.QueryContext(ctx,
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

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *StorageDestinationRepo) SetDefault(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "UPDATE storage_destinations SET is_default = 0"); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE storage_destinations SET is_default = 1, updated_at = ? WHERE id = ?", time.Now(), id); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *StorageDestinationRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM storage_destinations WHERE id = ?", id)
	return err
}
