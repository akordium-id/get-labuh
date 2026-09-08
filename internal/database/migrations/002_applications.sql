CREATE TABLE applications (
    id TEXT PRIMARY KEY,
    environment_id TEXT NOT NULL REFERENCES environments(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    source_type TEXT NOT NULL DEFAULT 'git',
    repository_url TEXT,
    branch TEXT DEFAULT 'main',
    build_path TEXT DEFAULT '/',
    dockerfile_path TEXT DEFAULT 'Dockerfile',
    docker_image TEXT,
    custom_domain TEXT UNIQUE,
    app_port INTEGER NOT NULL DEFAULT 8080,
    container_id TEXT,
    container_name TEXT UNIQUE,
    status TEXT NOT NULL DEFAULT 'idle',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(environment_id, slug)
);

CREATE TABLE app_env_vars (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    is_secret BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(application_id, key)
);

CREATE TABLE deployments (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    commit_hash TEXT,
    commit_message TEXT,
    status TEXT NOT NULL DEFAULT 'queued',
    error_message TEXT,
    log_path TEXT,
    started_at TIMESTAMP WITH TIME ZONE,
    finished_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);
