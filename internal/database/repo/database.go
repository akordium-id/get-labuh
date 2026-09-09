package repo

import (
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

type DatabaseRepo struct {
	db *sql.DB
}

func NewDatabaseRepo(db *sql.DB) *DatabaseRepo {
	return &DatabaseRepo{db: db}
}

func (r *DatabaseRepo) Create(input models.CreateDatabaseServiceInput) (*models.DatabaseService, error) {
	dbService := &models.DatabaseService{
		ID:            uuid.New().String(),
		EnvironmentID: input.EnvironmentID,
		Name:          input.Name,
		Slug:          input.Slug,
		Engine:        input.Engine,
		Version:       input.Version,
		Username:      input.Username,
		Password:      input.Password,
		DatabaseName:  input.DatabaseName,
		Port:          input.Port,
		Status:        models.DatabaseStatusIdle,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if dbService.Slug == "" {
		dbService.Slug = GenerateSlug(dbService.Name)
	}
	if dbService.Version == "" {
		dbService.Version = "latest"
	}
	if dbService.Port == 0 {
		switch dbService.Engine {
		case models.DatabaseEnginePostgres:
			dbService.Port = 5432
		case models.DatabaseEngineMySQL:
			dbService.Port = 3306
		case models.DatabaseEngineRedis:
			dbService.Port = 6379
		case models.DatabaseEngineMongoDB:
			dbService.Port = 27017
		}
	}

	_, err := r.db.Exec(
		`INSERT INTO database_services (id, environment_id, name, slug, engine, version, username, password, database_name, port, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dbService.ID, dbService.EnvironmentID, dbService.Name, dbService.Slug, dbService.Engine, dbService.Version, dbService.Username, dbService.Password, dbService.DatabaseName, dbService.Port, dbService.Status, dbService.CreatedAt, dbService.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return dbService, nil
}

func (r *DatabaseRepo) GetByID(id string) (*models.DatabaseService, error) {
	dbService := &models.DatabaseService{}
	err := r.db.QueryRow(
		`SELECT id, environment_id, name, slug, engine, version, username, password, database_name, port, container_id, container_name, status, connection_string, created_at, updated_at
		 FROM database_services WHERE id = ?`,
		id,
	).Scan(&dbService.ID, &dbService.EnvironmentID, &dbService.Name, &dbService.Slug, &dbService.Engine, &dbService.Version, &dbService.Username, &dbService.Password, &dbService.DatabaseName, &dbService.Port, &dbService.ContainerID, &dbService.ContainerName, &dbService.Status, &dbService.ConnectionString, &dbService.CreatedAt, &dbService.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return dbService, nil
}

func (r *DatabaseRepo) GetBySlug(envID, slug string) (*models.DatabaseService, error) {
	dbService := &models.DatabaseService{}
	err := r.db.QueryRow(
		`SELECT id, environment_id, name, slug, engine, version, username, password, database_name, port, container_id, container_name, status, connection_string, created_at, updated_at
		 FROM database_services WHERE environment_id = ? AND slug = ?`,
		envID, slug,
	).Scan(&dbService.ID, &dbService.EnvironmentID, &dbService.Name, &dbService.Slug, &dbService.Engine, &dbService.Version, &dbService.Username, &dbService.Password, &dbService.DatabaseName, &dbService.Port, &dbService.ContainerID, &dbService.ContainerName, &dbService.Status, &dbService.ConnectionString, &dbService.CreatedAt, &dbService.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return dbService, nil
}

func (r *DatabaseRepo) GetByEnvironmentID(envID string) ([]*models.DatabaseService, error) {
	rows, err := r.db.Query(
		`SELECT id, environment_id, name, slug, engine, version, username, password, database_name, port, container_id, container_name, status, connection_string, created_at, updated_at
		 FROM database_services WHERE environment_id = ? ORDER BY created_at DESC`,
		envID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dbServices []*models.DatabaseService
	for rows.Next() {
		dbService := &models.DatabaseService{}
		err := rows.Scan(&dbService.ID, &dbService.EnvironmentID, &dbService.Name, &dbService.Slug, &dbService.Engine, &dbService.Version, &dbService.Username, &dbService.Password, &dbService.DatabaseName, &dbService.Port, &dbService.ContainerID, &dbService.ContainerName, &dbService.Status, &dbService.ConnectionString, &dbService.CreatedAt, &dbService.UpdatedAt)
		if err != nil {
			return nil, err
		}
		dbServices = append(dbServices, dbService)
	}

	return dbServices, nil
}

func (r *DatabaseRepo) GetByEnvironmentIDPaginated(envID string, limit, offset int) ([]*models.DatabaseService, int, error) {
	var total int
	err := r.db.QueryRow("SELECT COUNT(*) FROM database_services WHERE environment_id = ?", envID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(
		`SELECT id, environment_id, name, slug, engine, version, username, password, database_name, port, container_id, container_name, status, connection_string, created_at, updated_at
		 FROM database_services WHERE environment_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		envID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var dbServices []*models.DatabaseService
	for rows.Next() {
		dbService := &models.DatabaseService{}
		err := rows.Scan(&dbService.ID, &dbService.EnvironmentID, &dbService.Name, &dbService.Slug, &dbService.Engine, &dbService.Version, &dbService.Username, &dbService.Password, &dbService.DatabaseName, &dbService.Port, &dbService.ContainerID, &dbService.ContainerName, &dbService.Status, &dbService.ConnectionString, &dbService.CreatedAt, &dbService.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}
		dbServices = append(dbServices, dbService)
	}

	return dbServices, total, nil
}

func (r *DatabaseRepo) SearchPaginated(envID, search string, limit, offset int) ([]*models.DatabaseService, int, error) {
	var total int
	searchPattern := "%" + search + "%"
	err := r.db.QueryRow("SELECT COUNT(*) FROM database_services WHERE environment_id = ? AND (name LIKE ? OR slug LIKE ?)", envID, searchPattern, searchPattern).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(
		`SELECT id, environment_id, name, slug, engine, version, username, password, database_name, port, container_id, container_name, status, connection_string, created_at, updated_at
		 FROM database_services WHERE environment_id = ? AND (name LIKE ? OR slug LIKE ?) ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		envID, searchPattern, searchPattern, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var dbServices []*models.DatabaseService
	for rows.Next() {
		dbService := &models.DatabaseService{}
		err := rows.Scan(&dbService.ID, &dbService.EnvironmentID, &dbService.Name, &dbService.Slug, &dbService.Engine, &dbService.Version, &dbService.Username, &dbService.Password, &dbService.DatabaseName, &dbService.Port, &dbService.ContainerID, &dbService.ContainerName, &dbService.Status, &dbService.ConnectionString, &dbService.CreatedAt, &dbService.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}
		dbServices = append(dbServices, dbService)
	}

	return dbServices, total, nil
}

func (r *DatabaseRepo) UpdateStatus(id string, status models.DatabaseStatus) error {
	_, err := r.db.Exec(
		`UPDATE database_services SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	return err
}

func (r *DatabaseRepo) UpdateContainer(id, containerID string) error {
	_, err := r.db.Exec(
		`UPDATE database_services SET container_id = ?, updated_at = ? WHERE id = ?`,
		containerID, time.Now(), id,
	)
	return err
}

func (r *DatabaseRepo) UpdateContainerAndStatus(id, containerID string, status models.DatabaseStatus) error {
	_, err := r.db.Exec(
		`UPDATE database_services SET container_id = ?, status = ?, updated_at = ? WHERE id = ?`,
		containerID, status, time.Now(), id,
	)
	return err
}

func (r *DatabaseRepo) UpdateConnectionString(id, connectionString string) error {
	_, err := r.db.Exec(
		`UPDATE database_services SET connection_string = ?, updated_at = ? WHERE id = ?`,
		connectionString, time.Now(), id,
	)
	return err
}

func (r *DatabaseRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM database_services WHERE id = ?", id)
	return err
}
