# Application Persistent Volumes & File Mounts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menambahkan kapabilitas penyimpanan persisten (Docker Named Volumes & Host Bind Mounts) dan injeksi file konfigurasi (File Mounts) untuk aplikasi di get-labuh.

**Architecture:** Memperluas skema DB dengan tabel `app_volumes` dan `app_file_mounts`, menambahkan validasi keamanan host bind mount, implementasi penulisan file runtime di host, integrasi Docker SDK `hostCfg.Binds`, adaptasi pipeline Blue-Green dengan isolasi Ephemeral PR preview, pendaftaran HTTP endpoints, dan penambahan tab `#storage` di UI Templ.

**Tech Stack:** Go 1.22+, SQLite (`modernc.org/sqlite` / `mattn/go-sqlite3`), Docker SDK (Moby), Templ, Chi v5, HTMX, Alpine.js, Tailwind CSS.

**Spec:** `docs/superpowers/specs/2026-11-23-application-persistent-volumes-design.md`

## Global Constraints

- Go 1.22+ standard library & existing modules only (`github.com/akordium-id/get-labuh/*`).
- Database migration baru di `internal/database/migrations/028_application_volumes.sql` tanpa mengubah migration lama.
- In-memory SQLite tests wajib menggunakan `database.Connect(":memory:")` dan `database.RunAllMigrations(db)`.
- Semua file `.templ` wajib memiliki pasangan `*_templ.go` via `templ generate`.
- Error response pada HTMX request menggunakan out-of-band feedback (`HX-Trigger: showToast`).
- Zero-downtime Blue-Green deployment tidak boleh terganggu oleh adanya volume mounts.

## Review Focus

- Host path bind mount tidak boleh mengakses file sistem kritis host (`/etc`, `/var/run`, `/proc`, `/sys`, dsb.) — dicegah oleh `ValidateHostPathSecurity`.
- Nama named volume pada Ephemeral PR Previews wajib terisolasi (`...-pr-<number>-...`) agar tidak menimpa data production.
- File mount runtime direktori wajib dibuat dengan permission yang aman (`0644` untuk file, `0755` untuk direktori).
- Container candidates pada Blue-Green deployment dapat me-mount volume bersamaan tanpa error "device or resource busy".
- Path container wajib absolut dan ternormalisasi (`filepath.IsAbs`, `filepath.Clean`).

---

### Task 1: Database Migration & Domain Models

**Files:**
- Create: `internal/database/migrations/028_application_volumes.sql`
- Modify: `internal/models/application.go`
- Test: `internal/models/application_test.go`

**Interfaces:**
- Consumes: None
- Produces: `models.VolumeType`, `models.AppVolume`, `models.CreateAppVolumeInput`, `models.AppFileMount`, `models.CreateAppFileMountInput`

- [ ] **Step 1: Write failing unit test for volume models in `internal/models/application_test.go`**

```go
func TestAppVolumeModel(t *testing.T) {
	vol := models.AppVolume{
		ID:            "vol-1",
		ApplicationID: "app-1",
		Name:          "uploads",
		VolumeType:    models.VolumeTypeNamed,
		MountPath:     "/app/storage",
		IsReadOnly:    false,
	}
	assert.Equal(t, models.VolumeTypeNamed, vol.VolumeType)
	assert.Equal(t, "/app/storage", vol.MountPath)

	fileMount := models.AppFileMount{
		ID:            "fm-1",
		ApplicationID: "app-1",
		MountPath:     "/etc/nginx/nginx.conf",
		Content:       "events {}",
		FileMode:      "0644",
		IsReadOnly:    true,
	}
	assert.Equal(t, "/etc/nginx/nginx.conf", fileMount.MountPath)
	assert.True(t, fileMount.IsReadOnly)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/models -run TestAppVolumeModel`  
Expected: FAIL with undefined `models.AppVolume`

- [ ] **Step 3: Create migration `internal/database/migrations/028_application_volumes.sql`**

Buat tabel `app_volumes` dan `app_file_mounts` sesuai spec Bagian 4.1.

- [ ] **Step 4: Add struct definitions to `internal/models/application.go`**

Tambahkan `VolumeType`, `AppVolume`, `CreateAppVolumeInput`, `AppFileMount`, dan `CreateAppFileMountInput`.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test -v ./internal/models -run TestAppVolumeModel`  
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/database/migrations/028_application_volumes.sql internal/models/application.go internal/models/application_test.go
git commit -m "feat(models): add application volume and file mount models and migration"
```

---

### Task 2: Repository Layer for Volumes and File Mounts

**Files:**
- Create: `internal/modules/applications/repo_volume.go`
- Test: `internal/modules/applications/repo_volume_test.go`

