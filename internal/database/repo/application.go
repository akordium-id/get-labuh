package repo

import (
	"database/sql"
	"strings"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

type ApplicationRepo struct {
	db *sql.DB
}

func NewApplicationRepo(db *sql.DB) *ApplicationRepo {
	return &ApplicationRepo{db: db}
}

func (r *ApplicationRepo) Create(input models.CreateApplicationInput) (*models.Application, error) {
	app := &models.Application{
		ID:            uuid.New().String(),
		EnvironmentID: input.EnvironmentID,
		Name:          input.Name,
		Slug:          input.Slug,
		SourceType:    input.SourceType,
		RepositoryURL: input.RepositoryURL,
		Branch:        input.Branch,
		BuildPath:     input.BuildPath,
		DockerfilePath: input.DockerfilePath,
		DockerImage:   input.DockerImage,
		CustomDomain:  input.CustomDomain,
		AppPort:       input.AppPort,
		Status:        models.AppStatusIdle,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if app.Slug == "" {
		app.Slug = GenerateSlug(app.Name)
	}
	if app.Branch == nil || *app.Branch == "" {
		branch := "main"
		app.Branch = &branch
	}
	if app.BuildPath == nil || *app.BuildPath == "" {
		buildPath := "/"
		app.BuildPath = &buildPath
	}
	if app.DockerfilePath == nil || *app.DockerfilePath == "" {
		dockerfilePath := "Dockerfile"
		app.DockerfilePath = &dockerfilePath
	}
	if app.AppPort == 0 {
		app.AppPort = 8080
	}

	containerName := generateContainerName("env", app.Slug, app.ID)
	app.ContainerName = &containerName

	_, err := r.db.Exec(
		`INSERT INTO applications (id, environment_id, name, slug, source_type, repository_url, branch, build_path, dockerfile_path, docker_image, custom_domain, app_port, container_name, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		app.ID, app.EnvironmentID, app.Name, app.Slug, app.SourceType, app.RepositoryURL, app.Branch, app.BuildPath, app.DockerfilePath, app.DockerImage, app.CustomDomain, app.AppPort, app.ContainerName, app.Status, app.CreatedAt, app.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return app, nil
}

func (r *ApplicationRepo) GetByID(id string) (*models.Application, error) {
	app := &models.Application{}
	err := r.db.QueryRow(
		`SELECT id, environment_id, name, slug, source_type, repository_url, branch, build_path, dockerfile_path, docker_image, custom_domain, app_port, container_id, container_name, status, created_at, updated_at
		 FROM applications WHERE id = ?`,
		id,
	).Scan(&app.ID, &app.EnvironmentID, &app.Name, &app.Slug, &app.SourceType, &app.RepositoryURL, &app.Branch, &app.BuildPath, &app.DockerfilePath, &app.DockerImage, &app.CustomDomain, &app.AppPort, &app.ContainerID, &app.ContainerName, &app.Status, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return app, nil
}

func (r *ApplicationRepo) GetBySlug(envID, slug string) (*models.Application, error) {
	app := &models.Application{}
	err := r.db.QueryRow(
		`SELECT id, environment_id, name, slug, source_type, repository_url, branch, build_path, dockerfile_path, docker_image, custom_domain, app_port, container_id, container_name, status, created_at, updated_at
		 FROM applications WHERE environment_id = ? AND slug = ?`,
		envID, slug,
	).Scan(&app.ID, &app.EnvironmentID, &app.Name, &app.Slug, &app.SourceType, &app.RepositoryURL, &app.Branch, &app.BuildPath, &app.DockerfilePath, &app.DockerImage, &app.CustomDomain, &app.AppPort, &app.ContainerID, &app.ContainerName, &app.Status, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return app, nil
}

func (r *ApplicationRepo) GetByContainerName(containerName string) (*models.Application, error) {
	app := &models.Application{}
	err := r.db.QueryRow(
		`SELECT id, environment_id, name, slug, source_type, repository_url, branch, build_path, dockerfile_path, docker_image, custom_domain, app_port, container_id, container_name, status, created_at, updated_at
		 FROM applications WHERE container_name = ?`,
		containerName,
	).Scan(&app.ID, &app.EnvironmentID, &app.Name, &app.Slug, &app.SourceType, &app.RepositoryURL, &app.Branch, &app.BuildPath, &app.DockerfilePath, &app.DockerImage, &app.CustomDomain, &app.AppPort, &app.ContainerID, &app.ContainerName, &app.Status, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return app, nil
}

func (r *ApplicationRepo) GetByEnvironmentID(envID string) ([]*models.Application, error) {
	rows, err := r.db.Query(
		`SELECT id, environment_id, name, slug, source_type, repository_url, branch, build_path, dockerfile_path, docker_image, custom_domain, app_port, container_id, container_name, status, created_at, updated_at
		 FROM applications WHERE environment_id = ? ORDER BY created_at DESC`,
		envID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []*models.Application
	for rows.Next() {
		app := &models.Application{}
		err := rows.Scan(&app.ID, &app.EnvironmentID, &app.Name, &app.Slug, &app.SourceType, &app.RepositoryURL, &app.Branch, &app.BuildPath, &app.DockerfilePath, &app.DockerImage, &app.CustomDomain, &app.AppPort, &app.ContainerID, &app.ContainerName, &app.Status, &app.CreatedAt, &app.UpdatedAt)
		if err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}

	return apps, nil
}

func (r *ApplicationRepo) UpdateStatus(id string, status models.AppStatus) error {
	_, err := r.db.Exec(
		`UPDATE applications SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	return err
}

func (r *ApplicationRepo) UpdateContainer(id, containerID string) error {
	_, err := r.db.Exec(
		`UPDATE applications SET container_id = ?, updated_at = ? WHERE id = ?`,
		containerID, time.Now(), id,
	)
	return err
}

func (r *ApplicationRepo) UpdateContainerAndStatus(id, containerID string, status models.AppStatus) error {
	_, err := r.db.Exec(
		`UPDATE applications SET container_id = ?, status = ?, updated_at = ? WHERE id = ?`,
		containerID, status, time.Now(), id,
	)
	return err
}

func (r *ApplicationRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM applications WHERE id = ?", id)
	return err
}

func generateContainerName(envSlug, appSlug, appID string) string {
	shortID := appID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	slug := strings.ToLower(appSlug)
	slug = strings.ReplaceAll(slug, " ", "-")
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	return "labuh-" + envSlug + "-" + result.String() + "-" + shortID
}
