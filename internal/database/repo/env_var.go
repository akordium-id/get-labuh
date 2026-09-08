package repo

import (
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

type EnvVarRepo struct {
	db *sql.DB
}

func NewEnvVarRepo(db *sql.DB) *EnvVarRepo {
	return &EnvVarRepo{db: db}
}

func (r *EnvVarRepo) Create(input models.CreateAppEnvVarInput) (*models.AppEnvVar, error) {
	envVar := &models.AppEnvVar{
		ID:            uuid.New().String(),
		ApplicationID: input.ApplicationID,
		Key:           input.Key,
		Value:         input.Value,
		IsSecret:      input.IsSecret,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	_, err := r.db.Exec(
		`INSERT INTO app_env_vars (id, application_id, key, value, is_secret, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		envVar.ID, envVar.ApplicationID, envVar.Key, envVar.Value, envVar.IsSecret, envVar.CreatedAt, envVar.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return envVar, nil
}

func (r *EnvVarRepo) GetByID(id string) (*models.AppEnvVar, error) {
	envVar := &models.AppEnvVar{}
	err := r.db.QueryRow(
		`SELECT id, application_id, key, value, is_secret, created_at, updated_at
		 FROM app_env_vars WHERE id = ?`,
		id,
	).Scan(&envVar.ID, &envVar.ApplicationID, &envVar.Key, &envVar.Value, &envVar.IsSecret, &envVar.CreatedAt, &envVar.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return envVar, nil
}

func (r *EnvVarRepo) GetByApplicationID(appID string) ([]*models.AppEnvVar, error) {
	rows, err := r.db.Query(
		`SELECT id, application_id, key, value, is_secret, created_at, updated_at
		 FROM app_env_vars WHERE application_id = ? ORDER BY created_at DESC`,
		appID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var envVars []*models.AppEnvVar
	for rows.Next() {
		envVar := &models.AppEnvVar{}
		err := rows.Scan(&envVar.ID, &envVar.ApplicationID, &envVar.Key, &envVar.Value, &envVar.IsSecret, &envVar.CreatedAt, &envVar.UpdatedAt)
		if err != nil {
			return nil, err
		}
		envVars = append(envVars, envVar)
	}

	return envVars, nil
}

func (r *EnvVarRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM app_env_vars WHERE id = ?", id)
	return err
}
