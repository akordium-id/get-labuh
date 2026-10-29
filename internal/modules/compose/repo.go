package compose

import (
	"context"
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/modules/projects"
	"github.com/google/uuid"
)

// Repo mengelola operasi basis data untuk aplikasi berbasis Docker Compose.
type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) CreateWithContext(ctx context.Context, input models.CreateComposeApplicationInput) (*models.ComposeApplication, error) {
	compose := &models.ComposeApplication{
		ID:                 uuid.New().String(),
		EnvironmentID:      input.EnvironmentID,
		Name:               input.Name,
		Slug:               input.Slug,
		ComposeFilePath:    input.ComposeFilePath,
		ComposeProjectName: input.ComposeProjectName,
		CustomDomain:       input.CustomDomain,
		Status:             models.ComposeStatusIdle,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if compose.Slug == "" {
		compose.Slug = projects.GenerateSlug(compose.Name)
	}
	if compose.ComposeFilePath == "" {
		compose.ComposeFilePath = "docker-compose.yml"
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO compose_applications (id, environment_id, name, slug, compose_file_path, compose_project_name, custom_domain, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		compose.ID, compose.EnvironmentID, compose.Name, compose.Slug, compose.ComposeFilePath, compose.ComposeProjectName, compose.CustomDomain, compose.Status, compose.CreatedAt, compose.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return compose, nil
}

func (r *Repo) Create(input models.CreateComposeApplicationInput) (*models.ComposeApplication, error) {
	return r.CreateWithContext(context.Background(), input)
}

func (r *Repo) GetByIDWithContext(ctx context.Context, id string) (*models.ComposeApplication, error) {
	compose := &models.ComposeApplication{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, environment_id, name, slug, compose_file_path, compose_project_name, custom_domain, status, created_at, updated_at
		 FROM compose_applications WHERE id = ?`,
		id,
	).Scan(&compose.ID, &compose.EnvironmentID, &compose.Name, &compose.Slug, &compose.ComposeFilePath, &compose.ComposeProjectName, &compose.CustomDomain, &compose.Status, &compose.CreatedAt, &compose.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return compose, nil
}

func (r *Repo) GetByID(id string) (*models.ComposeApplication, error) {
	return r.GetByIDWithContext(context.Background(), id)
}

func (r *Repo) GetBySlugWithContext(ctx context.Context, envID, slug string) (*models.ComposeApplication, error) {
	compose := &models.ComposeApplication{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, environment_id, name, slug, compose_file_path, compose_project_name, custom_domain, status, created_at, updated_at
		 FROM compose_applications WHERE environment_id = ? AND slug = ?`,
		envID, slug,
	).Scan(&compose.ID, &compose.EnvironmentID, &compose.Name, &compose.Slug, &compose.ComposeFilePath, &compose.ComposeProjectName, &compose.CustomDomain, &compose.Status, &compose.CreatedAt, &compose.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return compose, nil
}

func (r *Repo) GetBySlug(envID, slug string) (*models.ComposeApplication, error) {
	return r.GetBySlugWithContext(context.Background(), envID, slug)
}

func (r *Repo) GetByEnvironmentIDWithContext(ctx context.Context, envID string) ([]*models.ComposeApplication, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, environment_id, name, slug, compose_file_path, compose_project_name, custom_domain, status, created_at, updated_at
		 FROM compose_applications WHERE environment_id = ? ORDER BY created_at DESC`,
		envID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var composes []*models.ComposeApplication
	for rows.Next() {
		compose := &models.ComposeApplication{}
		err := rows.Scan(&compose.ID, &compose.EnvironmentID, &compose.Name, &compose.Slug, &compose.ComposeFilePath, &compose.ComposeProjectName, &compose.CustomDomain, &compose.Status, &compose.CreatedAt, &compose.UpdatedAt)
		if err != nil {
			return nil, err
		}
		composes = append(composes, compose)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return composes, nil
}

func (r *Repo) GetByEnvironmentID(envID string) ([]*models.ComposeApplication, error) {
	return r.GetByEnvironmentIDWithContext(context.Background(), envID)
}

func (r *Repo) UpdateStatusWithContext(ctx context.Context, id string, status models.ComposeStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE compose_applications SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	return err
}

func (r *Repo) UpdateStatus(id string, status models.ComposeStatus) error {
	return r.UpdateStatusWithContext(context.Background(), id, status)
}

func (r *Repo) UpdateCustomDomainWithContext(ctx context.Context, id string, customDomain *string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE compose_applications SET custom_domain = ?, updated_at = ? WHERE id = ?`,
		customDomain, time.Now(), id,
	)
	return err
}

func (r *Repo) UpdateCustomDomain(id string, customDomain *string) error {
	return r.UpdateCustomDomainWithContext(context.Background(), id, customDomain)
}

func (r *Repo) DeleteWithContext(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM compose_applications WHERE id = ?", id)
	return err
}

func (r *Repo) Delete(id string) error {
	return r.DeleteWithContext(context.Background(), id)
}
