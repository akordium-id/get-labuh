package applications

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"strings"
	"time"

	"github.com/akordium-id/get-labuh/internal/encryption"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

// Repo menangani operasi data untuk model Application.
type Repo struct {
	db *sql.DB
}

// NewRepo membuat instance baru Repo aplikasi.
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// CreateWithContext membuat aplikasi baru dengan context.
func (r *Repo) CreateWithContext(ctx context.Context, input models.CreateApplicationInput) (*models.Application, error) {
	app := &models.Application{
		ID:                 uuid.New().String(),
		EnvironmentID:      input.EnvironmentID,
		Name:               input.Name,
		Slug:               input.Slug,
		SourceType:         input.SourceType,
		RepositoryURL:      input.RepositoryURL,
		Branch:             input.Branch,
		BuildPath:          input.BuildPath,
		DockerfilePath:     input.DockerfilePath,
		DockerImage:        input.DockerImage,
		CustomDomain:       input.CustomDomain,
		AppPort:            input.AppPort,
		Status:             models.AppStatusIdle,
		HealthcheckPath:    input.HealthcheckPath,
		HealthcheckTimeout: input.HealthcheckTimeout,
		BasicAuthUser:      input.BasicAuthUser,
		BasicAuthPassword:  input.BasicAuthPassword,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
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
	if app.HealthcheckPath == nil || *app.HealthcheckPath == "" {
		hp := "/"
		app.HealthcheckPath = &hp
	}
	if app.HealthcheckTimeout == nil || *app.HealthcheckTimeout <= 0 {
		ht := 30
		app.HealthcheckTimeout = &ht
	}
	if app.WebhookSecret == nil || *app.WebhookSecret == "" {
		if secret, err := GenerateRandomSecret(32); err == nil {
			app.WebhookSecret = &secret
		}
	}

	containerName := generateContainerName("env", app.Slug, app.ID)
	app.ContainerName = &containerName

	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO applications (id, environment_id, name, slug, source_type, repository_url, branch, build_path, dockerfile_path, docker_image, custom_domain, app_port, container_name, status, webhook_secret, cpu_limit, memory_limit, memory_reservation, healthcheck_path, healthcheck_timeout, basic_auth_user, basic_auth_password, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		app.ID, app.EnvironmentID, app.Name, app.Slug, app.SourceType, app.RepositoryURL, app.Branch, app.BuildPath, app.DockerfilePath, app.DockerImage, app.CustomDomain, app.AppPort, app.ContainerName, app.Status, app.WebhookSecret, input.CPULimit, input.MemoryLimit, input.MemoryReservation, app.HealthcheckPath, app.HealthcheckTimeout, app.BasicAuthUser, app.BasicAuthPassword, app.CreatedAt, app.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	app.CPULimit = input.CPULimit
	app.MemoryLimit = input.MemoryLimit
	app.MemoryReservation = input.MemoryReservation

	return app, nil
}

func (r *Repo) Create(input models.CreateApplicationInput) (*models.Application, error) {
	return r.CreateWithContext(context.Background(), input)
}

// GetByIDWithContext mengambil aplikasi berdasarkan ID dengan context.
func (r *Repo) GetByIDWithContext(ctx context.Context, id string) (*models.Application, error) {
	app := &models.Application{}
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, environment_id, name, slug, source_type, repository_url, branch, build_path, dockerfile_path, docker_image, custom_domain, app_port, container_id, container_name, status, webhook_secret, cpu_limit, memory_limit, memory_reservation, healthcheck_path, healthcheck_timeout, basic_auth_user, basic_auth_password, created_at, updated_at
		 FROM applications WHERE id = ?`,
		id,
	).Scan(&app.ID, &app.EnvironmentID, &app.Name, &app.Slug, &app.SourceType, &app.RepositoryURL, &app.Branch, &app.BuildPath, &app.DockerfilePath, &app.DockerImage, &app.CustomDomain, &app.AppPort, &app.ContainerID, &app.ContainerName, &app.Status, &app.WebhookSecret, &app.CPULimit, &app.MemoryLimit, &app.MemoryReservation, &app.HealthcheckPath, &app.HealthcheckTimeout, &app.BasicAuthUser, &app.BasicAuthPassword, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return app, nil
}

func (r *Repo) GetByID(id string) (*models.Application, error) {
	return r.GetByIDWithContext(context.Background(), id)
}

// GetBySlugWithContext mengambil aplikasi berdasarkan environment dan slug dengan context.
func (r *Repo) GetBySlugWithContext(ctx context.Context, envID, slug string) (*models.Application, error) {
	app := &models.Application{}
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, environment_id, name, slug, source_type, repository_url, branch, build_path, dockerfile_path, docker_image, custom_domain, app_port, container_id, container_name, status, webhook_secret, cpu_limit, memory_limit, memory_reservation, healthcheck_path, healthcheck_timeout, basic_auth_user, basic_auth_password, created_at, updated_at
		 FROM applications WHERE environment_id = ? AND slug = ?`,
		envID, slug,
	).Scan(&app.ID, &app.EnvironmentID, &app.Name, &app.Slug, &app.SourceType, &app.RepositoryURL, &app.Branch, &app.BuildPath, &app.DockerfilePath, &app.DockerImage, &app.CustomDomain, &app.AppPort, &app.ContainerID, &app.ContainerName, &app.Status, &app.WebhookSecret, &app.CPULimit, &app.MemoryLimit, &app.MemoryReservation, &app.HealthcheckPath, &app.HealthcheckTimeout, &app.BasicAuthUser, &app.BasicAuthPassword, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return app, nil
}

func (r *Repo) GetBySlug(envID, slug string) (*models.Application, error) {
	return r.GetBySlugWithContext(context.Background(), envID, slug)
}

// GetByContainerNameWithContext mengambil aplikasi berdasarkan nama container dengan context.
func (r *Repo) GetByContainerNameWithContext(ctx context.Context, containerName string) (*models.Application, error) {
	app := &models.Application{}
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, environment_id, name, slug, source_type, repository_url, branch, build_path, dockerfile_path, docker_image, custom_domain, app_port, container_id, container_name, status, webhook_secret, cpu_limit, memory_limit, memory_reservation, healthcheck_path, healthcheck_timeout, basic_auth_user, basic_auth_password, created_at, updated_at
		 FROM applications WHERE container_name = ?`,
		containerName,
	).Scan(&app.ID, &app.EnvironmentID, &app.Name, &app.Slug, &app.SourceType, &app.RepositoryURL, &app.Branch, &app.BuildPath, &app.DockerfilePath, &app.DockerImage, &app.CustomDomain, &app.AppPort, &app.ContainerID, &app.ContainerName, &app.Status, &app.WebhookSecret, &app.CPULimit, &app.MemoryLimit, &app.MemoryReservation, &app.HealthcheckPath, &app.HealthcheckTimeout, &app.BasicAuthUser, &app.BasicAuthPassword, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return app, nil
}

func (r *Repo) GetByContainerName(containerName string) (*models.Application, error) {
	return r.GetByContainerNameWithContext(context.Background(), containerName)
}

// GetByEnvironmentIDWithContext mengambil seluruh aplikasi dalam environment dengan context.
func (r *Repo) GetByEnvironmentIDWithContext(ctx context.Context, envID string) ([]*models.Application, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, environment_id, name, slug, source_type, repository_url, branch, build_path, dockerfile_path, docker_image, custom_domain, app_port, container_id, container_name, status, webhook_secret, cpu_limit, memory_limit, memory_reservation, healthcheck_path, healthcheck_timeout, basic_auth_user, basic_auth_password, created_at, updated_at
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
		err := rows.Scan(&app.ID, &app.EnvironmentID, &app.Name, &app.Slug, &app.SourceType, &app.RepositoryURL, &app.Branch, &app.BuildPath, &app.DockerfilePath, &app.DockerImage, &app.CustomDomain, &app.AppPort, &app.ContainerID, &app.ContainerName, &app.Status, &app.WebhookSecret, &app.CPULimit, &app.MemoryLimit, &app.MemoryReservation, &app.HealthcheckPath, &app.HealthcheckTimeout, &app.BasicAuthUser, &app.BasicAuthPassword, &app.CreatedAt, &app.UpdatedAt)
		if err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}

	return apps, rows.Err()
}

func (r *Repo) GetByEnvironmentID(envID string) ([]*models.Application, error) {
	return r.GetByEnvironmentIDWithContext(context.Background(), envID)
}

// GetAllWithContext mengambil seluruh aplikasi dengan context.
func (r *Repo) GetAllWithContext(ctx context.Context) ([]*models.Application, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, environment_id, name, slug, source_type, repository_url, branch, build_path, dockerfile_path, docker_image, custom_domain, app_port, container_id, container_name, status, webhook_secret, cpu_limit, memory_limit, memory_reservation, healthcheck_path, healthcheck_timeout, basic_auth_user, basic_auth_password, created_at, updated_at
		 FROM applications ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []*models.Application
	for rows.Next() {
		app := &models.Application{}
		err := rows.Scan(&app.ID, &app.EnvironmentID, &app.Name, &app.Slug, &app.SourceType, &app.RepositoryURL, &app.Branch, &app.BuildPath, &app.DockerfilePath, &app.DockerImage, &app.CustomDomain, &app.AppPort, &app.ContainerID, &app.ContainerName, &app.Status, &app.WebhookSecret, &app.CPULimit, &app.MemoryLimit, &app.MemoryReservation, &app.HealthcheckPath, &app.HealthcheckTimeout, &app.BasicAuthUser, &app.BasicAuthPassword, &app.CreatedAt, &app.UpdatedAt)
		if err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}

	return apps, rows.Err()
}

func (r *Repo) GetAll() ([]*models.Application, error) {
	return r.GetAllWithContext(context.Background())
}

// GetByEnvironmentIDPaginatedWithContext mengambil aplikasi berhalaman dengan context.
func (r *Repo) GetByEnvironmentIDPaginatedWithContext(ctx context.Context, envID string, limit, offset int) ([]*models.Application, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM applications WHERE environment_id = ?", envID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, environment_id, name, slug, source_type, repository_url, branch, build_path, dockerfile_path, docker_image, custom_domain, app_port, container_id, container_name, status, webhook_secret, cpu_limit, memory_limit, memory_reservation, healthcheck_path, healthcheck_timeout, basic_auth_user, basic_auth_password, created_at, updated_at
		 FROM applications WHERE environment_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		envID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var apps []*models.Application
	for rows.Next() {
		app := &models.Application{}
		err := rows.Scan(&app.ID, &app.EnvironmentID, &app.Name, &app.Slug, &app.SourceType, &app.RepositoryURL, &app.Branch, &app.BuildPath, &app.DockerfilePath, &app.DockerImage, &app.CustomDomain, &app.AppPort, &app.ContainerID, &app.ContainerName, &app.Status, &app.WebhookSecret, &app.CPULimit, &app.MemoryLimit, &app.MemoryReservation, &app.HealthcheckPath, &app.HealthcheckTimeout, &app.BasicAuthUser, &app.BasicAuthPassword, &app.CreatedAt, &app.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}
		apps = append(apps, app)
	}

	return apps, total, rows.Err()
}

func (r *Repo) GetByEnvironmentIDPaginated(envID string, limit, offset int) ([]*models.Application, int, error) {
	return r.GetByEnvironmentIDPaginatedWithContext(context.Background(), envID, limit, offset)
}

// SearchPaginatedWithContext mencari aplikasi berhalaman dengan context.
func (r *Repo) SearchPaginatedWithContext(ctx context.Context, envID, search string, limit, offset int) ([]*models.Application, int, error) {
	var total int
	searchPattern := "%" + search + "%"
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM applications WHERE environment_id = ? AND (name LIKE ? OR slug LIKE ?)", envID, searchPattern, searchPattern).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, environment_id, name, slug, source_type, repository_url, branch, build_path, dockerfile_path, docker_image, custom_domain, app_port, container_id, container_name, status, webhook_secret, cpu_limit, memory_limit, memory_reservation, healthcheck_path, healthcheck_timeout, basic_auth_user, basic_auth_password, created_at, updated_at
		 FROM applications WHERE environment_id = ? AND (name LIKE ? OR slug LIKE ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		envID, searchPattern, searchPattern, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var apps []*models.Application
	for rows.Next() {
		app := &models.Application{}
		err := rows.Scan(&app.ID, &app.EnvironmentID, &app.Name, &app.Slug, &app.SourceType, &app.RepositoryURL, &app.Branch, &app.BuildPath, &app.DockerfilePath, &app.DockerImage, &app.CustomDomain, &app.AppPort, &app.ContainerID, &app.ContainerName, &app.Status, &app.WebhookSecret, &app.CPULimit, &app.MemoryLimit, &app.MemoryReservation, &app.HealthcheckPath, &app.HealthcheckTimeout, &app.BasicAuthUser, &app.BasicAuthPassword, &app.CreatedAt, &app.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}
		apps = append(apps, app)
	}

	return apps, total, rows.Err()
}

func (r *Repo) SearchPaginated(envID, search string, limit, offset int) ([]*models.Application, int, error) {
	return r.SearchPaginatedWithContext(context.Background(), envID, search, limit, offset)
}

// GetByStatusWithContext mengambil aplikasi berdasarkan status dengan context.
func (r *Repo) GetByStatusWithContext(ctx context.Context, status models.AppStatus) ([]*models.Application, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, environment_id, name, slug, source_type, repository_url, branch, build_path, dockerfile_path, docker_image, custom_domain, app_port, container_id, container_name, status, webhook_secret, cpu_limit, memory_limit, memory_reservation, healthcheck_path, healthcheck_timeout, basic_auth_user, basic_auth_password, created_at, updated_at
		 FROM applications WHERE status = ? ORDER BY created_at DESC`,
		status,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apps []*models.Application
	for rows.Next() {
		app := &models.Application{}
		err := rows.Scan(&app.ID, &app.EnvironmentID, &app.Name, &app.Slug, &app.SourceType, &app.RepositoryURL, &app.Branch, &app.BuildPath, &app.DockerfilePath, &app.DockerImage, &app.CustomDomain, &app.AppPort, &app.ContainerID, &app.ContainerName, &app.Status, &app.WebhookSecret, &app.CPULimit, &app.MemoryLimit, &app.MemoryReservation, &app.HealthcheckPath, &app.HealthcheckTimeout, &app.BasicAuthUser, &app.BasicAuthPassword, &app.CreatedAt, &app.UpdatedAt)
		if err != nil {
			return nil, err
		}
		apps = append(apps, app)
	}

	return apps, rows.Err()
}

func (r *Repo) GetByStatus(status models.AppStatus) ([]*models.Application, error) {
	return r.GetByStatusWithContext(context.Background(), status)
}

// GetByCustomDomainWithContext mengambil aplikasi berdasarkan domain dengan context.
func (r *Repo) GetByCustomDomainWithContext(ctx context.Context, domain string) (*models.Application, error) {
	app := &models.Application{}
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, environment_id, name, slug, source_type, repository_url, branch, build_path, dockerfile_path, docker_image, custom_domain, app_port, container_id, container_name, status, webhook_secret, cpu_limit, memory_limit, memory_reservation, healthcheck_path, healthcheck_timeout, basic_auth_user, basic_auth_password, created_at, updated_at
		 FROM applications WHERE custom_domain = ?`,
		domain,
	).Scan(&app.ID, &app.EnvironmentID, &app.Name, &app.Slug, &app.SourceType, &app.RepositoryURL, &app.Branch, &app.BuildPath, &app.DockerfilePath, &app.DockerImage, &app.CustomDomain, &app.AppPort, &app.ContainerID, &app.ContainerName, &app.Status, &app.WebhookSecret, &app.CPULimit, &app.MemoryLimit, &app.MemoryReservation, &app.HealthcheckPath, &app.HealthcheckTimeout, &app.BasicAuthUser, &app.BasicAuthPassword, &app.CreatedAt, &app.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return app, nil
}

func (r *Repo) GetByCustomDomain(domain string) (*models.Application, error) {
	return r.GetByCustomDomainWithContext(context.Background(), domain)
}

// UpdateStatusWithContext memperbarui status aplikasi dengan context.
func (r *Repo) UpdateStatusWithContext(ctx context.Context, id string, status models.AppStatus) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE applications SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	return err
}

func (r *Repo) UpdateStatus(id string, status models.AppStatus) error {
	return r.UpdateStatusWithContext(context.Background(), id, status)
}

// UpdateContainerWithContext memperbarui container ID dengan context.
func (r *Repo) UpdateContainerWithContext(ctx context.Context, id, containerID string) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE applications SET container_id = ?, updated_at = ? WHERE id = ?`,
		containerID, time.Now(), id,
	)
	return err
}

func (r *Repo) UpdateContainer(id, containerID string) error {
	return r.UpdateContainerWithContext(context.Background(), id, containerID)
}

// UpdateContainerAndStatusWithContext memperbarui container ID dan status dengan context.
func (r *Repo) UpdateContainerAndStatusWithContext(ctx context.Context, id, containerID string, status models.AppStatus) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE applications SET container_id = ?, status = ?, updated_at = ? WHERE id = ?`,
		containerID, status, time.Now(), id,
	)
	return err
}

func (r *Repo) UpdateContainerAndStatus(id, containerID string, status models.AppStatus) error {
	return r.UpdateContainerAndStatusWithContext(context.Background(), id, containerID, status)
}

// DeleteWithContext menghapus aplikasi dengan context.
func (r *Repo) DeleteWithContext(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM applications WHERE id = ?", id)
	return err
}

func (r *Repo) Delete(id string) error {
	return r.DeleteWithContext(context.Background(), id)
}

// CloneWithContext mengkloning konfigurasi aplikasi ke environment target dengan context.
func (r *Repo) CloneWithContext(ctx context.Context, srcApp *models.Application, targetEnvID, newName string) (*models.Application, error) {
	app := &models.Application{
		ID:             uuid.New().String(),
		EnvironmentID:  targetEnvID,
		Name:           newName,
		Slug:           GenerateSlug(newName),
		SourceType:     srcApp.SourceType,
		RepositoryURL:  srcApp.RepositoryURL,
		Branch:         srcApp.Branch,
		BuildPath:      srcApp.BuildPath,
		DockerfilePath: srcApp.DockerfilePath,
		DockerImage:    srcApp.DockerImage,
		CustomDomain:   srcApp.CustomDomain,
		AppPort:        srcApp.AppPort,
		Status:         models.AppStatusIdle,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	containerName := generateContainerName("env", app.Slug, app.ID)
	app.ContainerName = &containerName

	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO applications (id, environment_id, name, slug, source_type, repository_url, branch, build_path, dockerfile_path, docker_image, custom_domain, app_port, container_name, status, webhook_secret, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		app.ID, app.EnvironmentID, app.Name, app.Slug, app.SourceType, app.RepositoryURL, app.Branch, app.BuildPath, app.DockerfilePath, app.DockerImage, app.CustomDomain, app.AppPort, app.ContainerName, app.Status, srcApp.WebhookSecret, app.CreatedAt, app.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return app, nil
}

func (r *Repo) Clone(srcApp *models.Application, targetEnvID, newName string) (*models.Application, error) {
	return r.CloneWithContext(context.Background(), srcApp, targetEnvID, newName)
}

// MoveWithContext memindahkan aplikasi ke environment lain dengan context.
func (r *Repo) MoveWithContext(ctx context.Context, id, targetEnvID string) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE applications SET environment_id = ?, updated_at = ? WHERE id = ?`,
		targetEnvID, time.Now(), id,
	)
	return err
}

