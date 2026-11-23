# Design Spec: 1-Click Database Restore & Application Persistent Volume Backup/Restore

## 1. Context & Objectives

Sebelum fitur ini diimplementasikan:
- Managed Database Services (`PostgreSQL`, `MySQL`, `MongoDB`, `Redis`) telah memiliki cron runner untuk dump otomatis ke lokal/S3 (`internal/backup/database_runner.go`), namun dashboard hanya menyediakan aksi **Download** dan **Delete**. Belum ada mekanisme **1-Click Restore** untuk mengembalikan database dari file backup.
- Fitur Application Persistent Volumes & File Mounts (Opsi A) baru saja diselesaikan, tetapi belum memiliki sistem automated backup snapshot terjadwal maupun manual ke S3/lokal.

Tujuan spesifikasi ini:
1. Menyediakan fitur **1-Click Database Restore** dengan dua mode:
   - **In-place Restore**: Mengembalikan data langsung ke container database aktif dengan konfirmasi keamanan.
   - **Restore to New Database**: Mem-provision instance database baru secara otomatis dan menginjeksi data backup ke dalamnya.
2. Menyediakan fitur **Application Volume Backup & Safe Restore**:
   - Backup level aplikasi yang meng-archive seluruh persistent volumes & file mounts aplikasi menjadi satu file snapshot `.tar.gz` ke S3/lokal.
   - **Safe Restore**: Menghentikan container sementara, mengekstrak tarball snapshot ke volume, dan menyalakan kembali container untuk mencegah data corruption atau file locking.
3. Menjaga arsitektur tetap terpadu di dalam package `internal/backup` dengan memanfaatkan native Docker Engine API tanpa memerlukan dependency CLI eksternal atau root filesystem access.

---

## 2. Architecture & Components

```
+-----------------------------------------------------------------------------------+
|                               UI / HTTP Handlers                                  |
|   /databases/{id}/backups/{bid}/restore    /applications/{id}/backups/...         |
+-----------------------------------------------------------------------------------+
                                   |
                                   v
+-----------------------------------------------------------------------------------+
|                        Unified Engine (internal/backup)                           |
|  - DatabaseBackupRunner: Dump, Prune, RestoreBackup (psql, mysql, mongorestore)  |
|  - AppVolumeBackupRunner: Archive (.tar.gz), Prune, SafeRestoreAppVolume         |
+-----------------------------------------------------------------------------------+
           |                                                      |
           v                                                      v
+------------------------------------+  +-------------------------------------------+
|          S3 Storage Client         |  |             Docker Engine SDK             |
|   (Upload, Download, Delete S3)    |  |  - ExecStreamWithStdin (streaming stdin)  |
|                                    |  |  - CopyFromContainer / CopyToContainer    |
+------------------------------------+  +-------------------------------------------+
```

### Komponen Utama:
1. **`internal/docker/container.go`**:
   - `ExecStreamWithStdin(ctx, containerID, cmd, env, stdin, stdout)`: Membuka sesi exec dengan `AttachStdin: true` untuk mengalirkan dekompresi data dump langsung ke proses restore di container.
   - `CopyFromContainer` dan `CopyToContainer`: Native Docker API untuk membaca dan menulis stream archive tar ke mount path container.
2. **`internal/backup/database_runner.go`**:
   - Ditambahkan fungsi `RestoreBackup(ctx context.Context, backupID string, targetDB *models.DatabaseService) error`.
   - Streaming on-the-fly (`gzip.NewReader`) langsung dari file lokal atau S3 ke stdin container tanpa membuat file uncompressed sementara di disk.
3. **`internal/backup/app_volume_runner.go`**:
   - `RunBackup(ctx context.Context, appID string) (*models.AppBackup, error)`: Mengambil stream volume dari container via Docker Archive API, menggabungkannya ke dalam snapshot `.tar.gz`, mengunggah ke S3, dan menjalankan retention pruning.
   - `RestoreAppVolume(ctx context.Context, backupID string) error`: Melakukan *Safe Restore* dengan siklus `StopContainer` -> `CopyToContainer` -> `StartContainer`.
4. **`internal/database/migrations/029_application_volume_backups.sql`**:
   - Tabel `app_backup_configs` dan `app_backups`.
5. **UI Layer (`internal/web/pages`)**:
   - `databases/detail.templ`: Tombol & modal interaktif Restore (In-place vs New DB).
   - `applications/list.templ`: Card Volume Backups & Snapshots pada tab `#storage`.

---

## 3. Database Schema & Domain Models

### Migration: `029_application_volume_backups.sql`
```sql
CREATE TABLE IF NOT EXISTS app_backup_configs (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    storage_destination_id TEXT REFERENCES storage_destinations(id) ON DELETE SET NULL,
    enabled BOOLEAN NOT NULL DEFAULT 0,
    cron_expression TEXT NOT NULL DEFAULT '0 3 * * *',
    retention_count INTEGER NOT NULL DEFAULT 7,
    custom_prefix TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(application_id)
);

CREATE TABLE IF NOT EXISTS app_backups (
    id TEXT PRIMARY KEY,
    application_id TEXT NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    storage_destination_id TEXT REFERENCES storage_destinations(id) ON DELETE SET NULL,
    filename TEXT NOT NULL,
    file_size INTEGER NOT NULL DEFAULT 0,
    storage_type TEXT NOT NULL DEFAULT 'local',
    s3_key TEXT,
    status TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_app_backups_app_id ON app_backups(application_id);
CREATE INDEX IF NOT EXISTS idx_app_backups_created_at ON app_backups(created_at);
```

