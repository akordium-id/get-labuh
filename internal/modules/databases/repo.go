package databases

import (
	"context"
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/modules/projects"
	"github.com/google/uuid"
)

// ServiceRepo mengelola operasi data database service managed containers.
type ServiceRepo struct {
	db *sql.DB
}

func NewServiceRepo(db *sql.DB) *ServiceRepo {
	return &ServiceRepo{db: db}
}

func (r *ServiceRepo) CreateWithContext(ctx context.Context, input models.CreateDatabaseServiceInput) (*models.DatabaseService, error) {
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
		dbService.Slug = projects.GenerateSlug(dbService.Name)
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

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO database_services (id, environment_id, name, slug, engine, version, username, password, database_name, port, status, cpu_limit, memory_limit, memory_reservation, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		dbService.ID, dbService.EnvironmentID, dbService.Name, dbService.Slug, dbService.Engine, dbService.Version, dbService.Username, dbService.Password, dbService.DatabaseName, dbService.Port, dbService.Status, input.CPULimit, input.MemoryLimit, input.MemoryReservation, dbService.CreatedAt, dbService.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	dbService.CPULimit = input.CPULimit
	dbService.MemoryLimit = input.MemoryLimit
	dbService.MemoryReservation = input.MemoryReservation

	return dbService, nil
}

func (r *ServiceRepo) Create(input models.CreateDatabaseServiceInput) (*models.DatabaseService, error) {
	return r.CreateWithContext(context.Background(), input)
}

func (r *ServiceRepo) GetByIDWithContext(ctx context.Context, id string) (*models.DatabaseService, error) {
	dbService := &models.DatabaseService{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, environment_id, name, slug, engine, version, username, password, database_name, port, container_id, container_name, status, connection_string, cpu_limit, memory_limit, memory_reservation, created_at, updated_at
		 FROM database_services WHERE id = ?`,
		id,
	).Scan(&dbService.ID, &dbService.EnvironmentID, &dbService.Name, &dbService.Slug, &dbService.Engine, &dbService.Version, &dbService.Username, &dbService.Password, &dbService.DatabaseName, &dbService.Port, &dbService.ContainerID, &dbService.ContainerName, &dbService.Status, &dbService.ConnectionString, &dbService.CPULimit, &dbService.MemoryLimit, &dbService.MemoryReservation, &dbService.CreatedAt, &dbService.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return dbService, nil
}

func (r *ServiceRepo) GetByID(id string) (*models.DatabaseService, error) {
	return r.GetByIDWithContext(context.Background(), id)
}

func (r *ServiceRepo) GetBySlugWithContext(ctx context.Context, envID, slug string) (*models.DatabaseService, error) {
	dbService := &models.DatabaseService{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, environment_id, name, slug, engine, version, username, password, database_name, port, container_id, container_name, status, connection_string, cpu_limit, memory_limit, memory_reservation, created_at, updated_at
		 FROM database_services WHERE environment_id = ? AND slug = ?`,
		envID, slug,
	).Scan(&dbService.ID, &dbService.EnvironmentID, &dbService.Name, &dbService.Slug, &dbService.Engine, &dbService.Version, &dbService.Username, &dbService.Password, &dbService.DatabaseName, &dbService.Port, &dbService.ContainerID, &dbService.ContainerName, &dbService.Status, &dbService.ConnectionString, &dbService.CPULimit, &dbService.MemoryLimit, &dbService.MemoryReservation, &dbService.CreatedAt, &dbService.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return dbService, nil
}

func (r *ServiceRepo) GetBySlug(envID, slug string) (*models.DatabaseService, error) {
	return r.GetBySlugWithContext(context.Background(), envID, slug)
}

func (r *ServiceRepo) GetByEnvironmentIDWithContext(ctx context.Context, envID string) ([]*models.DatabaseService, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, environment_id, name, slug, engine, version, username, password, database_name, port, container_id, container_name, status, connection_string, cpu_limit, memory_limit, memory_reservation, created_at, updated_at
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
		err := rows.Scan(&dbService.ID, &dbService.EnvironmentID, &dbService.Name, &dbService.Slug, &dbService.Engine, &dbService.Version, &dbService.Username, &dbService.Password, &dbService.DatabaseName, &dbService.Port, &dbService.ContainerID, &dbService.ContainerName, &dbService.Status, &dbService.ConnectionString, &dbService.CPULimit, &dbService.MemoryLimit, &dbService.MemoryReservation, &dbService.CreatedAt, &dbService.UpdatedAt)
		if err != nil {
			return nil, err
		}
		dbServices = append(dbServices, dbService)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return dbServices, nil
}

func (r *ServiceRepo) GetByEnvironmentID(envID string) ([]*models.DatabaseService, error) {
	return r.GetByEnvironmentIDWithContext(context.Background(), envID)
}

func (r *ServiceRepo) GetByEnvironmentIDPaginatedWithContext(ctx context.Context, envID string, limit, offset int) ([]*models.DatabaseService, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM database_services WHERE environment_id = ?", envID).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, environment_id, name, slug, engine, version, username, password, database_name, port, container_id, container_name, status, connection_string, cpu_limit, memory_limit, memory_reservation, created_at, updated_at
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
		err := rows.Scan(&dbService.ID, &dbService.EnvironmentID, &dbService.Name, &dbService.Slug, &dbService.Engine, &dbService.Version, &dbService.Username, &dbService.Password, &dbService.DatabaseName, &dbService.Port, &dbService.ContainerID, &dbService.ContainerName, &dbService.Status, &dbService.ConnectionString, &dbService.CPULimit, &dbService.MemoryLimit, &dbService.MemoryReservation, &dbService.CreatedAt, &dbService.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}
		dbServices = append(dbServices, dbService)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return dbServices, total, nil
}

func (r *ServiceRepo) GetByEnvironmentIDPaginated(envID string, limit, offset int) ([]*models.DatabaseService, int, error) {
	return r.GetByEnvironmentIDPaginatedWithContext(context.Background(), envID, limit, offset)
}

func (r *ServiceRepo) SearchPaginatedWithContext(ctx context.Context, envID, search string, limit, offset int) ([]*models.DatabaseService, int, error) {
	var total int
	searchPattern := "%" + search + "%"
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM database_services WHERE environment_id = ? AND (name LIKE ? OR slug LIKE ?)", envID, searchPattern, searchPattern).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, environment_id, name, slug, engine, version, username, password, database_name, port, container_id, container_name, status, connection_string, cpu_limit, memory_limit, memory_reservation, created_at, updated_at
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
		err := rows.Scan(&dbService.ID, &dbService.EnvironmentID, &dbService.Name, &dbService.Slug, &dbService.Engine, &dbService.Version, &dbService.Username, &dbService.Password, &dbService.DatabaseName, &dbService.Port, &dbService.ContainerID, &dbService.ContainerName, &dbService.Status, &dbService.ConnectionString, &dbService.CPULimit, &dbService.MemoryLimit, &dbService.MemoryReservation, &dbService.CreatedAt, &dbService.UpdatedAt)
		if err != nil {
			return nil, 0, err
		}
		dbServices = append(dbServices, dbService)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	return dbServices, total, nil
}

func (r *ServiceRepo) SearchPaginated(envID, search string, limit, offset int) ([]*models.DatabaseService, int, error) {
	return r.SearchPaginatedWithContext(context.Background(), envID, search, limit, offset)
}

func (r *ServiceRepo) UpdateStatusWithContext(ctx context.Context, id string, status models.DatabaseStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE database_services SET status = ?, updated_at = ? WHERE id = ?`,
		status, time.Now(), id,
	)
	return err
}

func (r *ServiceRepo) UpdateStatus(id string, status models.DatabaseStatus) error {
	return r.UpdateStatusWithContext(context.Background(), id, status)
}

func (r *ServiceRepo) UpdateContainerWithContext(ctx context.Context, id, containerID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE database_services SET container_id = ?, updated_at = ? WHERE id = ?`,
		containerID, time.Now(), id,
	)
	return err
}

func (r *ServiceRepo) UpdateContainer(id, containerID string) error {
	return r.UpdateContainerWithContext(context.Background(), id, containerID)
}

func (r *ServiceRepo) UpdateContainerAndStatusWithContext(ctx context.Context, id, containerID string, status models.DatabaseStatus) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE database_services SET container_id = ?, status = ?, updated_at = ? WHERE id = ?`,
		containerID, status, time.Now(), id,
	)
	return err
}

func (r *ServiceRepo) UpdateContainerAndStatus(id, containerID string, status models.DatabaseStatus) error {
	return r.UpdateContainerAndStatusWithContext(context.Background(), id, containerID, status)
}

func (r *ServiceRepo) UpdateConnectionStringWithContext(ctx context.Context, id, connectionString string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE database_services SET connection_string = ?, updated_at = ? WHERE id = ?`,
		connectionString, time.Now(), id,
	)
	return err
}

