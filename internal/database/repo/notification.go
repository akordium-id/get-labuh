package repo

import (
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

type NotificationRepo struct {
	db *sql.DB
}

func NewNotificationRepo(db *sql.DB) *NotificationRepo {
	return &NotificationRepo{db: db}
}

func (r *NotificationRepo) Create(input models.CreateNotificationChannelInput) (*models.NotificationChannel, error) {
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

func (r *NotificationRepo) GetByID(id string) (*models.NotificationChannel, error) {
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

func (r *NotificationRepo) ListGlobal() ([]*models.NotificationChannel, error) {
	rows, err := r.db.Query(
		`SELECT id, project_id, type, name, enabled,
				discord_webhook_url, telegram_bot_token, telegram_chat_id,
				on_deploy_success, on_deploy_failure, on_backup_success, on_backup_failure,
				created_at, updated_at
		 FROM notification_channels WHERE project_id IS NULL ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.NotificationChannel
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
		list = append(list, c)
	}
	return list, nil
}

func (r *NotificationRepo) ListByProjectID(projectID string) ([]*models.NotificationChannel, error) {
	rows, err := r.db.Query(
		`SELECT id, project_id, type, name, enabled,
				discord_webhook_url, telegram_bot_token, telegram_chat_id,
				on_deploy_success, on_deploy_failure, on_backup_success, on_backup_failure,
				created_at, updated_at
		 FROM notification_channels WHERE project_id = ? ORDER BY created_at ASC`,
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.NotificationChannel
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
		list = append(list, c)
	}
	return list, nil
}

// ListActiveForEvent mengambil semua channel yang aktif untuk event tertentu:
// Termasuk channel Global (project_id IS NULL) dan channel project spesifik (jika projectID disediakan).
func (r *NotificationRepo) ListActiveForEvent(eventType string, projectID *string) ([]*models.NotificationChannel, error) {
	column := "on_deploy_success"
	switch eventType {
	case "deploy_success":
		column = "on_deploy_success"
	case "deploy_failure":
		column = "on_deploy_failure"
	case "backup_success":
		column = "on_backup_success"
	case "backup_failure":
		column = "on_backup_failure"
	}

	query := `SELECT id, project_id, type, name, enabled,
					 discord_webhook_url, telegram_bot_token, telegram_chat_id,
					 on_deploy_success, on_deploy_failure, on_backup_success, on_backup_failure,
					 created_at, updated_at
			  FROM notification_channels
			  WHERE enabled = 1 AND ` + column + ` = 1 AND (project_id IS NULL`

	var args []interface{}
	if projectID != nil && *projectID != "" {
		query += ` OR project_id = ?`
		args = append(args, *projectID)
	}
	query += `)`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []*models.NotificationChannel
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
		list = append(list, c)
	}
	return list, nil
}

func (r *NotificationRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM notification_channels WHERE id = ?", id)
	return err
}