### Model Domain (`internal/models/application.go`)
```go
type AppBackupConfig struct {
    ID                   string     `json:"id"`
    ApplicationID        string     `json:"application_id"`
    StorageDestinationID *string    `json:"storage_destination_id,omitempty"`
    Enabled              bool       `json:"enabled"`
    CronExpression       string     `json:"cron_expression"`
    RetentionCount       int        `json:"retention_count"`
    CustomPrefix         *string    `json:"custom_prefix,omitempty"`
    CreatedAt            time.Time  `json:"created_at"`
    UpdatedAt            time.Time  `json:"updated_at"`
}

type AppBackup struct {
    ID                   string     `json:"id"`
    ApplicationID        string     `json:"application_id"`
    StorageDestinationID *string    `json:"storage_destination_id,omitempty"`
    Filename             string     `json:"filename"`
    FileSize             int64      `json:"file_size"`
    StorageType          string     `json:"storage_type"`
    S3Key                *string    `json:"s3_key,omitempty"`
    Status               string     `json:"status"` // "pending", "completed", "failed"
    ErrorMessage         *string    `json:"error_message,omitempty"`
    CreatedAt            time.Time  `json:"created_at"`
}
```

---

## 4. Detailed Execution Flows

### 4.1. 1-Click Database Restore Flow

```
User clicks "Restore" on backup row
              │
              ▼
Modal Prompt: [In-place] or [Restore to New Database]
              │
    ┌─────────┴─────────┐
    ▼                   ▼
[In-place]      [New Database]
    │                   │
    │                   ├─ 1. dbRepo.Create(newSlug, credentials)
    │                   ├─ 2. docker.RunDatabaseContainer(newDB)
    │                   └─ 3. Wait until port/healthcheck passes
    │                           │
    └───────────┬───────────────┘
                ▼
Runner: Open local file OR download stream from S3
                │
                ▼
gzip.NewReader(fileStream)
                │
                ▼
docker.ExecStreamWithStdin(ctx, targetContainerID, restoreCmd, envs, gzReader, stdout)
                │
                ▼
Verify ExitCode == 0:
  - If error: return detailed stderr to user, trigger notification "restore_failed"
  - If success: return toast success, trigger notification "restore_success"
```

#### Engine Restore Commands:
- **PostgreSQL**: `psql -U <user> -d <dbname>` with `PGPASSWORD=<pass>`
- **MySQL**: `mysql -u<user> -p<pass> <dbname>`
- **MongoDB**: `mongorestore --username <user> --password <pass> --authenticationDatabase admin --db <dbname> --archive --drop`
- **Redis**: Graceful shutdown `redis-cli shutdown nosave`, copy `dump.rdb` into `/data/dump.rdb`, restart container.

### 4.2. Application Volume Backup Flow

1. Runner membaca daftar `app_volumes` dan `app_file_mounts` milik aplikasi.
2. Membuat file lokal `app-volume-<slug>-<timestamp>.tar.gz` di `/var/lib/labuh/backups/apps/<appID>/` (atau fallback home non-root).
3. Menggunakan `tar.Writer` + `gzip.Writer`:
   Untuk setiap mount path, memanggil `c.cli.CopyFromContainer(ctx, containerID, mountPath)` dan menuliskan entry ke dalam composite tar archive.
4. Jika `StorageDestinationID` aktif, upload ke S3 bucket menggunakan `S3Client.PutObject`.
5. Update status `completed` di tabel `app_backups`.
6. Pruning snapshot lama yang melebihi `retention_count` (hapus file lokal & S3 object).

### 4.3. Application Volume Safe Restore Flow

1. Ambil record `AppBackup` dan pastikan file backup tersedia (unduh dari S3 jika hanya tersimpan di remote).
2. **Stop Container Sementara**: `dockerClient.StopContainer(ctx, containerID, 10)` untuk mencegah file lock atau race condition penulisan disk.
3. Buka file `.tar.gz` via `gzip.NewReader` -> ekstrak stream menggunakan `c.cli.CopyToContainer(ctx, containerID, "/", tarStream, ...)`.
4. **Start Kembali Container**: `dockerClient.StartContainer(ctx, containerID)`.
5. Kirim notifikasi sukses/gagal.

---

## 5. HTTP Routing & UI Interactions

### Routes:
- `POST /databases/{id}/backups/{backup_id}/restore`
- `POST /applications/{id}/backups/config`
- `POST /applications/{id}/backups/create`
- `GET /applications/{id}/backups/{backup_id}/download`
- `POST /applications/{id}/backups/{backup_id}/restore`
- `POST /applications/{id}/backups/{backup_id}/delete`

### UI Modals:
- **Database Restore Modal**:
  Menampilkan pilihan Radio Mode:
  - `in_place`: *"Timpa database yang sedang berjalan"* (Warning Callout).
  - `new_database`: *"Buat instance database baru"* (Input: `Nama Database Baru`).
- **Application Volume Restore Modal**:
  Callout: *"Container aplikasi akan dihentikan sementara selama proses ekstraksi volume. Lanjutkan?"*.

---

## 6. Testing Strategy

1. **Unit Tests**:
   - `internal/docker`: Test `ExecStreamWithStdin` dengan container echo/cat streaming.
   - `internal/backup/database_runner_test.go`: Test `RestoreBackup` dengan mock/test database.
   - `internal/backup/app_volume_runner_test.go`: Test pembuatan `.tar.gz` archive dan ekstraksi restore.
   - `internal/modules/applications`: Test repo CRUD untuk `app_backup_configs` dan `app_backups`.
2. **Integration Tests**:
   - End-to-end flow backup & restore database (dump -> modify data -> restore -> verify data recovered).
   - End-to-end flow volume snapshot -> safe restore -> container running.
