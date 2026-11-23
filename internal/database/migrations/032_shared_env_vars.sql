CREATE TABLE IF NOT EXISTS shared_env_vars (
    id TEXT PRIMARY KEY,
    scope TEXT NOT NULL CHECK(scope IN ('project', 'team')),
    scope_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    is_secret BOOLEAN DEFAULT false,
    is_encrypted BOOLEAN DEFAULT false,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(scope, scope_id, key)
);

CREATE INDEX IF NOT EXISTS idx_shared_env_vars_scope ON shared_env_vars(scope, scope_id);
