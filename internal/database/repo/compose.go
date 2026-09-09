package repo

import (
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

type ComposeRepo struct {
	db *sql.DB
}

func NewComposeRepo(db *sql.DB) *ComposeRepo {
	return &ComposeRepo{db: db}
}

func (r *ComposeRepo) Create(input models.CreateComposeApplicationInput) (*models.ComposeApplication, error) {
	compose := &models.ComposeApplication{
		ID:                uuid.New().String(),
		EnvironmentID:     input.EnvironmentID,
		Name:              input.Name,
		Slug:              input.Slug,
		ComposeFilePath:   input.ComposeFilePath,
		ComposeProjectName: input.ComposeProjectName,
		CustomDomain:      input.CustomDomain,
		Status:            models.ComposeStatusIdle,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if compose.Slug == "" {
		compose.Slug = GenerateSlug(compose.Name)
	}
	if compose.ComposeFilePath == "" {
		compose.ComposeFilePath = "docker-compose.yml"
	}

	_, err := r.db.Exec(
		`INSERT INTO compose_applications (id, environment_id, name, slug, compose_file_path, compose_project_name, custom_domain, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		compose.ID, compose.EnvironmentID, compose.Name, compose.Slug, compose.ComposeFilePath, compose.ComposeProjectName, compose.CustomDomain, compose.Status, compose.CreatedAt, compose.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return compose, nil
}

func (r *ComposeRepo) GetByID(id string) (*models.ComposeApplication, error) {
	compose := &models.ComposeApplication{}
	err := r.db.QueryRow(
		`SELECT id, environment_id, name, slug, compose_file_path, compose_project_name, custom_domain, status, created_at, updated_at
		 FROM compose_applications WHERE id = ?`,
		id,
	).Scan(&compose.ID, &compose.EnvironmentID, &compose.Name, &compose.Slug, &compose.ComposeFilePath, &compose.ComposeProjectName, &compose.CustomDomain, &compose.Status, &compose.CreatedAt, &compose.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return compose, nil
}

func (r *ComposeRepo) GetBySlug(envID, slug string) (*models.ComposeApplication, error) {
	compose := &models.ComposeApplication{}
	err := r.db.QueryRow(
		`SELECT id, environment_id, name, slug, compose_file_path, compose_project_name, custom_domain, status, created_at, updated_at
		 FROM compose_applications WHERE environment_id = ? AND slug = ?`,
		envID, slug,
	).Scan(&compose.ID, &compose.EnvironmentID, &compose.Name, &compose.Slug, &compose.ComposeFilePath, &compose.ComposeProjectName, &compose.CustomDomain, &compose.Status, &compose.CreatedAt, &compose.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return compose, nil
}

func (r *ComposeRepo) GetByEnvironmentID(envID string) ([]*models.ComposeApplication, error) {
	rows, err := r.db.Query(
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

	return composes, nil
}

func (r *ComposeRepo) UpdateStatus(id string, status models.ComposeStatus) error {
	_, err := r.db.Exec(
		`UPDATE compose_applications SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	return err
}

func (r *ComposeRepo) UpdateCustomDomain(id string, customDomain *string) error {
	_, err := r.db.Exec(
		`UPDATE compose_applications SET custom_domain = ?, updated_at = ? WHERE id = ?`,
		customDomain, time.Now(), id,
	)
	return err
}

func (r *ComposeRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM compose_applications WHERE id = ?", id)
	return err
}
