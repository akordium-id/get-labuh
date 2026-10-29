package deployments

import (
	"context"
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

// Repo menangani akses data deployments.
type Repo struct {
	db *sql.DB
}

// NewRepo membuat instance baru Repo.
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// CreateWithContext membuat deployment baru dengan context.
func (r *Repo) CreateWithContext(ctx context.Context, input models.CreateDeploymentInput) (*models.Deployment, error) {
	deployment := &models.Deployment{
		ID:            uuid.New().String(),
		ApplicationID: input.ApplicationID,
		CommitHash:    input.CommitHash,
		CommitMessage: input.CommitMessage,
		ImageName:     input.ImageName,
		Status:        models.DeployStatusQueued,
		Step:          input.Step,
		Output:        input.Output,
		ErrorMessage:  nil,
		LogPath:       input.LogPath,
		CreatedAt:     time.Now(),
	}

	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO deployments (id, application_id, commit_hash, commit_message, image_name, status, step, output, error_message, log_path, started_at, finished_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		deployment.ID, deployment.ApplicationID, deployment.CommitHash, deployment.CommitMessage, deployment.ImageName, deployment.Status, deployment.Step, deployment.Output, deployment.ErrorMessage, deployment.LogPath, nil, nil, deployment.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}

func (r *Repo) Create(input models.CreateDeploymentInput) (*models.Deployment, error) {
	return r.CreateWithContext(context.Background(), input)
}

// GetByIDWithContext mengambil deployment berdasarkan ID dengan context.
func (r *Repo) GetByIDWithContext(ctx context.Context, id string) (*models.Deployment, error) {
	deployment := &models.Deployment{}
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, application_id, commit_hash, commit_message, image_name, status, step, output, error_message, log_path, started_at, finished_at, created_at
		 FROM deployments WHERE id = ?`,
		id,
	).Scan(&deployment.ID, &deployment.ApplicationID, &deployment.CommitHash, &deployment.CommitMessage, &deployment.ImageName, &deployment.Status, &deployment.Step, &deployment.Output, &deployment.ErrorMessage, &deployment.LogPath, &deployment.StartedAt, &deployment.FinishedAt, &deployment.CreatedAt)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}

func (r *Repo) GetByID(id string) (*models.Deployment, error) {
	return r.GetByIDWithContext(context.Background(), id)
}

// GetByApplicationIDWithContext mengambil riwayat deployment suatu aplikasi dengan context.
func (r *Repo) GetByApplicationIDWithContext(ctx context.Context, appID string) ([]*models.Deployment, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, application_id, commit_hash, commit_message, image_name, status, step, output, error_message, log_path, started_at, finished_at, created_at
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
		err := rows.Scan(&deployment.ID, &deployment.ApplicationID, &deployment.CommitHash, &deployment.CommitMessage, &deployment.ImageName, &deployment.Status, &deployment.Step, &deployment.Output, &deployment.ErrorMessage, &deployment.LogPath, &deployment.StartedAt, &deployment.FinishedAt, &deployment.CreatedAt)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, deployment)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return deployments, nil
}

func (r *Repo) GetByApplicationID(appID string) ([]*models.Deployment, error) {
	return r.GetByApplicationIDWithContext(context.Background(), appID)
}

// GetByApplicationIDPaginatedWithContext mengambil deployment aplikasi berhalaman dengan context.
func (r *Repo) GetByApplicationIDPaginatedWithContext(ctx context.Context, appID string, limit, offset int) ([]*models.Deployment, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM deployments WHERE application_id = ?", appID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, application_id, commit_hash, commit_message, image_name, status, step, output, error_message, log_path, started_at, finished_at, created_at
		 FROM deployments WHERE application_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		appID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var deployments []*models.Deployment
	for rows.Next() {
		deployment := &models.Deployment{}
		err := rows.Scan(&deployment.ID, &deployment.ApplicationID, &deployment.CommitHash, &deployment.CommitMessage, &deployment.ImageName, &deployment.Status, &deployment.Step, &deployment.Output, &deployment.ErrorMessage, &deployment.LogPath, &deployment.StartedAt, &deployment.FinishedAt, &deployment.CreatedAt)
		if err != nil {
			return nil, 0, err
		}
		deployments = append(deployments, deployment)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return deployments, total, nil
}

func (r *Repo) GetByApplicationIDPaginated(appID string, limit, offset int) ([]*models.Deployment, int, error) {
	return r.GetByApplicationIDPaginatedWithContext(context.Background(), appID, limit, offset)
}

// GetByStatusPaginatedWithContext mengambil deployment berdasarkan status berhalaman dengan context.
func (r *Repo) GetByStatusPaginatedWithContext(ctx context.Context, status models.DeployStatus, limit, offset int) ([]*models.Deployment, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM deployments WHERE status = ?", status).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, application_id, commit_hash, commit_message, image_name, status, step, output, error_message, log_path, started_at, finished_at, created_at
		 FROM deployments WHERE status = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		status, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var deployments []*models.Deployment
	for rows.Next() {
		deployment := &models.Deployment{}
		err := rows.Scan(&deployment.ID, &deployment.ApplicationID, &deployment.CommitHash, &deployment.CommitMessage, &deployment.ImageName, &deployment.Status, &deployment.Step, &deployment.Output, &deployment.ErrorMessage, &deployment.LogPath, &deployment.StartedAt, &deployment.FinishedAt, &deployment.CreatedAt)
		if err != nil {
			return nil, 0, err
		}
		deployments = append(deployments, deployment)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return deployments, total, nil
}

func (r *Repo) GetByStatusPaginated(status models.DeployStatus, limit, offset int) ([]*models.Deployment, int, error) {
	return r.GetByStatusPaginatedWithContext(context.Background(), status, limit, offset)
}

// UpdateStatusWithContext memperbarui status deployment dengan context.
func (r *Repo) UpdateStatusWithContext(ctx context.Context, id string, status models.DeployStatus) error {
	now := time.Now()
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE deployments SET status = ?, updated_at = ? WHERE id = ?`,
		status, now, id,
	)
	return err
}

func (r *Repo) UpdateStatus(id string, status models.DeployStatus) error {
	return r.UpdateStatusWithContext(context.Background(), id, status)
}

// UpdateStatusWithErrorWithContext memperbarui status deployment dengan pesan error dan context.
func (r *Repo) UpdateStatusWithErrorWithContext(ctx context.Context, id string, status models.DeployStatus, errorMessage string) error {
	now := time.Now()
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE deployments SET status = ?, error_message = ?, updated_at = ? WHERE id = ?`,
		status, errorMessage, now, id,
	)
	return err
}

func (r *Repo) UpdateStatusWithError(id string, status models.DeployStatus, errorMessage string) error {
	return r.UpdateStatusWithErrorWithContext(context.Background(), id, status, errorMessage)
}

// UpdateStepWithContext memperbarui step deployment dengan context.
func (r *Repo) UpdateStepWithContext(ctx context.Context, id, step string) error {
	now := time.Now()
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE deployments SET step = ?, updated_at = ? WHERE id = ?`,
		step, now, id,
	)
	return err
}

