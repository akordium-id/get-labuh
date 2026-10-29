# Module: Settings & System Backup

Package `internal/modules/settings` — konfigurasi runtime reverse proxy (Caddy), perpindahan mode (Lite vs Pro), audit logs, multi-storage destinations (S3/MinIO), dan database system backup.

## Tanggung Jawab

- **`repo.go`**:
  - `SettingRepo`: Key-value storage untuk config (`caddy_api_url`, `caddy_network`, dsb.).
  - `AuditLogRepo`: Catatan audit aktivitas user (`audit_logs`).
  - `StorageDestinationRepo`: S3 storage destinations untuk backup target.
- **`handler.go`**:
  - Caddy configuration view & update.
  - Lite/Pro mode switcher cookie (`labuh_mode`).
  - Audit logs viewer.
  - S3 Storage destination CRUD & test connection.
  - SQLite database backup/restore (`VACUUM INTO`).
- **`module.go`**: Inisialisasi `New(db, backupPath)` dan registrasi rute `RegisterRoutes(r)`.

## Rules untuk AI Agent

1. Selalu gunakan `context.Context` pada query ke repository setting, audit log, dan storage.
2. Saat menambahkan pengaturan sistem baru, inisialisasi default-nya di `InitializeDefaults` di `repo.go`.
3. Validasi magic bytes SQLite (`SQLite format 3\x00`) saat proses restore backup database.
