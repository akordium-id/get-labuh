package repo

import (
	"database/sql"
	"strings"
	"time"

	"github.com/faiq/labuh/internal/models"
	"github.com/google/uuid"
)

type ProjectRepo struct {
	db *sql.DB
}

func NewProjectRepo(db *sql.DB) *ProjectRepo {
	return &ProjectRepo{db: db}
}

func (r *ProjectRepo) Create(input models.CreateProjectInput) (*models.Project, error) {
	project := &models.Project{
		ID:          uuid.New().String(),
		Name:        input.Name,
		Slug:        input.Slug,
		Description: input.Description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err := r.db.Exec(
		"INSERT INTO projects (id, name, slug, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		project.ID, project.Name, project.Slug, project.Description, project.CreatedAt, project.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func (r *ProjectRepo) GetByID(id string) (*models.Project, error) {
	project := &models.Project{}
	err := r.db.QueryRow(
		"SELECT id, name, slug, description, created_at, updated_at FROM projects WHERE id = ?",
		id,
	).Scan(&project.ID, &project.Name, &project.Slug, &project.Description, &project.CreatedAt, &project.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func (r *ProjectRepo) GetBySlug(slug string) (*models.Project, error) {
	project := &models.Project{}
	err := r.db.QueryRow(
		"SELECT id, name, slug, description, created_at, updated_at FROM projects WHERE slug = ?",
		slug,
	).Scan(&project.ID, &project.Name, &project.Slug, &project.Description, &project.CreatedAt, &project.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return project, nil
}

func (r *ProjectRepo) GetAll() ([]*models.Project, error) {
	rows, err := r.db.Query(
		"SELECT id, name, slug, description, created_at, updated_at FROM projects ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []*models.Project
	for rows.Next() {
		project := &models.Project{}
		err := rows.Scan(&project.ID, &project.Name, &project.Slug, &project.Description, &project.CreatedAt, &project.UpdatedAt)
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}

	return projects, nil
}

func (r *ProjectRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM projects WHERE id = ?", id)
	return err
}

func (r *ProjectRepo) CreateEnvironment(projectID, name, slug string) (*models.Environment, error) {
	env := &models.Environment{
		ID:        uuid.New().String(),
		ProjectID: projectID,
		Name:      name,
		Slug:      slug,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err := r.db.Exec(
		"INSERT INTO environments (id, project_id, name, slug, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		env.ID, env.ProjectID, env.Name, env.Slug, env.CreatedAt, env.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return env, nil
}

func (r *ProjectRepo) GetEnvironmentsByProjectID(projectID string) ([]*models.Environment, error) {
	rows, err := r.db.Query(
		"SELECT id, project_id, name, slug, created_at, updated_at FROM environments WHERE project_id = ? ORDER BY created_at DESC",
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var environments []*models.Environment
	for rows.Next() {
		env := &models.Environment{}
		err := rows.Scan(&env.ID, &env.ProjectID, &env.Name, &env.Slug, &env.CreatedAt, &env.UpdatedAt)
		if err != nil {
			return nil, err
		}
		environments = append(environments, env)
	}

	return environments, nil
}

func GenerateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return result.String()
}
