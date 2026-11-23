CREATE TABLE IF NOT EXISTS app_backup_configs (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    storage_destination_id TEXT REFERENCES storage_destinations(id) ON DELETE SET NULL,
    enabled BOOLEAN NOT NULL DEFAULT 0,
    cron_expression TEXT NOT NULL DEFAULT '0 3 * * *',
    retention_count INTEGER NOT NULL DEFAULT 7,
    custom_prefix TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(application_id)
);

CREATE TABLE IF NOT EXISTS app_backups (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    storage_destination_id TEXT REFERENCES storage_destinations(id) ON DELETE SET NULL,
    filename TEXT NOT NULL,
    file_size INTEGER NOT NULL DEFAULT 0,
    storage_type TEXT NOT NULL DEFAULT 'local',
    s3_key TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_app_backups_app_id ON app_backups(application_id);
CREATE INDEX IF NOT EXISTS idx_app_backups_created_at ON app_backups(created_at);
