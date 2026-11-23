-- Migration 028: Tambahkan dukungan Persistent Volumes dan File Mounts untuk Applications

CREATE TABLE IF NOT EXISTS app_volumes (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    volume_type TEXT NOT NULL DEFAULT 'named', -- 'named' atau 'bind'
    host_path TEXT,                            -- NULL untuk 'named'; path host jika 'bind'
    mount_path TEXT NOT NULL,                   -- Path target di container (mis. /app/storage)
    is_read_only BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(application_id, mount_path)
);

CREATE INDEX IF NOT EXISTS idx_app_volumes_application_id ON app_volumes(application_id);

CREATE TABLE IF NOT EXISTS app_file_mounts (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    mount_path TEXT NOT NULL,                   -- Path target di container (mis. /etc/nginx/nginx.conf)
    content TEXT NOT NULL,                     -- Isi konten teks konfigurasi
    file_mode TEXT NOT NULL DEFAULT '0644',     -- Permissions octal file
    is_read_only BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(application_id, mount_path)
);

CREATE INDEX IF NOT EXISTS idx_app_file_mounts_application_id ON app_file_mounts(application_id);
