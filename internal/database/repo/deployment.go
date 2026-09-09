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
		Step:          input.Step,
		Output:        input.Output,
		ErrorMessage:  nil,
		LogPath:       input.LogPath,
		CreatedAt:     time.Now(),
	}

	_, err := r.db.Exec(
		`INSERT INTO deployments (id, application_id, commit_hash, commit_message, status, step, output, error_message, log_path, started_at, finished_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		deployment.ID, deployment.ApplicationID, deployment.CommitHash, deployment.CommitMessage, deployment.Status, deployment.Step, deployment.Output, deployment.ErrorMessage, deployment.LogPath, nil, nil, deployment.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}

func (r *DeploymentRepo) GetByID(id string) (*models.Deployment, error) {
	deployment := &models.Deployment{}
	err := r.db.QueryRow(
		`SELECT id, application_id, commit_hash, commit_message, status, step, output, error_message, log_path, started_at, finished_at, created_at
		 FROM deployments WHERE id = ?`,
		id,
	).Scan(&deployment.ID, &deployment.ApplicationID, &deployment.CommitHash, &deployment.CommitMessage, &deployment.Status, &deployment.Step, &deployment.Output, &deployment.ErrorMessage, &deployment.LogPath, &deployment.StartedAt, &deployment.FinishedAt, &deployment.CreatedAt)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}

func (r *DeploymentRepo) GetByApplicationID(appID string) ([]*models.Deployment, error) {
	return r.GetByApplicationIDPaginated(appID, 0, 0)
}

func (r *DeploymentRepo) GetByApplicationIDPaginated(appID string, limit, offset int) ([]*models.Deployment, error) {
	query := `SELECT id, application_id, commit_hash, commit_message, status, step, output, error_message, log_path, started_at, finished_at, created_at
		 FROM deployments WHERE application_id = ? ORDER BY created_at DESC`
	var rows *sql.Rows
	var err error

	if limit > 0 {
		query += " LIMIT ? OFFSET ?"
		rows, err = r.db.Query(query, appID, limit, offset)
	} else {
		rows, err = r.db.Query(query, appID)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployments []*models.Deployment
	for rows.Next() {
		deployment := &models.Deployment{}
		err := rows.Scan(&deployment.ID, &deployment.ApplicationID, &deployment.CommitHash, &deployment.CommitMessage, &deployment.Status, &deployment.Step, &deployment.Output, &deployment.ErrorMessage, &deployment.LogPath, &deployment.StartedAt, &deployment.FinishedAt, &deployment.CreatedAt)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, deployment)
	}

	return deployments, nil
}

func (r *DeploymentRepo) GetByStatus(status models.DeployStatus) ([]*models.Deployment, error) {
	rows, err := r.db.Query(
		`SELECT id, application_id, commit_hash, commit_message, status, step, output, error_message, log_path, started_at, finished_at, created_at
		 FROM deployments WHERE status = ? ORDER BY created_at DESC`,
		status,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployments []*models.Deployment
	for rows.Next() {
		deployment := &models.Deployment{}
		err := rows.Scan(&deployment.ID, &deployment.ApplicationID, &deployment.CommitHash, &deployment.CommitMessage, &deployment.Status, &deployment.Step, &deployment.Output, &deployment.ErrorMessage, &deployment.LogPath, &deployment.StartedAt, &deployment.FinishedAt, &deployment.CreatedAt)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, deployment)
	}

	return deployments, nil
}

func (r *DeploymentRepo) CountByStatus(status models.DeployStatus) (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM deployments WHERE status = ?", status).Scan(&count)
	return count, err
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

func (r *DeploymentRepo) UpdateStep(id, step string) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE deployments SET step = ?, updated_at = ? WHERE id = ?`,
		step, now, id,
	)
	return err
}

func (r *DeploymentRepo) UpdateOutput(id, output string) error {
	now := time.Now()
	_, err := r.db.Exec(
		`UPDATE deployments SET output = ?, updated_at = ? WHERE id = ?`,
		output, now, id,
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

func (r *DeploymentRepo) GetByProjectID(projectID string) ([]*models.Deployment, error) {
	rows, err := r.db.Query(
		`SELECT d.id, d.application_id, d.commit_hash, d.commit_message, d.status, d.step, d.output, d.error_message, d.log_path, d.started_at, d.finished_at, d.created_at
		 FROM deployments d
		 JOIN applications a ON d.application_id = a.id
		 JOIN environments e ON a.environment_id = e.id
		 WHERE e.project_id = ?
		 ORDER BY d.created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployments []*models.Deployment
	for rows.Next() {
		deployment := &models.Deployment{}
		err := rows.Scan(&deployment.ID, &deployment.ApplicationID, &deployment.CommitHash, &deployment.CommitMessage, &deployment.Status, &deployment.Step, &deployment.Output, &deployment.ErrorMessage, &deployment.LogPath, &deployment.StartedAt, &deployment.FinishedAt, &deployment.CreatedAt)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, deployment)
	}

	return deployments, nil
}

func (r *DeploymentRepo) GetByEnvironmentID(envID string) ([]*models.Deployment, error) {
	rows, err := r.db.Query(
		`SELECT d.id, d.application_id, d.commit_hash, d.commit_message, d.status, d.step, d.output, d.error_message, d.log_path, d.started_at, d.finished_at, d.created_at
		 FROM deployments d
		 JOIN applications a ON d.application_id = a.id
		 WHERE a.environment_id = ?
		 ORDER BY d.created_at DESC`,
		envID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deployments []*models.Deployment
	for rows.Next() {
		deployment := &models.Deployment{}
		err := rows.Scan(&deployment.ID, &deployment.ApplicationID, &deployment.CommitHash, &deployment.CommitMessage, &deployment.Status, &deployment.Step, &deployment.Output, &deployment.ErrorMessage, &deployment.LogPath, &deployment.StartedAt, &deployment.FinishedAt, &deployment.CreatedAt)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, deployment)
	}

	return deployments, nil
}

func (r *DeploymentRepo) GetPreviousSuccessful(appID string) (*models.Deployment, error) {
	deployment := &models.Deployment{}
	err := r.db.QueryRow(
		`SELECT id, application_id, commit_hash, commit_message, status, step, output, error_message, log_path, started_at, finished_at, created_at
		 FROM deployments
		 WHERE application_id = ? AND status = ?
		 ORDER BY created_at DESC
		 LIMIT 1`,
		appID, models.DeployStatusSuccess,
	).Scan(&deployment.ID, &deployment.ApplicationID, &deployment.CommitHash, &deployment.CommitMessage, &deployment.Status, &deployment.Step, &deployment.Output, &deployment.ErrorMessage, &deployment.LogPath, &deployment.StartedAt, &deployment.FinishedAt, &deployment.CreatedAt)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}
