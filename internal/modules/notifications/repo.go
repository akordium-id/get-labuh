package notifications

import (
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

func (r *Repo) Create(input models.CreateNotificationChannelInput) (*models.NotificationChannel, error) {
	id := uuid.New().String()
	now := time.Now()

	_, err := r.db.Exec(
		`INSERT INTO notification_channels (
			id, project_id, type, name, enabled,
			discord_webhook_url, telegram_bot_token, telegram_chat_id,
			on_deploy_success, on_deploy_failure, on_backup_success, on_backup_failure,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, input.ProjectID, input.Type, input.Name, input.Enabled,
		input.DiscordWebhookURL, input.TelegramBotToken, input.TelegramChatID,
		input.OnDeploySuccess, input.OnDeployFailure, input.OnBackupSuccess, input.OnBackupFailure,
		now, now,
	)
	if err != nil {
		return nil, err
	}

	return r.GetByID(id)
}

func (r *Repo) GetByID(id string) (*models.NotificationChannel, error) {
	c := &models.NotificationChannel{}
	err := r.db.QueryRow(
		`SELECT id, project_id, type, name, enabled,
				discord_webhook_url, telegram_bot_token, telegram_chat_id,
				on_deploy_success, on_deploy_failure, on_backup_success, on_backup_failure,
				created_at, updated_at
		 FROM notification_channels WHERE id = ?`,
		id,
	).Scan(
		&c.ID, &c.ProjectID, &c.Type, &c.Name, &c.Enabled,
		&c.DiscordWebhookURL, &c.TelegramBotToken, &c.TelegramChatID,
		&c.OnDeploySuccess, &c.OnDeployFailure, &c.OnBackupSuccess, &c.OnBackupFailure,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (r *Repo) ListGlobal() ([]*models.NotificationChannel, error) {
	rows, err := r.db.Query(
		`SELECT id, project_id, type, name, enabled,
				discord_webhook_url, telegram_bot_token, telegram_chat_id,
				on_deploy_success, on_deploy_failure, on_backup_success, on_backup_failure,
				created_at, updated_at
		 FROM notification_channels WHERE project_id IS NULL ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []*models.NotificationChannel
	for rows.Next() {
		c := &models.NotificationChannel{}
		if err := rows.Scan(
			&c.ID, &c.ProjectID, &c.Type, &c.Name, &c.Enabled,
			&c.DiscordWebhookURL, &c.TelegramBotToken, &c.TelegramChatID,
			&c.OnDeploySuccess, &c.OnDeployFailure, &c.OnBackupSuccess, &c.OnBackupFailure,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, nil
}

func (r *Repo) ListByProjectID(projectID string) ([]*models.NotificationChannel, error) {
	rows, err := r.db.Query(
		`SELECT id, project_id, type, name, enabled,
				discord_webhook_url, telegram_bot_token, telegram_chat_id,
				on_deploy_success, on_deploy_failure, on_backup_success, on_backup_failure,
				created_at, updated_at
		 FROM notification_channels WHERE project_id = ? ORDER BY created_at DESC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []*models.NotificationChannel
	for rows.Next() {
		c := &models.NotificationChannel{}
		if err := rows.Scan(
			&c.ID, &c.ProjectID, &c.Type, &c.Name, &c.Enabled,
			&c.DiscordWebhookURL, &c.TelegramBotToken, &c.TelegramChatID,
			&c.OnDeploySuccess, &c.OnDeployFailure, &c.OnBackupSuccess, &c.OnBackupFailure,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, nil
}

func (r *Repo) ListActiveForEvent(eventType string, projectID *string) ([]*models.NotificationChannel, error) {
	var eventCol string
	switch eventType {
	case "deploy_success":
		eventCol = "on_deploy_success = 1"
	case "deploy_failure":
		eventCol = "on_deploy_failure = 1"
	case "backup_success":
		eventCol = "on_backup_success = 1"
	case "backup_failure":
		eventCol = "on_backup_failure = 1"
	default:
		return nil, nil
	}

	query := `SELECT id, project_id, type, name, enabled,
			discord_webhook_url, telegram_bot_token, telegram_chat_id,
			on_deploy_success, on_deploy_failure, on_backup_success, on_backup_failure,
			created_at, updated_at
		FROM notification_channels
		WHERE enabled = 1 AND ` + eventCol

	var args []any
	if projectID != nil {
		query += ` AND (project_id IS NULL OR project_id = ?)`
		args = append(args, *projectID)
	} else {
		query += ` AND project_id IS NULL`
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []*models.NotificationChannel
	for rows.Next() {
		c := &models.NotificationChannel{}
		if err := rows.Scan(
			&c.ID, &c.ProjectID, &c.Type, &c.Name, &c.Enabled,
			&c.DiscordWebhookURL, &c.TelegramBotToken, &c.TelegramChatID,
			&c.OnDeploySuccess, &c.OnDeployFailure, &c.OnBackupSuccess, &c.OnBackupFailure,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, nil
}

func (r *Repo) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM notification_channels WHERE id = ?`, id)
	return err
}
