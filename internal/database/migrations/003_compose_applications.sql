CREATE TABLE compose_applications (
    id TEXT PRIMARY KEY,
    environment_id TEXT NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    compose_file_path TEXT NOT NULL DEFAULT 'docker-compose.yml',
    compose_project_name TEXT,
    custom_domain TEXT UNIQUE,
    status TEXT NOT NULL DEFAULT 'idle',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(environment_id, slug)
);
