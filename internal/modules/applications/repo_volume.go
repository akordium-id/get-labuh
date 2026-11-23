package applications

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/akordium-id/get-labuh/internal/models"
)

// CreateVolume menambahkan volume persisten baru untuk aplikasi.
func (r *Repo) CreateVolume(ctx context.Context, input models.CreateAppVolumeInput) (*models.AppVolume, error) {
	vol := &models.AppVolume{
		ID:            uuid.New().String(),
		ApplicationID: input.ApplicationID,
		Name:          input.Name,
		VolumeType:    input.VolumeType,
		HostPath:      input.HostPath,
		MountPath:     input.MountPath,
		IsReadOnly:    input.IsReadOnly,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	query := `
		INSERT INTO app_volumes (id, application_id, name, volume_type, host_path, mount_path, is_read_only, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		vol.ID,
		vol.ApplicationID,
		vol.Name,
		vol.VolumeType,
		vol.HostPath,
		vol.MountPath,
		vol.IsReadOnly,
		vol.CreatedAt,
		vol.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create volume: %w", err)
	}

	return vol, nil
}

// GetVolumesByAppID mengambil seluruh volume milik aplikasi.
func (r *Repo) GetVolumesByAppID(ctx context.Context, appID string) ([]*models.AppVolume, error) {
	query := `
		SELECT id, application_id, name, volume_type, host_path, mount_path, is_read_only, created_at, updated_at
		FROM app_volumes
		WHERE application_id = ?
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to query volumes: %w", err)
	}
	defer rows.Close()

	var volumes []*models.AppVolume
	for rows.Next() {
		vol := &models.AppVolume{}
		err := rows.Scan(
			&vol.ID,
			&vol.ApplicationID,
			&vol.Name,
			&vol.VolumeType,
			&vol.HostPath,
			&vol.MountPath,
			&vol.IsReadOnly,
			&vol.CreatedAt,
			&vol.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan volume: %w", err)
		}
		volumes = append(volumes, vol)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return volumes, nil
}

// GetVolumeByID mengambil satu volume berdasarkan ID.
func (r *Repo) GetVolumeByID(ctx context.Context, id string) (*models.AppVolume, error) {
	query := `
		SELECT id, application_id, name, volume_type, host_path, mount_path, is_read_only, created_at, updated_at
		FROM app_volumes
		WHERE id = ?
	`
	vol := &models.AppVolume{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&vol.ID,
		&vol.ApplicationID,
		&vol.Name,
		&vol.VolumeType,
		&vol.HostPath,
		&vol.MountPath,
		&vol.IsReadOnly,
		&vol.CreatedAt,
		&vol.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("volume not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get volume: %w", err)
	}

	return vol, nil
}

// DeleteVolume menghapus record volume dari database.
func (r *Repo) DeleteVolume(ctx context.Context, id string) error {
	query := `DELETE FROM app_volumes WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete volume: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("volume not found: %s", id)
	}
	return nil
}

// CreateFileMount menambahkan konfigurasi single file mount untuk aplikasi.
func (r *Repo) CreateFileMount(ctx context.Context, input models.CreateAppFileMountInput) (*models.AppFileMount, error) {
	mode := input.FileMode
	if mode == "" {
		mode = "0644"
	}

	fm := &models.AppFileMount{
		ID:            uuid.New().String(),
		ApplicationID: input.ApplicationID,
		MountPath:     input.MountPath,
		Content:       input.Content,
		FileMode:      mode,
		IsReadOnly:    input.IsReadOnly,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	query := `
		INSERT INTO app_file_mounts (id, application_id, mount_path, content, file_mode, is_read_only, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := r.db.ExecContext(ctx, query,
		fm.ID,
		fm.ApplicationID,
		fm.MountPath,
		fm.Content,
		fm.FileMode,
		fm.IsReadOnly,
		fm.CreatedAt,
		fm.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create file mount: %w", err)
	}

	return fm, nil
}

// GetFileMountsByAppID mengambil seluruh file mounts milik aplikasi.
func (r *Repo) GetFileMountsByAppID(ctx context.Context, appID string) ([]*models.AppFileMount, error) {
	query := `
		SELECT id, application_id, mount_path, content, file_mode, is_read_only, created_at, updated_at
		FROM app_file_mounts
		WHERE application_id = ?
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, appID)
	if err != nil {
		return nil, fmt.Errorf("failed to query file mounts: %w", err)
	}
	defer rows.Close()

	var fileMounts []*models.AppFileMount
	for rows.Next() {
		fm := &models.AppFileMount{}
		err := rows.Scan(
			&fm.ID,
			&fm.ApplicationID,
			&fm.MountPath,
			&fm.Content,
			&fm.FileMode,
			&fm.IsReadOnly,
			&fm.CreatedAt,
			&fm.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan file mount: %w", err)
		}
		fileMounts = append(fileMounts, fm)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return fileMounts, nil
}

// GetFileMountByID mengambil satu file mount berdasarkan ID.
func (r *Repo) GetFileMountByID(ctx context.Context, id string) (*models.AppFileMount, error) {
	query := `
		SELECT id, application_id, mount_path, content, file_mode, is_read_only, created_at, updated_at
		FROM app_file_mounts
		WHERE id = ?
	`
	fm := &models.AppFileMount{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&fm.ID,
		&fm.ApplicationID,
		&fm.MountPath,
		&fm.Content,
		&fm.FileMode,
		&fm.IsReadOnly,
		&fm.CreatedAt,
		&fm.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("file mount not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get file mount: %w", err)
	}

	return fm, nil
}

// UpdateFileMount memperbarui isi konten teks file mount.
func (r *Repo) UpdateFileMount(ctx context.Context, id string, content string) error {
	query := `
		UPDATE app_file_mounts
		SET content = ?, updated_at = ?
		WHERE id = ?
	`
	res, err := r.db.ExecContext(ctx, query, content, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("failed to update file mount: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("file mount not found: %s", id)
	}
	return nil
}

// DeleteFileMount menghapus record file mount dari database.
func (r *Repo) DeleteFileMount(ctx context.Context, id string) error {
	query := `DELETE FROM app_file_mounts WHERE id = ?`
	res, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete file mount: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("file mount not found: %s", id)
	}
	return nil
}
