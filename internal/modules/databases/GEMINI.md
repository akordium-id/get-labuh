# Module: Databases & Database Backup

Package `internal/modules/databases` — pengelolaan container database (PostgreSQL, MySQL, Redis, MongoDB) dan backup otomatis ke local/S3 storage.

## Tanggung Jawab

- **`repo.go`**:
  - `ServiceRepo`: CRUD managed database service (`database_services`), resource limit tracking, container status synchronization.
  - `BackupRepo`: Konfigurasi jadwal cron backup (`database_backup_configs`) dan riwayat backup execution (`database_backups`).
- **`handler.go`**:
  - Listing database per environment dengan pagination dan search.
  - Form & pembuatan database baru (auto generate credential & container provisioning).
  - Lifecycle controller: start, stop, delete container.
  - Live resource limit update (CPU & RAM) ke Docker container.
  - Backup scheduling, backup execution trigger, download, dan pruning.
  - Polling badge status widget.
- **`module.go`**:
  - Inisialisasi modul `New(db, envRepo)`.
  - Dependency injection `SetDockerClient`, `SetBackupDependencies`.
  - Registrasi rute `RegisterRoutes(r chi.Router)`.

## Rules untuk AI Agent

1. Selalu sertakan `context.Context` untuk semua query ke database (`*WithContext`).
2. Jangan mengimpor modul handler legacy ke dalam modul databases.
3. Selalu validasi container status ke Docker runtime sebelum mengubah status running/stopped.