func (r *Repo) Move(id, targetEnvID string) error {
	return r.MoveWithContext(context.Background(), id, targetEnvID)
}

// UpdateCustomDomainWithContext memperbarui domain kustom aplikasi dengan context.
func (r *Repo) UpdateCustomDomainWithContext(ctx context.Context, id string, customDomain *string) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE applications SET custom_domain = ?, updated_at = ? WHERE id = ?`,
		customDomain, time.Now(), id,
	)
	return err
}

func (r *Repo) UpdateCustomDomain(id string, customDomain *string) error {
	return r.UpdateCustomDomainWithContext(context.Background(), id, customDomain)
}

// UpdateWebhookSecretWithContext memperbarui webhook secret dengan context.
func (r *Repo) UpdateWebhookSecretWithContext(ctx context.Context, id string, secret *string) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE applications SET webhook_secret = ?, updated_at = ? WHERE id = ?`,
		secret, time.Now(), id,
	)
	return err
}

func (r *Repo) UpdateWebhookSecret(id string, secret *string) error {
	return r.UpdateWebhookSecretWithContext(context.Background(), id, secret)
}

// UpdateResourceLimitsWithContext memperbarui batasan CPU dan RAM aplikasi dengan context.
func (r *Repo) UpdateResourceLimitsWithContext(ctx context.Context, id string, cpuLimit *float64, memoryLimit, memoryReservation *int64) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE applications SET cpu_limit = ?, memory_limit = ?, memory_reservation = ?, updated_at = ? WHERE id = ?`,
		cpuLimit, memoryLimit, memoryReservation, time.Now(), id,
	)
	return err
}

func (r *Repo) UpdateResourceLimits(id string, cpuLimit *float64, memoryLimit, memoryReservation *int64) error {
	return r.UpdateResourceLimitsWithContext(context.Background(), id, cpuLimit, memoryLimit, memoryReservation)
}

// UpdateHealthcheckWithContext memperbarui path dan timeout healthcheck aplikasi dengan context.
func (r *Repo) UpdateHealthcheckWithContext(ctx context.Context, id string, path *string, timeout *int) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE applications SET healthcheck_path = ?, healthcheck_timeout = ?, updated_at = ? WHERE id = ?`,
		path, timeout, time.Now(), id,
	)
	return err
}

