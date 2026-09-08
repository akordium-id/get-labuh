package repo

import (
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

type TemplateRepo struct {
	db *sql.DB
}

func NewTemplateRepo(db *sql.DB) *TemplateRepo {
	return &TemplateRepo{db: db}
}

func (r *TemplateRepo) Create(input models.CreateServiceTemplateInput) (*models.ServiceTemplate, error) {
	template := &models.ServiceTemplate{
		ID:            uuid.New().String(),
		Name:          input.Name,
		Description:   input.Description,
		SourceType:    input.SourceType,
		RepositoryURL: input.RepositoryURL,
		DockerImage:   input.DockerImage,
		ComposeYAML:   input.ComposeYAML,
		IconURL:       input.IconURL,
		Category:      input.Category,
		IsOfficial:    input.IsOfficial,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	_, err := r.db.Exec(
		`INSERT INTO service_templates (id, name, description, source_type, repository_url, docker_image, compose_yaml, icon_url, category, is_official, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		template.ID, template.Name, template.Description, template.SourceType, template.RepositoryURL,
		template.DockerImage, template.ComposeYAML, template.IconURL, template.Category, template.IsOfficial,
		template.CreatedAt, template.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return template, nil
}

func (r *TemplateRepo) GetByID(id string) (*models.ServiceTemplate, error) {
	template := &models.ServiceTemplate{}
	err := r.db.QueryRow(
		`SELECT id, name, description, source_type, repository_url, docker_image, compose_yaml, icon_url, category, is_official, created_at, updated_at
		 FROM service_templates WHERE id = ?`,
		id,
	).Scan(&template.ID, &template.Name, &template.Description, &template.SourceType, &template.RepositoryURL,
		&template.DockerImage, &template.ComposeYAML, &template.IconURL, &template.Category, &template.IsOfficial,
		&template.CreatedAt, &template.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return template, nil
}

func (r *TemplateRepo) GetAll() ([]*models.ServiceTemplate, error) {
	rows, err := r.db.Query(
		`SELECT id, name, description, source_type, repository_url, docker_image, compose_yaml, icon_url, category, is_official, created_at, updated_at
		 FROM service_templates ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []*models.ServiceTemplate
	for rows.Next() {
		template := &models.ServiceTemplate{}
		err := rows.Scan(&template.ID, &template.Name, &template.Description, &template.SourceType, &template.RepositoryURL,
			&template.DockerImage, &template.ComposeYAML, &template.IconURL, &template.Category, &template.IsOfficial,
			&template.CreatedAt, &template.UpdatedAt)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}

	return templates, nil
}

func (r *TemplateRepo) GetByCategory(category models.TemplateCategory) ([]*models.ServiceTemplate, error) {
	rows, err := r.db.Query(
		`SELECT id, name, description, source_type, repository_url, docker_image, compose_yaml, icon_url, category, is_official, created_at, updated_at
		 FROM service_templates WHERE category = ? ORDER BY created_at DESC`,
		category,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []*models.ServiceTemplate
	for rows.Next() {
		template := &models.ServiceTemplate{}
		err := rows.Scan(&template.ID, &template.Name, &template.Description, &template.SourceType, &template.RepositoryURL,
			&template.DockerImage, &template.ComposeYAML, &template.IconURL, &template.Category, &template.IsOfficial,
			&template.CreatedAt, &template.UpdatedAt)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}

	return templates, nil
}

func (r *TemplateRepo) Update(id string, input models.CreateServiceTemplateInput) (*models.ServiceTemplate, error) {
	_, err := r.db.Exec(
		`UPDATE service_templates SET name = ?, description = ?, source_type = ?, repository_url = ?, docker_image = ?, compose_yaml = ?, icon_url = ?, category = ?, is_official = ?, updated_at = ? WHERE id = ?`,
		input.Name, input.Description, input.SourceType, input.RepositoryURL, input.DockerImage, input.ComposeYAML, input.IconURL, input.Category, input.IsOfficial, time.Now(), id,
	)
	if err != nil {
		return nil, err
	}

	return r.GetByID(id)
}

func (r *TemplateRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM service_templates WHERE id = ?", id)
	return err
}

func (r *TemplateRepo) CreateVariable(input models.CreateTemplateVariableInput) (*models.TemplateVariable, error) {
	variable := &models.TemplateVariable{
		ID:          uuid.New().String(),
		TemplateID:  input.TemplateID,
		Key:         input.Key,
		DisplayName: input.DisplayName,
		Description: input.Description,
		Required:    input.Required,
		DefaultValue: input.DefaultValue,
		CreatedAt:   time.Now(),
	}

	_, err := r.db.Exec(
		`INSERT INTO template_variables (id, template_id, key, display_name, description, required, default_value, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		variable.ID, variable.TemplateID, variable.Key, variable.DisplayName, variable.Description, variable.Required, variable.DefaultValue, variable.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return variable, nil
}

func (r *TemplateRepo) GetVariablesByTemplateID(templateID string) ([]*models.TemplateVariable, error) {
	rows, err := r.db.Query(
		`SELECT id, template_id, key, display_name, description, required, default_value, created_at
		 FROM template_variables WHERE template_id = ? ORDER BY created_at ASC`,
		templateID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var variables []*models.TemplateVariable
	for rows.Next() {
		variable := &models.TemplateVariable{}
		err := rows.Scan(&variable.ID, &variable.TemplateID, &variable.Key, &variable.DisplayName, &variable.Description, &variable.Required, &variable.DefaultValue, &variable.CreatedAt)
		if err != nil {
			return nil, err
		}
		variables = append(variables, variable)
	}

	return variables, nil
}

func (r *TemplateRepo) DeleteVariablesByTemplateID(templateID string) error {
	_, err := r.db.Exec("DELETE FROM template_variables WHERE template_id = ?", templateID)
	return err
}
