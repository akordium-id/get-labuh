CREATE TABLE database_services (
    id TEXT PRIMARY KEY,
    environment_id TEXT NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    engine TEXT NOT NULL,
    version TEXT NOT NULL DEFAULT 'latest',
    username TEXT NOT NULL,
    password TEXT NOT NULL,
    database_name TEXT NOT NULL,
    port INTEGER NOT NULL,
    container_id TEXT,
    container_name TEXT UNIQUE,
    status TEXT NOT NULL DEFAULT 'idle',
    connection_string TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(environment_id, slug)
);