func (r *Repo) UpdateStep(id, step string) error {
	return r.UpdateStepWithContext(context.Background(), id, step)
}

// UpdateOutputWithContext memperbarui output deployment dengan context.
func (r *Repo) UpdateOutputWithContext(ctx context.Context, id, output string) error {
	now := time.Now()
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE deployments SET output = ?, updated_at = ? WHERE id = ?`,
		output, now, id,
	)
	return err
}

func (r *Repo) UpdateOutput(id, output string) error {
	return r.UpdateOutputWithContext(context.Background(), id, output)
}

// MarkStartedWithContext menandai deployment dimulai dengan context.
func (r *Repo) MarkStartedWithContext(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE deployments SET started_at = ?, updated_at = ? WHERE id = ?`,
		now, now, id,
	)
	return err
}

func (r *Repo) MarkStarted(id string) error {
	return r.MarkStartedWithContext(context.Background(), id)
}

// MarkFinishedWithContext menandai deployment selesai dengan context.
func (r *Repo) MarkFinishedWithContext(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE deployments SET finished_at = ?, updated_at = ? WHERE id = ?`,
		now, now, id,
	)
	return err
}

func (r *Repo) MarkFinished(id string) error {
	return r.MarkFinishedWithContext(context.Background(), id)
}

// GetByProjectIDWithContext mengambil deployment berdasarkan project ID dengan context.
func (r *Repo) GetByProjectIDWithContext(ctx context.Context, projectID string) ([]*models.Deployment, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT d.id, d.application_id, d.commit_hash, d.commit_message, d.image_name, d.status, d.step, d.output, d.error_message, d.log_path, d.started_at, d.finished_at, d.created_at
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
		err := rows.Scan(&deployment.ID, &deployment.ApplicationID, &deployment.CommitHash, &deployment.CommitMessage, &deployment.ImageName, &deployment.Status, &deployment.Step, &deployment.Output, &deployment.ErrorMessage, &deployment.LogPath, &deployment.StartedAt, &deployment.FinishedAt, &deployment.CreatedAt)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, deployment)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return deployments, nil
}

func (r *Repo) GetByProjectID(projectID string) ([]*models.Deployment, error) {
	return r.GetByProjectIDWithContext(context.Background(), projectID)
}

// GetByEnvironmentIDWithContext mengambil deployment berdasarkan environment ID dengan context.
func (r *Repo) GetByEnvironmentIDWithContext(ctx context.Context, envID string) ([]*models.Deployment, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT d.id, d.application_id, d.commit_hash, d.commit_message, d.image_name, d.status, d.step, d.output, d.error_message, d.log_path, d.started_at, d.finished_at, d.created_at
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
		err := rows.Scan(&deployment.ID, &deployment.ApplicationID, &deployment.CommitHash, &deployment.CommitMessage, &deployment.ImageName, &deployment.Status, &deployment.Step, &deployment.Output, &deployment.ErrorMessage, &deployment.LogPath, &deployment.StartedAt, &deployment.FinishedAt, &deployment.CreatedAt)
		if err != nil {
			return nil, err
		}
		deployments = append(deployments, deployment)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return deployments, nil
}

func (r *Repo) GetByEnvironmentID(envID string) ([]*models.Deployment, error) {
	return r.GetByEnvironmentIDWithContext(context.Background(), envID)
}

// GetPreviousSuccessfulWithContext mengambil deployment sukses terakhir suatu aplikasi dengan context.
func (r *Repo) GetPreviousSuccessfulWithContext(ctx context.Context, appID string) (*models.Deployment, error) {
	deployment := &models.Deployment{}
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, application_id, commit_hash, commit_message, image_name, status, step, output, error_message, log_path, started_at, finished_at, created_at
		 FROM deployments
		 WHERE application_id = ? AND status = ?
		 ORDER BY created_at DESC
		 LIMIT 1`,
		appID, models.DeployStatusSuccess,
	).Scan(&deployment.ID, &deployment.ApplicationID, &deployment.CommitHash, &deployment.CommitMessage, &deployment.ImageName, &deployment.Status, &deployment.Step, &deployment.Output, &deployment.ErrorMessage, &deployment.LogPath, &deployment.StartedAt, &deployment.FinishedAt, &deployment.CreatedAt)
	if err != nil {
		return nil, err
	}

	return deployment, nil
}

