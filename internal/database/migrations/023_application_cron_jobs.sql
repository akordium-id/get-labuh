CREATE TABLE IF NOT EXISTS application_cron_jobs (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    command TEXT NOT NULL,
    cron_expression TEXT NOT NULL,
    is_enabled BOOLEAN NOT NULL DEFAULT 1,
    last_run_at DATETIME,
    last_status TEXT,
    last_output TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_application_cron_jobs_app_id ON application_cron_jobs(application_id);
CREATE INDEX IF NOT EXISTS idx_application_cron_jobs_enabled ON application_cron_jobs(is_enabled);
