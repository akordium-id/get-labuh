package repo

import (
	"context"
	"database/sql"
	"strings"

	"github.com/google/uuid"

	"github.com/akordium-id/get-labuh/internal/encryption"
	"github.com/akordium-id/get-labuh/internal/models"
)

// SharedEnvVarRepo mengelola penyimpanan variabel lingkungan bersama (level Project atau Team).
type SharedEnvVarRepo struct {
	db     *sql.DB
	encKey string
}

// NewSharedEnvVarRepo membuat instance baru SharedEnvVarRepo.
func NewSharedEnvVarRepo(db *sql.DB, encKey ...string) *SharedEnvVarRepo {
	k := ""
	if len(encKey) > 0 {
		k = encKey[0]
	}
	return &SharedEnvVarRepo{
		db:     db,
		encKey: k,
	}
}

// Set menyimpan atau mengupdate (UPSERT) satu variabel lingkungan bersama.
func (r *SharedEnvVarRepo) Set(ctx context.Context, item *models.SharedEnvVar) error {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}

	val := item.Value
	isEnc := false
	if item.IsSecret && encryption.IsInitialized() {
		enc, err := encryption.Encrypt(val)
		if err != nil {
			return err
		}
		val = encryption.EncryptedValue(enc)
		isEnc = true
	}

	query := `
		INSERT INTO shared_env_vars (id, scope, scope_id, key, value, is_secret, is_encrypted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(scope, scope_id, key) DO UPDATE SET
			value = excluded.value,
			is_secret = excluded.is_secret,
			is_encrypted = excluded.is_encrypted,
			updated_at = CURRENT_TIMESTAMP;
	`
	_, err := r.db.ExecContext(ctx, query,
		item.ID,
		item.Scope,
		item.ScopeID,
		item.Key,
		val,
		item.IsSecret,
		isEnc,
	)
	if err != nil {
		return err
	}

	item.IsEncrypted = isEnc
	return nil
}

// ListByScope mengambil seluruh shared env vars berdasarkan scope dan scopeID.
func (r *SharedEnvVarRepo) ListByScope(ctx context.Context, scope models.SharedEnvVarScope, scopeID string) ([]*models.SharedEnvVar, error) {
	query := `
		SELECT id, scope, scope_id, key, value, is_secret, is_encrypted, created_at, updated_at
		FROM shared_env_vars
		WHERE scope = ? AND scope_id = ?
		ORDER BY key ASC;
	`
	rows, err := r.db.QueryContext(ctx, query, scope, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*models.SharedEnvVar
	for rows.Next() {
		item := &models.SharedEnvVar{}
		err := rows.Scan(
			&item.ID,
			&item.Scope,
			&item.ScopeID,
			&item.Key,
			&item.Value,
			&item.IsSecret,
			&item.IsEncrypted,
			&item.CreatedAt,
			&item.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if item.IsEncrypted && encryption.IsInitialized() {
			decrypted, err := encryption.Decrypt(strings.TrimPrefix(item.Value, "ENC:"))
			if err == nil {
				item.Value = decrypted
			}
		}

		result = append(result, item)
	}

	return result, rows.Err()
}

// GetByKey mengambil satu shared env var berdasarkan scope, scopeID, dan key.
func (r *SharedEnvVarRepo) GetByKey(ctx context.Context, scope models.SharedEnvVarScope, scopeID, key string) (*models.SharedEnvVar, error) {
	query := `
		SELECT id, scope, scope_id, key, value, is_secret, is_encrypted, created_at, updated_at
		FROM shared_env_vars
		WHERE scope = ? AND scope_id = ? AND key = ?
		LIMIT 1;
	`
	row := r.db.QueryRowContext(ctx, query, scope, scopeID, key)

	item := &models.SharedEnvVar{}
	err := row.Scan(
		&item.ID,
		&item.Scope,
		&item.ScopeID,
		&item.Key,
		&item.Value,
		&item.IsSecret,
		&item.IsEncrypted,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if item.IsEncrypted && encryption.IsInitialized() {
		decrypted, err := encryption.Decrypt(strings.TrimPrefix(item.Value, "ENC:"))
		if err == nil {
			item.Value = decrypted
		}
	}

	return item, nil
}

// Delete menghapus satu shared env var berdasarkan ID.
func (r *SharedEnvVarRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM shared_env_vars WHERE id = ?", id)
	return err
}

// DeleteByKey menghapus satu shared env var berdasarkan scope, scopeID, dan key.
func (r *SharedEnvVarRepo) DeleteByKey(ctx context.Context, scope models.SharedEnvVarScope, scopeID, key string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM shared_env_vars WHERE scope = ? AND scope_id = ? AND key = ?", scope, scopeID, key)
	return err
}

// BulkUpsert menyimpan banyak variabel sekaligus di dalam satu transaksi.
func (r *SharedEnvVarRepo) BulkUpsert(ctx context.Context, scope models.SharedEnvVarScope, scopeID string, items []*models.SharedEnvVar) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO shared_env_vars (id, scope, scope_id, key, value, is_secret, is_encrypted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		ON CONFLICT(scope, scope_id, key) DO UPDATE SET
			value = excluded.value,
			is_secret = excluded.is_secret,
			is_encrypted = excluded.is_encrypted,
			updated_at = CURRENT_TIMESTAMP;
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range items {
		if item.ID == "" {
			item.ID = uuid.New().String()
		}
		val := item.Value
		isEnc := false
		if item.IsSecret && encryption.IsInitialized() {
			enc, err := encryption.Encrypt(val)
			if err != nil {
				return err
			}
			val = encryption.EncryptedValue(enc)
			isEnc = true
		}

		_, err := stmt.ExecContext(ctx,
			item.ID,
			scope,
			scopeID,
			item.Key,
			val,
			item.IsSecret,
			isEnc,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}
