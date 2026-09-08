package repo

import (
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

type DeploymentRepo struct {
	db *sql.DB
}

func NewDeploymentRepo(db *sql.DB) *DeploymentRepo {
	return &DeploymentRepo{db: db}
}

func (r *DeploymentRepo) Create(input models.CreateDeploymentInput) (*models.Deployment, error) {
	deployment := &models.Deployment{
		ID:            uuid.New().String(),
		ApplicationID: input.ApplicationID,
		CommitHash:    input.CommitHash,
		CommitMessage: input.CommitMessage,
		Status:        models.DeployStatusQueued,
		ErrorMessage:  nil,
		LogPath:       input.LogPath,
		CreatedAt:     time.Now(),
	}

	_, err := r.db.Exec(
		`INSERT INTO deployments (id, application_id, commit_hash, commit_message, status, error_message, log_path, started_at, finished_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		deployment.ID, deployment.ApplicationID, deployment.CommitHash, deployment.CommitMessage, deployment.Status, deployment.ErrorMessage, deployment.LogPath, nil, nil, deployment.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}

func (r *DeploymentRepo) GetByID(id string) (*models.Deployment, error) {
	deployment := &models.Deployment{}
	err := r.db.QueryRow(
		`SELECT id, application_id, commit_hash, commit_message, status, error_message, log_path, started_at, finished_at, created_at
		 FROM deployments WHERE id = ?`,
		id,
	).Scan(&deployment.ID, &deployment.ApplicationID, &deployment.CommitHash, &deployment.CommitMessage, &deployment.Status, &deployment.ErrorMessage, &deployment.LogPath, &deployment.StartedAt, &deployment.FinishedAt, &deployment.CreatedAt)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}

func (r *DeploymentRepo) GetByApplicationID(appID string) ([]*models.Deployment, error) {
	rows, err := r.db.Query(
		`SELECT id, application_id, commit_hash, commit_message, status, error_message, log_path, started_at, finished_at, created_at
		 FROM deployments WHERE application_id = ? ORDER BY created_at DESC`,
		appID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployments []*models.Deployment
	for rows.Next() {
		deployment := &models.Deployment{}
		err := rows.Scan(&deployment.ID, &deployment.ApplicationID, &deployment.CommitHash, &deployment.CommitMessage, &deployment.Status, &deployment.ErrorMessage, &deployment.LogPath, &deployment.StartedAt, &deployment.FinishedAt, &deployment.CreatedAt)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, deployment)
	}

	return deployments, nil
}

func (r *DeploymentRepo) UpdateStatus(id string, status models.DeployStatus) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE deployments SET status = ?, updated_at = ? WHERE id = ?`,
		status, now, id,
	)
	return err
}

func (r *DeploymentRepo) UpdateStatusWithError(id string, status models.DeployStatus, errorMessage string) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE deployments SET status = ?, error_message = ?, updated_at = ? WHERE id = ?`,
		status, errorMessage, now, id,
	)
	return err
}

func (r *DeploymentRepo) MarkStarted(id string) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE deployments SET started_at = ?, updated_at = ? WHERE id = ?`,
		now, now, id,
	)
	return err
}

func (r *DeploymentRepo) MarkFinished(id string) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE deployments SET finished_at = ?, updated_at = ? WHERE id = ?`,
		now, now, id,
	)
	return err
}
