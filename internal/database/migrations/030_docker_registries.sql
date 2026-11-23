CREATE TABLE IF NOT EXISTS docker_registries (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    server_address TEXT NOT NULL,
    username TEXT NOT NULL,
    password TEXT NOT NULL,
    email TEXT DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_docker_registries_server_address ON docker_registries(server_address);

ALTER TABLE applications ADD COLUMN docker_registry_id TEXT REFERENCES docker_registries(id) ON DELETE SET NULL;
