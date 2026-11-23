package settings

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/akordium-id/get-labuh/internal/encryption"
	"github.com/akordium-id/get-labuh/internal/models"
)

// RegistryRepo mengelola data autentikasi Docker Registries private.
type RegistryRepo struct {
	db *sql.DB
}

func NewRegistryRepo(db *sql.DB) *RegistryRepo {
	return &RegistryRepo{db: db}
}

func (r *RegistryRepo) Create(ctx context.Context, input models.CreateDockerRegistryInput) (*models.DockerRegistry, error) {
	id := uuid.New().String()
	now := time.Now()

	storedPassword := input.Password
	if encryption.IsInitialized() && storedPassword != "" {
		encrypted, err := encryption.Encrypt(storedPassword)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt registry password: %w", err)
		}
		storedPassword = encryption.EncryptedValue(encrypted)
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO docker_registries (id, name, server_address, username, password, email, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		id, input.Name, input.ServerAddress, input.Username, storedPassword, input.Email, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert docker registry: %w", err)
	}

	return r.GetByID(ctx, id)
}

func (r *RegistryRepo) GetByID(ctx context.Context, id string) (*models.DockerRegistry, error) {
	reg := &models.DockerRegistry{}
	var rawPassword string

	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, server_address, username, password, email, created_at, updated_at
		 FROM docker_registries WHERE id = ?`,
		id,
	).Scan(&reg.ID, &reg.Name, &reg.ServerAddress, &reg.Username, &rawPassword, &reg.Email, &reg.CreatedAt, &reg.UpdatedAt)
	if err != nil {
		return nil, err
	}

	reg.Password = rawPassword
	if encryption.IsInitialized() && encryption.IsEncrypted(rawPassword) {
		cipherText := strings.TrimPrefix(rawPassword, "ENC:")
		decrypted, err := encryption.Decrypt(cipherText)
		if err == nil {
			reg.Password = decrypted
		}
	}

	return reg, nil
}

func (r *RegistryRepo) List(ctx context.Context) ([]*models.DockerRegistry, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, server_address, username, password, email, created_at, updated_at
		 FROM docker_registries ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.DockerRegistry
	for rows.Next() {
		reg := &models.DockerRegistry{}
		var rawPassword string
		if err := rows.Scan(&reg.ID, &reg.Name, &reg.ServerAddress, &reg.Username, &rawPassword, &reg.Email, &reg.CreatedAt, &reg.UpdatedAt); err != nil {
			return nil, err
		}

		reg.Password = rawPassword
		if encryption.IsInitialized() && encryption.IsEncrypted(rawPassword) {
			cipherText := strings.TrimPrefix(rawPassword, "ENC:")
			decrypted, err := encryption.Decrypt(cipherText)
			if err == nil {
				reg.Password = decrypted
			}
		}
		list = append(list, reg)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *RegistryRepo) Update(ctx context.Context, input models.UpdateDockerRegistryInput) error {
	now := time.Now()

	if input.Password != nil && *input.Password != "" {
		storedPassword := *input.Password
		if encryption.IsInitialized() {
			encrypted, err := encryption.Encrypt(storedPassword)
			if err != nil {
				return fmt.Errorf("failed to encrypt registry password: %w", err)
			}
			storedPassword = encryption.EncryptedValue(encrypted)
		}

		_, err := r.db.ExecContext(ctx,
			`UPDATE docker_registries
			 SET name = ?, server_address = ?, username = ?, password = ?, email = ?, updated_at = ?
			 WHERE id = ?`,
			input.Name, input.ServerAddress, input.Username, storedPassword, input.Email, now, input.ID,
		)
		return err
	}

	_, err := r.db.ExecContext(ctx,
		`UPDATE docker_registries
		 SET name = ?, server_address = ?, username = ?, email = ?, updated_at = ?
		 WHERE id = ?`,
		input.Name, input.ServerAddress, input.Username, input.Email, now, input.ID,
	)
	return err
}

func (r *RegistryRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM docker_registries WHERE id = ?", id)
	return err
}
