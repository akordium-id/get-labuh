CREATE TABLE IF NOT EXISTS notification_channels (
    id TEXT PRIMARY KEY,
    project_id TEXT REFERENCES projects(id) ON DELETE CASCADE,
    type TEXT NOT NULL, -- 'discord' or 'telegram'
    name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    discord_webhook_url TEXT,
    telegram_bot_token TEXT,
    telegram_chat_id TEXT,
    on_deploy_success BOOLEAN NOT NULL DEFAULT 1,
    on_deploy_failure BOOLEAN NOT NULL DEFAULT 1,
    on_backup_success BOOLEAN NOT NULL DEFAULT 1,
    on_backup_failure BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_notification_channels_project ON notification_channels(project_id);
