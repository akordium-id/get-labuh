CREATE TABLE IF NOT EXISTS storage_destinations (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT 0,
    endpoint TEXT NOT NULL,
    bucket TEXT NOT NULL,
    region TEXT NOT NULL DEFAULT 'us-east-1',
    access_key_id TEXT NOT NULL,
    secret_access_key TEXT NOT NULL,
    prefix TEXT NOT NULL DEFAULT 'backups',
    use_ssl BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS database_backup_configs (
    id TEXT PRIMARY KEY,
    database_id TEXT NOT NULL REFERENCES database_services(id) ON DELETE CASCADE,
    storage_destination_id TEXT REFERENCES storage_destinations(id) ON DELETE SET NULL,
    enabled BOOLEAN NOT NULL DEFAULT 0,
    cron_expression TEXT NOT NULL DEFAULT '0 2 * * *',
    retention_count INTEGER NOT NULL DEFAULT 7,
    custom_prefix TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(database_id)
);

CREATE TABLE IF NOT EXISTS database_backups (
    id TEXT PRIMARY KEY,
    database_id TEXT NOT NULL REFERENCES database_services(id) ON DELETE CASCADE,
    storage_destination_id TEXT REFERENCES storage_destinations(id) ON DELETE SET NULL,
    filename TEXT NOT NULL,
    file_size INTEGER NOT NULL DEFAULT 0,
    storage_type TEXT NOT NULL DEFAULT 'local',
    s3_key TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_database_backups_db_id ON database_backups(database_id);
CREATE INDEX IF NOT EXISTS idx_database_backups_created_at ON database_backups(created_at);
