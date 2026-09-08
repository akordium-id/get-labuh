package repo

import (
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

type ApiKeyRepo struct {
	db *sql.DB
}

func NewApiKeyRepo(db *sql.DB) *ApiKeyRepo {
	return &ApiKeyRepo{db: db}
}

func (r *ApiKeyRepo) Create(input models.CreateApiKeyInput) (*models.ApiKey, error) {
	apiKey := &models.ApiKey{
		ID:       uuid.New().String(),
		UserID:   input.UserID,
		Name:     input.Name,
		KeyHash:  input.KeyHash,
		CreatedAt: time.Now(),
	}

	_, err := r.db.Exec(
		"INSERT INTO api_keys (id, user_id, name, key_hash, created_at) VALUES (?, ?, ?, ?, ?)",
		apiKey.ID, apiKey.UserID, apiKey.Name, apiKey.KeyHash, apiKey.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return apiKey, nil
}

func (r *ApiKeyRepo) GetByID(id string) (*models.ApiKey, error) {
	apiKey := &models.ApiKey{}
	err := r.db.QueryRow(
		"SELECT id, user_id, name, key_hash, last_used_at, created_at FROM api_keys WHERE id = ?",
		id,
	).Scan(&apiKey.ID, &apiKey.UserID, &apiKey.Name, &apiKey.KeyHash, &apiKey.LastUsedAt, &apiKey.CreatedAt)
	if err != nil {
		return nil, err
	}

	return apiKey, nil
}

func (r *ApiKeyRepo) GetByKeyHash(keyHash string) (*models.ApiKey, error) {
	apiKey := &models.ApiKey{}
	err := r.db.QueryRow(
		"SELECT id, user_id, name, key_hash, last_used_at, created_at FROM api_keys WHERE key_hash = ?",
		keyHash,
	).Scan(&apiKey.ID, &apiKey.UserID, &apiKey.Name, &apiKey.KeyHash, &apiKey.LastUsedAt, &apiKey.CreatedAt)
	if err != nil {
		return nil, err
	}

	return apiKey, nil
}

func (r *ApiKeyRepo) GetByUserID(userID string) ([]*models.ApiKey, error) {
	rows, err := r.db.Query(
		"SELECT id, user_id, name, key_hash, last_used_at, created_at FROM api_keys WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apiKeys []*models.ApiKey
	for rows.Next() {
		apiKey := &models.ApiKey{}
		err := rows.Scan(&apiKey.ID, &apiKey.UserID, &apiKey.Name, &apiKey.KeyHash, &apiKey.LastUsedAt, &apiKey.CreatedAt)
		if err != nil {
			return nil, err
		}
		apiKeys = append(apiKeys, apiKey)
	}

	return apiKeys, nil
}

func (r *ApiKeyRepo) UpdateLastUsed(id string) error {
	now := time.Now()
	_, err := r.db.Exec("UPDATE api_keys SET last_used_at = ? WHERE id = ?", now, id)
	return err
}

func (r *ApiKeyRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM api_keys WHERE id = ?", id)
	return err
}
