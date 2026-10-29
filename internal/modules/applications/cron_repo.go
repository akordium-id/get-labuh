package applications

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/akordium-id/get-labuh/internal/models"
)

type CronJobRepo struct {
	db *sql.DB
}

func NewCronJobRepo(db *sql.DB) *CronJobRepo {
	return &CronJobRepo{db: db}
}

func (r *CronJobRepo) CreateWithContext(ctx context.Context, input models.CreateCronJobInput) (*models.ApplicationCronJob, error) {
	id := uuid.New().String()
	now := time.Now()

	query := `
		INSERT INTO application_cron_jobs (
			id, application_id, name, command, cron_expression, is_enabled, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, 1, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		id, input.ApplicationID, input.Name, input.Command, input.CronExpression, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cron job: %w", err)
	}

	return r.GetByIDWithContext(ctx, id)
}

func (r *CronJobRepo) Create(input models.CreateCronJobInput) (*models.ApplicationCronJob, error) {
	return r.CreateWithContext(context.Background(), input)
}

func (r *CronJobRepo) GetByIDWithContext(ctx context.Context, id string) (*models.ApplicationCronJob, error) {
	query := `
		SELECT id, application_id, name, command, cron_expression, is_enabled,
		       last_run_at, last_status, last_output, created_at, updated_at
		FROM application_cron_jobs
		WHERE id = ?`

	job := &models.ApplicationCronJob{}
	var lastStatus *string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&job.ID, &job.ApplicationID, &job.Name, &job.Command, &job.CronExpression, &job.IsEnabled,
		&job.LastRunAt, &lastStatus, &job.LastOutput, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if lastStatus != nil {
		st := models.CronJobStatus(*lastStatus)
		job.LastStatus = &st
	}
	return job, nil
}

func (r *CronJobRepo) GetByID(id string) (*models.ApplicationCronJob, error) {
	return r.GetByIDWithContext(context.Background(), id)
}

func (r *CronJobRepo) GetByApplicationIDWithContext(ctx context.Context, appID string) ([]*models.ApplicationCronJob, error) {
	query := `
		SELECT id, application_id, name, command, cron_expression, is_enabled,
		       last_run_at, last_status, last_output, created_at, updated_at
		FROM application_cron_jobs
		WHERE application_id = ?
		ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, appID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*models.ApplicationCronJob
	for rows.Next() {
		job := &models.ApplicationCronJob{}
		var lastStatus *string
		err := rows.Scan(
			&job.ID, &job.ApplicationID, &job.Name, &job.Command, &job.CronExpression, &job.IsEnabled,
			&job.LastRunAt, &lastStatus, &job.LastOutput, &job.CreatedAt, &job.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if lastStatus != nil {
			st := models.CronJobStatus(*lastStatus)
			job.LastStatus = &st
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (r *CronJobRepo) GetByApplicationID(appID string) ([]*models.ApplicationCronJob, error) {
	return r.GetByApplicationIDWithContext(context.Background(), appID)
}

func (r *CronJobRepo) ListActiveJobsWithContext(ctx context.Context) ([]*models.ApplicationCronJob, error) {
	query := `
		SELECT id, application_id, name, command, cron_expression, is_enabled,
		       last_run_at, last_status, last_output, created_at, updated_at
		FROM application_cron_jobs
		WHERE is_enabled = 1
		ORDER BY created_at ASC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*models.ApplicationCronJob
	for rows.Next() {
		job := &models.ApplicationCronJob{}
		var lastStatus *string
		err := rows.Scan(
			&job.ID, &job.ApplicationID, &job.Name, &job.Command, &job.CronExpression, &job.IsEnabled,
			&job.LastRunAt, &lastStatus, &job.LastOutput, &job.CreatedAt, &job.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		if lastStatus != nil {
			st := models.CronJobStatus(*lastStatus)
			job.LastStatus = &st
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (r *CronJobRepo) ListActiveJobs() ([]*models.ApplicationCronJob, error) {
	return r.ListActiveJobsWithContext(context.Background())
}

func (r *CronJobRepo) UpdateExecutionWithContext(ctx context.Context, id string, status models.CronJobStatus, output string) error {
	now := time.Now()
	query := `
		UPDATE application_cron_jobs
		SET last_run_at = ?, last_status = ?, last_output = ?, updated_at = ?
		WHERE id = ?`

	_, err := r.db.ExecContext(ctx, query, now, string(status), output, now, id)
	return err
}

func (r *CronJobRepo) UpdateExecution(id string, status models.CronJobStatus, output string) error {
	return r.UpdateExecutionWithContext(context.Background(), id, status, output)
}

func (r *CronJobRepo) ToggleEnabledWithContext(ctx context.Context, id string) (bool, error) {
	job, err := r.GetByIDWithContext(ctx, id)
	if err != nil {
		return false, err
	}
	newStatus := !job.IsEnabled
	now := time.Now()

	query := `UPDATE application_cron_jobs SET is_enabled = ?, updated_at = ? WHERE id = ?`
	_, err = r.db.ExecContext(ctx, query, newStatus, now, id)
	return newStatus, err
}

func (r *CronJobRepo) ToggleEnabled(id string) (bool, error) {
	return r.ToggleEnabledWithContext(context.Background(), id)
}

func (r *CronJobRepo) DeleteWithContext(ctx context.Context, id string) error {
	query := `DELETE FROM application_cron_jobs WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *CronJobRepo) Delete(id string) error {
	return r.DeleteWithContext(context.Background(), id)
}
