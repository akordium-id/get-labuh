-- Tambahkan kolom ssh_key_content ke tabel servers untuk menyimpan private key langsung
ALTER TABLE servers ADD COLUMN ssh_key_content TEXT;

-- Tambahkan kolom server_id ke tabel applications
ALTER TABLE applications ADD COLUMN server_id TEXT REFERENCES servers(id);

-- Tambahkan kolom server_id ke tabel database_services
ALTER TABLE database_services ADD COLUMN server_id TEXT REFERENCES servers(id);

-- Masukkan default server local jika belum ada
INSERT OR IGNORE INTO servers (id, name, host, port, ssh_user, ssh_key_path, status, created_at, updated_at)
VALUES ('local', 'Localhost (Built-in)', '127.0.0.1', 22, 'root', '', 'online', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

CREATE INDEX IF NOT EXISTS idx_applications_server_id ON applications(server_id);
CREATE INDEX IF NOT EXISTS idx_database_services_server_id ON database_services(server_id);