func (r *Repo) GetPreviousSuccessful(appID string) (*models.Deployment, error) {
	return r.GetPreviousSuccessfulWithContext(context.Background(), appID)
}

// UpdateImageNameWithContext memperbarui nama docker image dari deployment.
func (r *Repo) UpdateImageNameWithContext(ctx context.Context, id, imageName string) error {
	now := time.Now()
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE deployments SET image_name = ?, updated_at = ? WHERE id = ?`,
		imageName, now, id,
	)
	return err
}

func (r *Repo) UpdateImageName(id, imageName string) error {
	return r.UpdateImageNameWithContext(context.Background(), id, imageName)
}

// GetRecentSuccessfulImagesWithContext mengambil daftar image_name dari N deployment sukses terakhir untuk retention cache.
func (r *Repo) GetRecentSuccessfulImagesWithContext(ctx context.Context, appID string, keepCount int) ([]string, error) {
	if keepCount <= 0 {
		keepCount = 5
	}
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT DISTINCT image_name
		 FROM deployments
		 WHERE application_id = ? AND status = ? AND image_name IS NOT NULL AND image_name != ''
		 ORDER BY created_at DESC
		 LIMIT ?`,
		appID, models.DeployStatusSuccess, keepCount,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []string
	for rows.Next() {
		var img string
		if err := rows.Scan(&img); err != nil {
			return nil, err
		}
		if img != "" {
			images = append(images, img)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return images, nil
}

func (r *Repo) GetRecentSuccessfulImages(appID string, keepCount int) ([]string, error) {
	return r.GetRecentSuccessfulImagesWithContext(context.Background(), appID, keepCount)
}