func (r *Repo) UpdateHealthcheck(id string, path *string, timeout *int) error {
	return r.UpdateHealthcheckWithContext(context.Background(), id, path, timeout)
}

// UpdateBasicAuthWithContext memperbarui basic auth username dan password dengan context.
func (r *Repo) UpdateBasicAuthWithContext(ctx context.Context, id string, user, password *string) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE applications SET basic_auth_user = ?, basic_auth_password = ?, updated_at = ? WHERE id = ?`,
		user, password, time.Now(), id,
	)
	return err
}

func (r *Repo) UpdateBasicAuth(id string, user, password *string) error {
	return r.UpdateBasicAuthWithContext(context.Background(), id, user, password)
}

// EnvVarRepo mengelola environment variables aplikasi.
type EnvVarRepo struct {
	db *sql.DB
}

// NewEnvVarRepo membuat instance baru EnvVarRepo.
func NewEnvVarRepo(db *sql.DB) *EnvVarRepo {
	return &EnvVarRepo{db: db}
}

// CreateWithContext membuat variabel lingkungan baru dengan context.
func (r *EnvVarRepo) CreateWithContext(ctx context.Context, input models.CreateAppEnvVarInput) (*models.AppEnvVar, error) {
	value := input.Value
	if input.IsSecret && encryption.IsInitialized() {
		encrypted, err := encryption.Encrypt(value)
		if err != nil {
			return nil, err
		}
		value = encryption.EncryptedValue(encrypted)
	}

	envVar := &models.AppEnvVar{
		ID:            uuid.New().String(),
		ApplicationID: input.ApplicationID,
		Key:           input.Key,
		Value:         value,
		IsSecret:      input.IsSecret,
		IsEncrypted:   input.IsSecret && encryption.IsInitialized(),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO app_env_vars (id, application_id, key, value, is_secret, is_encrypted, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		envVar.ID, envVar.ApplicationID, envVar.Key, envVar.Value, envVar.IsSecret, envVar.IsEncrypted, envVar.CreatedAt, envVar.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return envVar, nil
}

func (r *EnvVarRepo) Create(input models.CreateAppEnvVarInput) (*models.AppEnvVar, error) {
	return r.CreateWithContext(context.Background(), input)
}

// GetByIDWithContext mengambil variabel lingkungan berdasarkan ID dengan context.
func (r *EnvVarRepo) GetByIDWithContext(ctx context.Context, id string) (*models.AppEnvVar, error) {
	envVar := &models.AppEnvVar{}
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, application_id, key, value, is_secret, is_encrypted, created_at, updated_at
		 FROM app_env_vars WHERE id = ?`,
		id,
	).Scan(&envVar.ID, &envVar.ApplicationID, &envVar.Key, &envVar.Value, &envVar.IsSecret, &envVar.IsEncrypted, &envVar.CreatedAt, &envVar.UpdatedAt)
	if err != nil {
		return nil, err
	}

	if envVar.IsEncrypted && encryption.IsInitialized() {
		decrypted, err := encryption.Decrypt(strings.TrimPrefix(envVar.Value, "ENC:"))
		if err != nil {
			return nil, err
		}
		envVar.Value = decrypted
	}

	return envVar, nil
}

