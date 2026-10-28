package models

import "time"

type NotificationType string

const (
	NotificationTypeDiscord  NotificationType = "discord"
	NotificationTypeTelegram NotificationType = "telegram"
)

type NotificationChannel struct {
	ID                string           `json:"id"`
	ProjectID         *string          `json:"project_id"` // nil means global
	Type              NotificationType `json:"type"`
	Name              string           `json:"name"`
	Enabled           bool             `json:"enabled"`
	DiscordWebhookURL *string          `json:"discord_webhook_url"`
	TelegramBotToken  *string          `json:"telegram_bot_token"`
	TelegramChatID    *string          `json:"telegram_chat_id"`
	OnDeploySuccess   bool             `json:"on_deploy_success"`
	OnDeployFailure   bool             `json:"on_deploy_failure"`
	OnBackupSuccess   bool             `json:"on_backup_success"`
	OnBackupFailure   bool             `json:"on_backup_failure"`
	CreatedAt         time.Time        `json:"created_at"`
	UpdatedAt         time.Time        `json:"updated_at"`
}

type CreateNotificationChannelInput struct {
	ProjectID         *string          `json:"project_id"`
	Type              NotificationType `json:"type"`
	Name              string           `json:"name"`
	Enabled           bool             `json:"enabled"`
	DiscordWebhookURL *string          `json:"discord_webhook_url"`
	TelegramBotToken  *string          `json:"telegram_bot_token"`
	TelegramChatID    *string          `json:"telegram_chat_id"`
	OnDeploySuccess   bool             `json:"on_deploy_success"`
	OnDeployFailure   bool             `json:"on_deploy_failure"`
	OnBackupSuccess   bool             `json:"on_backup_success"`
	OnBackupFailure   bool             `json:"on_backup_failure"`
}

type NotificationEvent struct {
	Title       string
	Description string
	Status      string // "success" or "failure"
	EventType   string // "deploy_success", "deploy_failure", "backup_success", "backup_failure"
	Fields      map[string]string
	ProjectID   *string
}
