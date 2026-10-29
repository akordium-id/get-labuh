package repo

import (
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/modules/databases"
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

	if err = rows.Err(); err != nil {
		return nil, err
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

// DatabaseBackupRepo adalah type alias ke internal/modules/databases untuk kompatibilitas ke belakang.
type DatabaseBackupRepo = databases.BackupRepo

func NewDatabaseBackupRepo(db *sql.DB) *DatabaseBackupRepo {
	return databases.NewBackupRepo(db)
}
