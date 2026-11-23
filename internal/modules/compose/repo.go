package compose

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/modules/projects"
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
		RawYAML:            input.RawYAML,
		ParsedServices:     []models.ComposeServiceDef{},
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

	parsedJSON := "[]"
	if len(compose.ParsedServices) > 0 {
		if b, err := json.Marshal(compose.ParsedServices); err == nil {
			parsedJSON = string(b)
		}
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO compose_applications (
			id, environment_id, name, slug, compose_file_path, compose_project_name, 
			custom_domain, raw_yaml, parsed_services, last_deployed_at, error_message, 
			status, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		compose.ID, compose.EnvironmentID, compose.Name, compose.Slug, compose.ComposeFilePath, compose.ComposeProjectName,
		compose.CustomDomain, compose.RawYAML, parsedJSON, compose.LastDeployedAt, compose.ErrorMessage,
		compose.Status, compose.CreatedAt, compose.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return compose, nil
}

func (r *Repo) Create(input models.CreateComposeApplicationInput) (*models.ComposeApplication, error) {
	return r.CreateWithContext(context.Background(), input)
}

func (r *Repo) scanRow(scanner interface{ Scan(dest ...interface{}) error }) (*models.ComposeApplication, error) {
	compose := &models.ComposeApplication{}
	var parsedJSON string
	err := scanner.Scan(
		&compose.ID,
		&compose.EnvironmentID,
		&compose.Name,
		&compose.Slug,
		&compose.ComposeFilePath,
		&compose.ComposeProjectName,
		&compose.CustomDomain,
		&compose.RawYAML,
		&parsedJSON,
		&compose.LastDeployedAt,
		&compose.ErrorMessage,
		&compose.Status,
		&compose.CreatedAt,
		&compose.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if parsedJSON != "" {
		_ = json.Unmarshal([]byte(parsedJSON), &compose.ParsedServices)
	}
	if compose.ParsedServices == nil {
		compose.ParsedServices = []models.ComposeServiceDef{}
	}

	return compose, nil
}

func (r *Repo) GetByIDWithContext(ctx context.Context, id string) (*models.ComposeApplication, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, environment_id, name, slug, compose_file_path, compose_project_name, 
		        custom_domain, raw_yaml, parsed_services, last_deployed_at, error_message, 
		        status, created_at, updated_at
		 FROM compose_applications WHERE id = ?`,
		id,
	)
	return r.scanRow(row)
}

func (r *Repo) GetByID(id string) (*models.ComposeApplication, error) {
	return r.GetByIDWithContext(context.Background(), id)
}

func (r *Repo) GetBySlugWithContext(ctx context.Context, envID, slug string) (*models.ComposeApplication, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, environment_id, name, slug, compose_file_path, compose_project_name, 
		        custom_domain, raw_yaml, parsed_services, last_deployed_at, error_message, 
		        status, created_at, updated_at
		 FROM compose_applications WHERE environment_id = ? AND slug = ?`,
		envID, slug,
	)
	return r.scanRow(row)
}

func (r *Repo) GetBySlug(envID, slug string) (*models.ComposeApplication, error) {
	return r.GetBySlugWithContext(context.Background(), envID, slug)
}

func (r *Repo) GetByEnvironmentIDWithContext(ctx context.Context, envID string) ([]*models.ComposeApplication, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, environment_id, name, slug, compose_file_path, compose_project_name, 
		        custom_domain, raw_yaml, parsed_services, last_deployed_at, error_message, 
		        status, created_at, updated_at
		 FROM compose_applications WHERE environment_id = ? ORDER BY created_at DESC`,
		envID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var composes []*models.ComposeApplication
	for rows.Next() {
		compose, err := r.scanRow(rows)
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

func (r *Repo) UpdateYAMLWithContext(ctx context.Context, id string, rawYAML string, parsedServices []models.ComposeServiceDef) error {
	parsedJSON := "[]"
	if len(parsedServices) > 0 {
		if b, err := json.Marshal(parsedServices); err == nil {
			parsedJSON = string(b)
		}
	}

	_, err := r.db.ExecContext(ctx,
		`UPDATE compose_applications SET raw_yaml = ?, parsed_services = ?, updated_at = ? WHERE id = ?`,
		rawYAML, parsedJSON, time.Now(), id,
	)
	return err
}

func (r *Repo) UpdateDeploymentStatusWithContext(ctx context.Context, id string, status models.ComposeStatus, lastDeployedAt *time.Time, errMsg *string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE compose_applications SET status = ?, last_deployed_at = ?, error_message = ?, updated_at = ? WHERE id = ?`,
		status, lastDeployedAt, errMsg, time.Now(), id,
	)
	return err
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