func (r *EnvVarRepo) GetByID(id string) (*models.AppEnvVar, error) {
	return r.GetByIDWithContext(context.Background(), id)
}

// GetByApplicationIDWithContext mengambil seluruh variabel lingkungan aplikasi dengan context.
func (r *EnvVarRepo) GetByApplicationIDWithContext(ctx context.Context, appID string) ([]*models.AppEnvVar, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, application_id, key, value, is_secret, is_encrypted, created_at, updated_at
		 FROM app_env_vars WHERE application_id = ? ORDER BY created_at DESC`,
		appID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var envVars []*models.AppEnvVar
	for rows.Next() {
		envVar := &models.AppEnvVar{}
		err := rows.Scan(&envVar.ID, &envVar.ApplicationID, &envVar.Key, &envVar.Value, &envVar.IsSecret, &envVar.IsEncrypted, &envVar.CreatedAt, &envVar.UpdatedAt)
		if err != nil {
			return nil, err
		}

		if envVar.IsEncrypted && encryption.IsInitialized() {
			decrypted, err := encryption.Decrypt(strings.TrimPrefix(envVar.Value, "ENC:"))
			if err != nil {
				return nil, err
			}
			envVar.Value = decrypted
		}

		envVars = append(envVars, envVar)
	}

	return envVars, rows.Err()
}

func (r *EnvVarRepo) GetByApplicationID(appID string) ([]*models.AppEnvVar, error) {
	return r.GetByApplicationIDWithContext(context.Background(), appID)
}

// DeleteWithContext menghapus variabel lingkungan dengan context.
func (r *EnvVarRepo) DeleteWithContext(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM app_env_vars WHERE id = ?", id)
	return err
}

func (r *EnvVarRepo) Delete(id string) error {
	return r.DeleteWithContext(context.Background(), id)
}

// GenerateRandomSecret membuat secret acak dengan panjang tertentu.
func GenerateRandomSecret(byteLen int) (string, error) {
	b := make([]byte, byteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// GenerateSlug mengonversi nama menjadi format slug yang valid.
func GenerateSlug(name string) string {
	slug := strings.ToLower(name)
	slug = strings.ReplaceAll(slug, " ", "-")
	var result strings.Builder
	for _, r := range slug {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	res := result.String()
	for strings.Contains(res, "--") {
		res = strings.ReplaceAll(res, "--", "-")
	}
	return strings.Trim(res, "-")
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