**Interfaces:**
- Consumes: `models.AppVolume`, `models.CreateAppVolumeInput`, `models.AppFileMount`, `models.CreateAppFileMountInput`
- Produces:
  - `(r *Repo) CreateVolume(ctx context.Context, input models.CreateAppVolumeInput) (*models.AppVolume, error)`
  - `(r *Repo) GetVolumesByAppID(ctx context.Context, appID string) ([]*models.AppVolume, error)`
  - `(r *Repo) GetVolumeByID(ctx context.Context, id string) (*models.AppVolume, error)`
  - `(r *Repo) DeleteVolume(ctx context.Context, id string) error`
  - `(r *Repo) CreateFileMount(ctx context.Context, input models.CreateAppFileMountInput) (*models.AppFileMount, error)`
  - `(r *Repo) GetFileMountsByAppID(ctx context.Context, appID string) ([]*models.AppFileMount, error)`
  - `(r *Repo) GetFileMountByID(ctx context.Context, id string) (*models.AppFileMount, error)`
  - `(r *Repo) UpdateFileMount(ctx context.Context, id string, content string) error`
  - `(r *Repo) DeleteFileMount(ctx context.Context, id string) error`

- [ ] **Step 1: Write failing repository tests in `internal/modules/applications/repo_volume_test.go`**

Uji alur CRUD untuk `CreateVolume`, `GetVolumesByAppID`, `DeleteVolume`, serta `CreateFileMount`, `UpdateFileMount`, dan `DeleteFileMount` menggunakan database `:memory:`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/modules/applications -run TestVolumeRepo`  
Expected: FAIL with undefined methods on `Repo`

- [ ] **Step 3: Implement `repo_volume.go`**

Implementasikan query SQL ke tabel `app_volumes` dan `app_file_mounts`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/modules/applications -run TestVolumeRepo`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/applications/repo_volume.go internal/modules/applications/repo_volume_test.go
git commit -m "feat(repo): add CRUD methods for application volumes and file mounts"
```

---

### Task 3: Security Confinement & Host Path Validator

**Files:**
- Create: `internal/docker/security.go`
- Test: `internal/docker/security_test.go`

**Interfaces:**
- Consumes: None
- Produces: `ValidateHostPathSecurity(hostPath string) error`

- [ ] **Step 1: Write failing security tests in `internal/docker/security_test.go`**

Uji berbagai input path:
- Legal: `/opt/data/uploads`, `/var/myapp/storage`, `/home/deploy/data` -> return nil
- Relative path: `relative/path` -> return error (must be absolute)
- Blacklist root: `/`, `/etc`, `/etc/shadow`, `/var/run`, `/var/run/docker.sock`, `/proc`, `/sys`, `/dev` -> return error
- Traversal attack: `/var/myapp/../../etc/passwd` -> return error (after filepath.Clean)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/docker -run TestValidateHostPathSecurity`  
Expected: FAIL with undefined `ValidateHostPathSecurity`

- [ ] **Step 3: Implement `internal/docker/security.go`**

Implementasikan `ValidateHostPathSecurity(hostPath string) error` dengan `filepath.IsAbs`, `filepath.Clean`, dan blacklist pengecekan prefix direktori terlarang.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/docker -run TestValidateHostPathSecurity`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/docker/security.go internal/docker/security_test.go
git commit -m "feat(docker): add host path security confinement validator"
```

---

### Task 4: Docker Engine Integration & Runtime File Mount Writer

**Files:**
- Create: `internal/docker/volume.go`
- Modify: `internal/docker/container.go`
- Test: `internal/docker/volume_test.go`

**Interfaces:**
- Consumes: `models.AppVolume`, `models.AppFileMount`, `ValidateHostPathSecurity`
- Produces:
  - `type VolumeBinding struct { Source, Target string; IsReadOnly bool }`
  - `(c *Client) EnsureVolume(ctx context.Context, volumeName string) error`
  - `(c *Client) RemoveVolume(ctx context.Context, volumeName string) error`
  - `PrepareFileMounts(baseDir, appID string, mounts []*models.AppFileMount) ([]VolumeBinding, error)`
  - Update `RunContainerWithCustomName(..., bindings []VolumeBinding)`

- [ ] **Step 1: Write failing test in `internal/docker/volume_test.go`**

Uji `PrepareFileMounts` yang menulis file ke filesystem lokal dengan permission yang tepat, dan uji pembuatan `VolumeBinding`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/docker -run TestPrepareFileMounts`  
Expected: FAIL

- [ ] **Step 3: Implement `volume.go` and update `container.go`**

- Buat `internal/docker/volume.go` dengan `EnsureVolume`, `RemoveVolume`, dan `PrepareFileMounts`.
- Di `internal/docker/container.go`: pada `RunContainerWithCustomName`, tambahkan binding ke `hostCfg.Binds` dalam format `fmt.Sprintf("%s:%s:%s", b.Source, b.Target, mode)`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/docker -run TestPrepareFileMounts`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/docker/volume.go internal/docker/volume_test.go internal/docker/container.go
git commit -m "feat(docker): add docker volume management and file mount runtime writer"
```

---

### Task 5: Deploy Pipeline Integration & Ephemeral PR Isolation

**Files:**
- Modify: `internal/deploy/job.go`
- Test: `internal/deploy/job_test.go`

**Interfaces:**
- Consumes: `repo.GetVolumesByAppID`, `repo.GetFileMountsByAppID`, `dockerClient.EnsureVolume`, `PrepareFileMounts`
- Produces: Integrated volume & file mounts injection in candidate container creation with PR isolation

- [ ] **Step 1: Write failing test in `internal/deploy/job_test.go`**

Uji bahwa saat pipeline mendeploy aplikasi dengan volume, `hostCfg.Binds` pada container kandidat terisi dengan nama volume yang benar, dan aplikasi dengan `IsPreview = true` menghasilkan volume dengan format `...-pr-<number>-...`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/deploy -run TestDeployWithVolumes`  
Expected: FAIL

