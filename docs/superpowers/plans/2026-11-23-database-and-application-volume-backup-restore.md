# 1-Click Database Restore & Application Persistent Volume Backup/Restore Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement 1-Click Database Restore (both in-place and to a new database) and automated Application Persistent Volume Backups & Safe Restore to S3/local storage.

**Architecture:** Extend `internal/backup` with a unified backup/restore engine utilizing native Docker Engine SDK (`ExecStreamWithStdin` for streaming dump directly into container stdin, `CopyFromContainer` and `CopyToContainer` for volume archiving without host root access). Wire endpoints in `internal/modules/databases` and `internal/modules/applications`, and integrate Alpine.js modals in Templ UI.

**Tech Stack:** Go (1.23+), Chi v5, Docker Moby SDK, SQLite (`modernc.org/sqlite`), AWS S3 SDK (via `minio-go` or current S3 client), Templ, HTMX, Alpine.js.

**Spec:** [docs/superpowers/specs/2026-11-23-database-and-application-volume-backup-restore-design.md](file:///home/orin/code/archive/get-labuh/docs/superpowers/specs/2026-11-23-database-and-application-volume-backup-restore-design.md)

## Global Constraints

- Database migrations must be sequentially numbered in `internal/database/migrations/` (`029_application_volume_backups.sql`). Do not alter past migrations.
- Models in `internal/models/` must remain pure domain structs with no internal package imports.
- SQLite queries must use context passing and standard placeholders (`?`).
- Database tests must run on SQLite in-memory (`database.Connect(":memory:")`).
- All Templ files must be paired with generated `*_templ.go` via `templ generate`. Do not edit `*_templ.go` directly.
- Secrets and S3 keys must never be exposed in plaintext in logs or notifications.

## Review Focus

1. **Streaming Restore Failure / Broken Pipe**: When `gunzip` or database client (`psql`/`mysql`) fails mid-stream, `ExecStreamWithStdin` must terminate cleanly, capture stderr, and return a descriptive error instead of hanging indefinitely.
2. **Safe Restore Container Recovery**: During application volume restore, if extraction fails after container is stopped, `RestoreAppVolume` must ensure the container is restarted so the application does not remain permanently offline.
3. **Empty Volumes Edge Case**: If an application has no persistent volumes or file mounts configured, triggering a volume backup must fail fast with 400 Bad Request instead of producing a corrupt empty archive.
4. **Collision on Restore to New Database**: Generating slug for the new database (`<slug>-restored-<timestamp>`) must handle slug collisions safely.
5. **S3 Download Fallback**: If a backup record has `storage_type == 's3'` and the local disk file is missing, the restore process must stream directly from S3 without crashing.

---

### Task 1: Database Migration `029_application_volume_backups.sql` & Domain Models

**Files:**
- Create: `internal/database/migrations/029_application_volume_backups.sql`
- Modify: `internal/models/application.go`
- Test: `internal/models/application_test.go`

**Interfaces:**
- Produces: `models.AppBackupConfig`, `models.AppBackup`, `models.CreateAppBackupConfigInput`, `models.UpdateAppBackupConfigInput`, `models.CreateAppBackupInput`

- [ ] **Step 1: Write model tests verifying domain structs**

Add test in `internal/models/application_test.go` verifying instantiation and validation of `AppBackupConfig` and `AppBackup`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/models -run TestAppBackupModels`
Expected: FAIL (models undefined)

- [ ] **Step 3: Define domain models in `internal/models/application.go` and create SQL migration `029_application_volume_backups.sql`**

Define `AppBackupConfig` and `AppBackup` domain structs.
Create `029_application_volume_backups.sql` creating tables `app_backup_configs` and `app_backups` with indexes.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/models -run TestAppBackupModels`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/database/migrations/029_application_volume_backups.sql internal/models/application.go internal/models/application_test.go
git commit -m "feat(models): add application volume backup models and migration 029"
```

---

### Task 2: Repository CRUD for Application Volume Backups

**Files:**
- Create: `internal/modules/applications/repo_backup.go`
- Test: `internal/modules/applications/repo_backup_test.go`

**Interfaces:**
- Consumes: `models.AppBackupConfig`, `models.AppBackup` from Task 1
- Produces:
  - `(r *Repo) GetBackupConfigWithContext(ctx context.Context, appID string) (*models.AppBackupConfig, error)`
  - `(r *Repo) UpsertBackupConfigWithContext(ctx context.Context, input models.UpdateAppBackupConfigInput) error`
  - `(r *Repo) CreateBackupWithContext(ctx context.Context, input models.CreateAppBackupInput) (*models.AppBackup, error)`
  - `(r *Repo) UpdateBackupStatusWithContext(ctx context.Context, id, status string, size int64, s3Key *string, errMsg *string) error`
  - `(r *Repo) GetBackupByIDWithContext(ctx context.Context, id string) (*models.AppBackup, error)`
  - `(r *Repo) ListBackupsByAppIDWithContext(ctx context.Context, appID string) ([]*models.AppBackup, error)`
  - `(r *Repo) DeleteBackupWithContext(ctx context.Context, id string) error`
  - `(r *Repo) GetOldBackupsForPruning(ctx context.Context, appID string, keepCount int) ([]*models.AppBackup, error)`

- [ ] **Step 1: Write repository tests in `repo_backup_test.go`**

Test CRUD operations on in-memory SQLite database (`database.Connect(":memory:")`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/modules/applications -run TestAppBackupRepo`
Expected: FAIL (repo methods undefined)

- [ ] **Step 3: Implement `repo_backup.go`**

Implement all repository methods using `modernc.org/sqlite` with `?` parameters and context.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/modules/applications -run TestAppBackupRepo`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/applications/repo_backup.go internal/modules/applications/repo_backup_test.go
git commit -m "feat(repo): add application volume backup repository CRUD and pruning queries"
```

---

### Task 3: Docker SDK Stdin Streaming & Archive API Extensions

**Files:**
- Modify: `internal/docker/container.go`
- Test: `internal/docker/container_test.go`

**Interfaces:**
- Produces:
  - `(c *Client) ExecStreamWithStdin(ctx context.Context, containerID string, cmd []string, env []string, stdin io.Reader, stdout io.Writer) error`
  - `(c *Client) CopyFromContainer(ctx context.Context, containerID, srcPath string) (io.ReadCloser, error)`
  - `(c *Client) CopyToContainer(ctx context.Context, containerID, dstPath string, content io.Reader) error`

- [ ] **Step 1: Write test for `ExecStreamWithStdin` in `container_test.go`**

Test running `sh -c 'cat'` with an `io.Reader` input and asserting stdout captures the exact piped content.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/docker -run TestExecStreamWithStdin`
Expected: FAIL (`ExecStreamWithStdin` not implemented)

- [ ] **Step 3: Implement `ExecStreamWithStdin`, `CopyFromContainer`, and `CopyToContainer`**

In `internal/docker/container.go`, use Docker SDK `ContainerExecCreate`, `ContainerExecAttach` with `AttachStdin: true`, goroutine `io.Copy(conn, stdin)`, and `stdcopy.StdCopy`.
Add wrapper methods for `c.cli.CopyFromContainer` and `c.cli.CopyToContainer`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/docker -run TestExecStreamWithStdin`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/docker/container.go internal/docker/container_test.go
git commit -m "feat(docker): add ExecStreamWithStdin and archive copy wrappers"
```

---

### Task 4: Database Restore Engine in `DatabaseBackupRunner`

**Files:**
- Modify: `internal/backup/database_runner.go`
- Test: `internal/backup/database_runner_test.go`

**Interfaces:**
- Consumes: `dockerClient.ExecStreamWithStdin` from Task 3, `backupRepo.GetByID`, `dbRepo.GetByID`, `storageRepo.GetByID`
- Produces:
  - `(r *DatabaseBackupRunner) RestoreBackup(ctx context.Context, backupID string, targetDB *models.DatabaseService) error`

- [ ] **Step 1: Write unit test in `database_runner_test.go`**

Test `RestoreBackup` with a mock/test database service and gzip SQL input, verifying correct restore command and environment generation (`psql`, `mysql`, `mongorestore`, `redis`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/backup -run TestDatabaseRestore`
Expected: FAIL (`RestoreBackup` not implemented)

- [ ] **Step 3: Implement `RestoreBackup` in `internal/backup/database_runner.go`**

1. Locate backup: read local file or stream from S3.
2. Wrap with `gzip.NewReader`.
3. Build command:
   - PostgreSQL: `psql -U <user> -d <dbname>`
   - MySQL: `mysql -u<user> -p<pass> <dbname>`
   - MongoDB: `mongorestore --username ... --archive --drop`
   - Redis: shutdown, write RDB, restart.
4. Execute via `dockerClient.ExecStreamWithStdin`.
5. Dispatch notification event.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/backup -run TestDatabaseRestore`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/backup/database_runner.go internal/backup/database_runner_test.go
git commit -m "feat(backup): implement streaming database restore in DatabaseBackupRunner"
```

---

### Task 5: Application Volume Backup & Safe Restore Engine in `AppVolumeBackupRunner`

**Files:**
- Create: `internal/backup/app_volume_runner.go`
- Test: `internal/backup/app_volume_runner_test.go`

**Interfaces:**
- Consumes: `appRepo.GetVolumesByAppID`, `appRepo.GetFileMountsByAppID`, `dockerClient.CopyFromContainer`, `dockerClient.CopyToContainer`, `dockerClient.StopContainer`, `dockerClient.StartContainer`, `storageRepo`
- Produces:
  - `NewAppVolumeBackupRunner(dockerClient, appRepo, storageRepo, localBaseDir) *AppVolumeBackupRunner`
  - `(r *AppVolumeBackupRunner) RunBackup(ctx context.Context, appID string) (*models.AppBackup, error)`
  - `(r *AppVolumeBackupRunner) RestoreAppVolume(ctx context.Context, backupID string) error`

- [ ] **Step 1: Write unit test in `app_volume_runner_test.go`**

Test `RunBackup` creating a valid `.tar.gz` archive of volume contents, and `RestoreAppVolume` performing safe restore (stop container -> extract -> start container).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/backup -run TestAppVolumeRunner`
Expected: FAIL (runner undefined)

- [ ] **Step 3: Implement `internal/backup/app_volume_runner.go`**

1. `RunBackup`: Read volumes & file mounts for application. Package into composite `.tar.gz`. Upload to S3 if configured. Run pruning.
2. `RestoreAppVolume`: Stop container. Download stream if S3. Extract via `CopyToContainer`. Restart container. Always ensure container restarts even if extract fails.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/backup -run TestAppVolumeRunner`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/backup/app_volume_runner.go internal/backup/app_volume_runner_test.go
git commit -m "feat(backup): add AppVolumeBackupRunner for volume snapshots and safe restore"
```

---

### Task 6: HTTP Handlers for Database Restore (In-place & New DB)

**Files:**
- Modify: `internal/modules/databases/handler.go`
- Modify: `internal/modules/databases/module.go`
- Test: `internal/modules/databases/databases_test.go`

**Interfaces:**
- Consumes: `backupRunner.RestoreBackup` from Task 4, `dbRepo.Create`
- Produces: HTTP route `POST /databases/{id}/backups/{backup_id}/restore`

- [ ] **Step 1: Write handler test in `databases_test.go`**

Test `POST /databases/{id}/backups/{backup_id}/restore`:
- Case `mode=in_place`: calls `RestoreBackup` on current DB and returns success.
- Case `mode=new_database`: creates new DB service, provisions container, restores, returns redirect.
- Case invalid mode: returns 400.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/modules/databases -run TestDatabaseRestoreHandler`
Expected: FAIL (handler endpoint undefined)

- [ ] **Step 3: Implement handler & route in `internal/modules/databases/`**

Add `RestoreBackup(w http.ResponseWriter, r *http.Request)` in `handler.go`.
Register route in `module.go`: `r.Post("/databases/{id}/backups/{backup_id}/restore", m.Handler.RestoreBackup)`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/modules/databases -run TestDatabaseRestoreHandler`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/databases/handler.go internal/modules/databases/module.go internal/modules/databases/databases_test.go
git commit -m "feat(databases): add HTTP handler and route for in-place and new-database restore"
```

---

### Task 7: HTTP Handlers for Application Volume Backups & Safe Restore

**Files:**
- Modify: `internal/modules/applications/handler.go`
- Modify: `internal/modules/applications/module.go`
- Test: `internal/modules/applications/applications_test.go`

**Interfaces:**
- Consumes: `AppVolumeBackupRunner` from Task 5, `appRepo`
- Produces:
  - `POST /applications/{id}/backups/config`
  - `POST /applications/{id}/backups/create`
  - `GET /applications/{id}/backups/{backup_id}/download`
  - `POST /applications/{id}/backups/{backup_id}/restore`
  - `POST /applications/{id}/backups/{backup_id}/delete`

- [ ] **Step 1: Write handler tests in `applications_test.go`**

Test volume backup endpoints: updating config, triggering manual backup, downloading archive, executing restore, and deleting snapshot.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/modules/applications -run TestAppVolumeBackupHandlers`
Expected: FAIL (endpoints undefined)

- [ ] **Step 3: Implement handlers & routes in `internal/modules/applications/`**

Implement handlers and wire routes in `module.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/modules/applications -run TestAppVolumeBackupHandlers`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/applications/handler.go internal/modules/applications/module.go internal/modules/applications/applications_test.go
git commit -m "feat(applications): add HTTP handlers and routes for volume backups and restore"
```

---

### Task 8: UI Integration in Templ (Database Restore Modal & App Volume Backups Card)

**Files:**
- Modify: `internal/web/pages/databases/detail.templ`
- Modify: `internal/web/pages/applications/list.templ`
- Generate: `internal/web/pages/databases/detail_templ.go`, `internal/web/pages/applications/list_templ.go`
- Test: `internal/modules/databases/databases_test.go`, `internal/modules/applications/applications_test.go`

- [ ] **Step 1: Add Restore button and Alpine.js modal in `databases/detail.templ`**

In backups table, add Restore action button. Add modal with warning banner, radio buttons for `in_place` vs `new_database`, and submit button.

- [ ] **Step 2: Add Volume Backups Card and Safe Restore modal in `applications/list.templ`**

In `#storage` tab, add card for Volume Backups: config form (cron, retention, S3 toggle), manual trigger button, snapshot history table (Date, File, Size, Storage, Status, Download/Restore/Delete actions), and Safe Restore confirmation modal.

- [ ] **Step 3: Run `templ generate` and verify compilation**

Run: `templ generate && go test ./internal/web/...`
Expected: Generated files compile with zero errors.

- [ ] **Step 4: Commit**

```bash
git add internal/web/pages/databases/ internal/web/pages/applications/
git commit -m "feat(ui): add database restore modal and application volume backups card in Templ UI"
```

---

### Task 9: End-to-End Integration Testing & Regression Suite

**Files:**
- Create: `internal/testing/backup_restore_test.go`
- Verify: Full repository test suite (`go test ./...`)

- [ ] **Step 1: Write end-to-end integration test in `backup_restore_test.go`**

Verify full flow:
1. Database backup dump -> restore to in-place database -> verify data integrity.
2. Database restore to new database -> verify new instance created and operational.
3. Application volume snapshot -> modify files in volume -> safe restore -> verify files restored.

- [ ] **Step 2: Run integration test to verify it passes**

Run: `go test -v ./internal/testing -run TestIntegration_BackupAndRestoreFlow`
Expected: PASS

- [ ] **Step 3: Run full repository test suite**

Run: `go test ./...`
Expected: 100% PASS across all packages.

- [ ] **Step 4: Commit**

```bash
git add internal/testing/backup_restore_test.go
git commit -m "test: add integration test for database and application volume backup/restore"
```
