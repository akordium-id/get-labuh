package repo

import (
	"database/sql"
	"strings"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

type DeployKeyRepo struct {
	db *sql.DB
}

func NewDeployKeyRepo(db *sql.DB) *DeployKeyRepo {
	return &DeployKeyRepo{db: db}
}

func (r *DeployKeyRepo) Create(input models.CreateDeployKeyInput) (*models.DeployKey, error) {
	deployKey := &models.DeployKey{
		ID:                uuid.New().String(),
		ProjectID:         input.ProjectID,
		Name:              input.Name,
		PublicKey:         input.PublicKey,
		PrivateKeyEncrypted: input.PrivateKeyEncrypted,
		Fingerprint:       input.Fingerprint,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	_, err := r.db.Exec(
		`INSERT INTO deploy_keys (id, project_id, name, public_key, private_key_encrypted, fingerprint, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		deployKey.ID, deployKey.ProjectID, deployKey.Name, deployKey.PublicKey, deployKey.PrivateKeyEncrypted, deployKey.Fingerprint, deployKey.CreatedAt, deployKey.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return deployKey, nil
}

func (r *DeployKeyRepo) GetByID(id string) (*models.DeployKey, error) {
	deployKey := &models.DeployKey{}
	err := r.db.QueryRow(
		`SELECT id, project_id, name, public_key, private_key_encrypted, fingerprint, created_at, updated_at
		 FROM deploy_keys WHERE id = ?`,
		id,
	).Scan(&deployKey.ID, &deployKey.ProjectID, &deployKey.Name, &deployKey.PublicKey, &deployKey.PrivateKeyEncrypted, &deployKey.Fingerprint, &deployKey.CreatedAt, &deployKey.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return deployKey, nil
}

func (r *DeployKeyRepo) GetByProjectID(projectID string) ([]*models.DeployKey, error) {
	rows, err := r.db.Query(
		`SELECT id, project_id, name, public_key, private_key_encrypted, fingerprint, created_at, updated_at
		 FROM deploy_keys WHERE project_id = ? ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployKeys []*models.DeployKey
	for rows.Next() {
		deployKey := &models.DeployKey{}
		err := rows.Scan(&deployKey.ID, &deployKey.ProjectID, &deployKey.Name, &deployKey.PublicKey, &deployKey.PrivateKeyEncrypted, &deployKey.Fingerprint, &deployKey.CreatedAt, &deployKey.UpdatedAt)
		if err != nil {
			return nil, err
		}
		deployKeys = append(deployKeys, deployKey)
	}

	return deployKeys, nil
}

func (r *DeployKeyRepo) GetByFingerprint(fingerprint string) (*models.DeployKey, error) {
	deployKey := &models.DeployKey{}
	err := r.db.QueryRow(
		`SELECT id, project_id, name, public_key, private_key_encrypted, fingerprint, created_at, updated_at
		 FROM deploy_keys WHERE fingerprint = ?`,
		fingerprint,
	).Scan(&deployKey.ID, &deployKey.ProjectID, &deployKey.Name, &deployKey.PublicKey, &deployKey.PrivateKeyEncrypted, &deployKey.Fingerprint, &deployKey.CreatedAt, &deployKey.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return deployKey, nil
}

func (r *DeployKeyRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM deploy_keys WHERE id = ?", id)
	return err
}

func GenerateFingerprint(publicKey string) string {
	parts := strings.Fields(publicKey)
	if len(parts) < 2 {
		return "unknown"
	}
	return parts[0] + " " + parts[1][:8] + "..."
}