func (r *ServiceRepo) UpdateConnectionString(id, connectionString string) error {
	return r.UpdateConnectionStringWithContext(context.Background(), id, connectionString)
}

func (r *ServiceRepo) UpdateResourceLimitsWithContext(ctx context.Context, id string, cpuLimit *float64, memoryLimit, memoryReservation *int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE database_services SET cpu_limit = ?, memory_limit = ?, memory_reservation = ?, updated_at = ? WHERE id = ?`,
		cpuLimit, memoryLimit, memoryReservation, time.Now(), id,
	)
	return err
}

func (r *ServiceRepo) UpdateResourceLimits(id string, cpuLimit *float64, memoryLimit, memoryReservation *int64) error {
	return r.UpdateResourceLimitsWithContext(context.Background(), id, cpuLimit, memoryLimit, memoryReservation)
}

func (r *ServiceRepo) DeleteWithContext(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM database_services WHERE id = ?", id)
	return err
}

func (r *ServiceRepo) Delete(id string) error {
	return r.DeleteWithContext(context.Background(), id)
}

// BackupRepo mengelola konfigurasi dan riwayat database backup.
type BackupRepo struct {
	db *sql.DB
}

func NewBackupRepo(db *sql.DB) *BackupRepo {
	return &BackupRepo{db: db}
}

func (r *BackupRepo) GetConfigWithContext(ctx context.Context, databaseID string) (*models.DatabaseBackupConfig, error) {
	cfg := &models.DatabaseBackupConfig{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, database_id, storage_destination_id, enabled, cron_expression, retention_count, custom_prefix, created_at, updated_at
		 FROM database_backup_configs WHERE database_id = ?`,
		databaseID,
	).Scan(&cfg.ID, &cfg.DatabaseID, &cfg.StorageDestinationID, &cfg.Enabled, &cfg.CronExpression, &cfg.RetentionCount, &cfg.CustomPrefix, &cfg.CreatedAt, &cfg.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (r *BackupRepo) GetConfig(databaseID string) (*models.DatabaseBackupConfig, error) {
	return r.GetConfigWithContext(context.Background(), databaseID)
}

func (r *BackupRepo) UpsertConfigWithContext(ctx context.Context, cfg *models.DatabaseBackupConfig) error {
	now := time.Now()
	if cfg.ID == "" {
		cfg.ID = uuid.New().String()
	}
	if cfg.RetentionCount <= 0 {
		cfg.RetentionCount = 7
	}
	if cfg.CronExpression == "" {
		cfg.CronExpression = "0 2 * * *"
	}

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO database_backup_configs (id, database_id, storage_destination_id, enabled, cron_expression, retention_count, custom_prefix, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(database_id) DO UPDATE SET
		 storage_destination_id = excluded.storage_destination_id,
		 enabled = excluded.enabled,
		 cron_expression = excluded.cron_expression,
		 retention_count = excluded.retention_count,
		 custom_prefix = excluded.custom_prefix,
		 updated_at = excluded.updated_at`,
		cfg.ID, cfg.DatabaseID, cfg.StorageDestinationID, cfg.Enabled, cfg.CronExpression, cfg.RetentionCount, cfg.CustomPrefix, now, now,
	)
	return err
}

func (r *BackupRepo) UpsertConfig(cfg *models.DatabaseBackupConfig) error {
	return r.UpsertConfigWithContext(context.Background(), cfg)
}

func (r *BackupRepo) ListActiveConfigsWithContext(ctx context.Context) ([]*models.DatabaseBackupConfig, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, database_id, storage_destination_id, enabled, cron_expression, retention_count, custom_prefix, created_at, updated_at
		 FROM database_backup_configs WHERE enabled = 1`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.DatabaseBackupConfig
	for rows.Next() {
		cfg := &models.DatabaseBackupConfig{}
		if err := rows.Scan(&cfg.ID, &cfg.DatabaseID, &cfg.StorageDestinationID, &cfg.Enabled, &cfg.CronExpression, &cfg.RetentionCount, &cfg.CustomPrefix, &cfg.CreatedAt, &cfg.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, cfg)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *BackupRepo) ListActiveConfigs() ([]*models.DatabaseBackupConfig, error) {
	return r.ListActiveConfigsWithContext(context.Background())
}

func (r *BackupRepo) CreateBackupWithContext(ctx context.Context, backup *models.DatabaseBackup) error {
	if backup.ID == "" {
		backup.ID = uuid.New().String()
	}
	backup.CreatedAt = time.Now()

	_, err := r.db.ExecContext(ctx,
		`INSERT INTO database_backups (id, database_id, storage_destination_id, filename, file_size, storage_type, s3_key, status, error_message, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		backup.ID, backup.DatabaseID, backup.StorageDestinationID, backup.Filename, backup.FileSize, backup.StorageType, backup.S3Key, backup.Status, backup.ErrorMessage, backup.CreatedAt,
	)
	return err
}

func (r *BackupRepo) CreateBackup(backup *models.DatabaseBackup) error {
	return r.CreateBackupWithContext(context.Background(), backup)
}

func (r *BackupRepo) UpdateStatusWithContext(ctx context.Context, id, status string, fileSize int64, s3Key *string, errMsg *string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE database_backups SET status = ?, file_size = ?, s3_key = ?, error_message = ? WHERE id = ?`,
		status, fileSize, s3Key, errMsg, id,
	)
	return err
}

func (r *BackupRepo) UpdateStatus(id, status string, fileSize int64, s3Key *string, errMsg *string) error {
	return r.UpdateStatusWithContext(context.Background(), id, status, fileSize, s3Key, errMsg)
}

func (r *BackupRepo) GetByIDWithContext(ctx context.Context, id string) (*models.DatabaseBackup, error) {
	b := &models.DatabaseBackup{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, database_id, storage_destination_id, filename, file_size, storage_type, s3_key, status, error_message, created_at
		 FROM database_backups WHERE id = ?`,
		id,
	).Scan(&b.ID, &b.DatabaseID, &b.StorageDestinationID, &b.Filename, &b.FileSize, &b.StorageType, &b.S3Key, &b.Status, &b.ErrorMessage, &b.CreatedAt)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (r *BackupRepo) GetByID(id string) (*models.DatabaseBackup, error) {
	return r.GetByIDWithContext(context.Background(), id)
}

func (r *BackupRepo) ListByDatabaseIDWithContext(ctx context.Context, databaseID string) ([]*models.DatabaseBackup, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, database_id, storage_destination_id, filename, file_size, storage_type, s3_key, status, error_message, created_at
		 FROM database_backups WHERE database_id = ? ORDER BY created_at DESC`,
		databaseID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.DatabaseBackup
	for rows.Next() {
		b := &models.DatabaseBackup{}
		if err := rows.Scan(&b.ID, &b.DatabaseID, &b.StorageDestinationID, &b.Filename, &b.FileSize, &b.StorageType, &b.S3Key, &b.Status, &b.ErrorMessage, &b.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, b)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *BackupRepo) ListByDatabaseID(databaseID string) ([]*models.DatabaseBackup, error) {
	return r.ListByDatabaseIDWithContext(context.Background(), databaseID)
}

func (r *BackupRepo) DeleteWithContext(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM database_backups WHERE id = ?", id)
	return err
}

func (r *BackupRepo) Delete(id string) error {
	return r.DeleteWithContext(context.Background(), id)
}

// GetOldBackupsForPruning mengembalikan backup sukses yang melebihi jumlah retentionCount
func (r *BackupRepo) GetOldBackupsForPruningWithContext(ctx context.Context, databaseID string, keepCount int) ([]*models.DatabaseBackup, error) {
	if keepCount <= 0 {
		keepCount = 7
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, database_id, storage_destination_id, filename, file_size, storage_type, s3_key, status, error_message, created_at
		 FROM database_backups
		 WHERE database_id = ? AND status = 'completed'
		 ORDER BY created_at DESC
		 LIMIT -1 OFFSET ?`,
		databaseID, keepCount,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.DatabaseBackup
	for rows.Next() {
		b := &models.DatabaseBackup{}
		if err := rows.Scan(&b.ID, &b.DatabaseID, &b.StorageDestinationID, &b.Filename, &b.FileSize, &b.StorageType, &b.S3Key, &b.Status, &b.ErrorMessage, &b.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, b)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func (r *BackupRepo) GetOldBackupsForPruning(databaseID string, keepCount int) ([]*models.DatabaseBackup, error) {
	return r.GetOldBackupsForPruningWithContext(context.Background(), databaseID, keepCount)
}