- [ ] **Step 3: Integrate volumes into `internal/deploy/job.go`**

Di `internal/deploy/job.go`:
- Ambil daftar volumes dan file mounts dari repository.
- Hitung nama volume:
  Jika preview: `fmt.Sprintf("labuh-vol-app-%s-%s-pr-%d-%s", envSlug, appSlug, *app.PRNumber, v.Name)`
  Jika normal: `fmt.Sprintf("labuh-vol-app-%s-%s-%s", envSlug, appSlug, v.Name)`
- Siapkan `VolumeBinding` dan teruskan ke `RunContainerWithCustomName`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/deploy -run TestDeployWithVolumes`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/deploy/job.go internal/deploy/job_test.go
git commit -m "feat(deploy): integrate persistent volumes and file mounts into deploy pipeline"
```

---

### Task 6: HTTP Handlers & Routes for Volumes & File Mounts

**Files:**
- Modify: `internal/modules/applications/handler.go`
- Modify: `internal/modules/applications/module.go`
- Test: `internal/modules/applications/applications_test.go`

**Interfaces:**
- Consumes: `repo_volume.go`, `ValidateHostPathSecurity`
- Produces:
  - `POST /applications/{id}/volumes`
  - `POST /applications/{id}/volumes/{vol_id}/delete`
  - `POST /applications/{id}/file-mounts`
  - `POST /applications/{id}/file-mounts/{file_id}/update`
  - `POST /applications/{id}/file-mounts/{file_id}/delete`

- [ ] **Step 1: Write failing handler tests in `internal/modules/applications/applications_test.go`**

Uji endpoint `POST /applications/{id}/volumes` (sukses dan validasi error jika host path terlarang) dan `POST /applications/{id}/file-mounts`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/modules/applications -run TestVolumeHandlers`  
Expected: FAIL

- [ ] **Step 3: Implement handler methods and register routes**

- Tambahkan method di `internal/modules/applications/handler.go`.
- Daftarkan route di `internal/modules/applications/module.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/modules/applications -run TestVolumeHandlers`  
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/applications/handler.go internal/modules/applications/module.go internal/modules/applications/applications_test.go
git commit -m "feat(applications): add HTTP handlers and routes for volumes and file mounts"
```

---

### Task 7: Templ UI Storage Tab & Forms

**Files:**
- Modify: `internal/web/pages/applications/list.templ`
- Modify: `internal/modules/applications/handler.go` (pass `volumes` and `fileMounts` to `ApplicationDetailPage`)
- Run: `templ generate`

**Interfaces:**
- Consumes: `volumes []*models.AppVolume`, `fileMounts []*models.AppFileMount`
- Produces: Tab `#storage` on Application Detail Page

- [ ] **Step 1: Update `ApplicationDetailPage` signature in `internal/web/pages/applications/list.templ`**

Tambahkan parameter `volumes []*models.AppVolume, fileMounts []*models.AppFileMount`.

- [ ] **Step 2: Add `#storage` tab button in navigation bar**

Tambahkan link tab `#storage` dengan icon database/harddrive dan counter badge `formatInt(len(volumes) + len(fileMounts))`.

- [ ] **Step 3: Add tab content card for Persistent Volumes & File Mounts**

- Card 1: Persistent Volumes (Named & Host Bind) dengan tabel dan form modal/tambah.
- Card 2: File Mounts dengan monospaced textarea config editor.

- [ ] **Step 4: Run `templ generate` and compile check**

Run: `templ generate && go test -c ./internal/web/pages/applications`  
Expected: PASS (0 errors)

- [ ] **Step 5: Commit**

```bash
git add internal/web/pages/applications/list.templ internal/web/pages/applications/list_templ.go internal/modules/applications/handler.go
git commit -m "feat(ui): add storage tab for persistent volumes and file mounts in application detail page"
```

---

### Task 8: End-to-End System Verification

**Files:**
- Modify: `internal/testing/integration_test.go`

**Interfaces:**
- Consumes: Full system stack
- Produces: Passing end-to-end integration tests & complete repository test suite

- [ ] **Step 1: Write integration test verifying persistent volume and file mount execution**

Uji bahwa deployment aplikasi dengan named volume dan file mount mengonfigurasi Docker engine dengan binds yang tepat.

- [ ] **Step 2: Run all tests in the repository**

Run: `go test ./...`  
Expected: PASS (100% tests passing across all packages)

- [ ] **Step 3: Commit**

```bash
git add internal/testing/integration_test.go
git commit -m "test: add integration test for application persistent volumes and file mounts"
```
