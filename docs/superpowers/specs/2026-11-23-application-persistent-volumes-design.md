# Design Spec: Application Persistent Volumes & File Mounts

## Status: Approved
**Author**: Faiq & Antigravity  
**Date**: 2026-11-23  
**Target Project**: `get-labuh`

---

## 1. Overview & Objective

Fitur **Application Persistent Volumes & File Mounts** menambahkan kemampuan pengelolaan penyimpanan persisten (stateful) dan injeksi file konfigurasi untuk container aplikasi di `get-labuh`.

### Masalah Saat Ini
Sebelum fitur ini, `internal/docker/container.go` (`RunContainerWithCustomName`) tidak menyertakan konfigurasi binding volume (`hostCfg.Binds` kosong). Akibatnya:
- Aplikasi stateful (mis. direktori upload Laravel `storage/app/public`, WordPress `wp-content/uploads`, PocketBase/SQLite, Ghost) kehilangan seluruh data yang ditulis di runtime setiap kali aplikasi di-redeploy atau di-rollback.
- Pengguna tidak dapat menyisipkan file konfigurasi kustom (seperti `nginx.conf`, custom `php.ini`, atau lisensi/sertifikat) tanpa mem-build ulang image Docker.

### Tujuan & Solusi
1. **Docker Named Volumes (`volume_type: "named"`)**:
   - Volume persisten yang dikelola oleh Docker Engine.
   - Format penamaan otomatis: `labuh-vol-app-<envSlug>-<appSlug>-<volName>`.
   - Terisolasi penuh pada Ephemeral PR Previews dengan format: `labuh-vol-app-<envSlug>-<appSlug>-pr-<prNumber>-<volName>`.
2. **Host Path Bind Mounts (`volume_type: "bind"`)**:
   - Me-mount direktori host langsung ke dalam container (`host_path:mount_path`).
   - Dilindungi oleh strict security confinement (path blacklist) untuk mencegah privilege escalation atau akses ke file sistem kritis host.
3. **Single File Mounts / Config Injection (`app_file_mounts`)**:
   - Pengguna dapat mengetikkan atau menempelkan konten teks konfigurasi langsung dari UI.
   - Disimpan di database, dan secara otomatis ditulis ke host pada runtime sebelum container dijalankan, lalu di-bind mount sebagai file read-only (`ro`) atau read-write (`rw`).

---

## 2. Architecture & Container Runtime Integration

### 2.1 Siklus Hidup Runtime & Binding

```
[Start Deploy / Candidate Container]
        │
        ▼
[Fetch Volumes & File Mounts from DB]
        │
        ▼
┌────────────────────────────────────────────────────────┐
│ 1. Process Named Volumes                               │
│    - Compute VolumeName (with PR isolation if preview) │
│    - dockerClient.EnsureVolume(ctx, volumeName)        │
│    - Append to Binds: "volumeName:/container/path:rw"  │
├────────────────────────────────────────────────────────┤
│ 2. Process Host Bind Mounts                            │
│    - ValidateHostPathSecurity(hostPath)                │
│    - Ensure host directory exists (mkdir -p)           │
│    - Append to Binds: "/host/path:/container/path:rw"  │
├────────────────────────────────────────────────────────┤
│ 3. Process File Mounts                                 │
│    - Compute host path: {runtimeDir}/{appID}/{fileHash}│
│    - Write file content to disk with file_mode (0644)  │
│    - Append to Binds: "/host/file:/container/file:ro"  │
└────────────────────────────────────────────────────────┘
        │
        ▼
[Set hostCfg.Binds in Docker CreateContainer]
        │
        ▼
[Start Container with Zero-Downtime Blue-Green Pipeline]
```

### 2.2 Kompatibilitas Blue-Green Deployment
- Docker named volume dan bind mount mendukung *concurrent access* oleh multiple container secara native.
- Saat container kandidat (Blue) dinyalakan berdampingan dengan container aktif (Green), keduanya me-mount volume yang sama.
- Setelah healthcheck lulus dan traffic dialihkan via Caddy, container Green dimatikan. Data volume tetap utuh dan persisten di container Blue.
- Jika kandidat Blue gagal healthcheck, container Blue dihapus dan volume tidak terpengaruh, container Green tetap melayani user normal.

### 2.3 Isolasi Ephemeral PR Previews
- Jika aplikasi merupakan PR Preview (`app.IsPreview == true` dan `app.PRNumber != nil`):
  - Nama volume menggunakan suffix PR: `labuh-vol-app-<envSlug>-<appSlug>-pr-<prNumber>-<volName>`.
  - Hal ini menjamin pengujian fitur di PR tidak menimpa atau merusak data production.
  - Saat PR ditutup/merged dan container preview di-teardown, volume PR ini otomatis di-prune/dibersihkan.

---

## 3. Security Confinement (Host Path Validator)

Untuk mencegah ancaman keamanan seperti *directory traversal* atau mounting file sistem kritis host yang dapat mengarah pada kompromi root server:

### 3.1 Aturan Validasi (`ValidateHostPathSecurity`)
1. **Wajib Path Absolut**: `filepath.IsAbs(p)` harus bernilai `true`.
2. **Sanitasi Path**: Menggunakan `filepath.Clean(p)` untuk mengeliminasi `../` atau symlink traversal.
3. **Blacklist Direktori Sistem Root**:
   Path tidak boleh sama dengan atau berada di bawah subdirektori berikut:
   - `/` (root sistem)
   - `/etc`
   - `/var/run` (termasuk `/var/run/docker.sock`)
   - `/proc`
   - `/sys`
   - `/dev`
   - `/boot`
   - `/root`
   - `/usr`
   - `/lib`, `/lib64`
   - `/bin`, `/sbin`
   - Direktori database lokal Labuh (mis. direktori tempat SQLite Labuh berada)

---

## 4. Database Schema & Data Models

### 4.1 Migrasi Database (`internal/database/migrations/028_application_volumes.sql`)

```sql
-- Migration 028: Tambahkan dukungan Persistent Volumes dan File Mounts untuk Applications

CREATE TABLE app_volumes (
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

CREATE INDEX idx_app_volumes_application_id ON app_volumes(application_id);

CREATE TABLE app_file_mounts (
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

CREATE INDEX idx_app_file_mounts_application_id ON app_file_mounts(application_id);
```

### 4.2 Domain Models (`internal/models/application.go`)

```go
type VolumeType string

const (
	VolumeTypeNamed VolumeType = "named"
	VolumeTypeBind  VolumeType = "bind"
)

type AppVolume struct {
	ID            string     `json:"id"`
	ApplicationID string     `json:"application_id"`
	Name          string     `json:"name"`
	VolumeType    VolumeType `json:"volume_type"`
	HostPath      *string    `json:"host_path"`
	MountPath     string     `json:"mount_path"`
	IsReadOnly    bool       `json:"is_read_only"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type CreateAppVolumeInput struct {
	ApplicationID string     `json:"application_id"`
	Name          string     `json:"name"`
	VolumeType    VolumeType `json:"volume_type"`
	HostPath      *string    `json:"host_path"`
	MountPath     string     `json:"mount_path"`
	IsReadOnly    bool       `json:"is_read_only"`
}

type AppFileMount struct {
	ID            string    `json:"id"`
	ApplicationID string    `json:"application_id"`
	MountPath     string    `json:"mount_path"`
	Content       string    `json:"content"`
	FileMode      string    `json:"file_mode"`
	IsReadOnly    bool      `json:"is_read_only"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CreateAppFileMountInput struct {
	ApplicationID string `json:"application_id"`
	MountPath     string `json:"mount_path"`
	Content       string `json:"content"`
	FileMode      string `json:"file_mode"`
	IsReadOnly    bool   `json:"is_read_only"`
}
```

---

## 5. Repository Layer (`internal/modules/applications/repo.go`)

Tambahkan method pada `Repo` (atau submodule `volume_repo.go`):
- `CreateVolume(ctx, input CreateAppVolumeInput) (*models.AppVolume, error)`
- `GetVolumesByAppID(ctx, appID string) ([]*models.AppVolume, error)`
- `DeleteVolume(ctx, volumeID string) error`
- `CreateFileMount(ctx, input CreateAppFileMountInput) (*models.AppFileMount, error)`
- `GetFileMountsByAppID(ctx, appID string) ([]*models.AppFileMount, error)`
- `UpdateFileMount(ctx, id string, content string) error`
- `DeleteFileMount(ctx, fileMountID string) error`

---

## 6. Docker Client Layer (`internal/docker/`)

1. **Volume Management**:
   - `EnsureVolume(ctx context.Context, volumeName string) error`
   - `RemoveVolume(ctx context.Context, volumeName string) error`
2. **Mount Preparation**:
   - Menambahkan struct `VolumeBinding`:
     ```go
     type VolumeBinding struct {
         Source     string // VolumeName atau HostPath atau RuntimeFilePath
         Target     string // Container MountPath
         IsReadOnly bool
     }
     ```
   - Di `RunContainerWithCustomName`, `hostCfg.Binds` diisi dari daftar `VolumeBinding`.
3. **File Mount Writer**:
   - Helper function `PrepareFileMounts(appID string, mounts []*models.AppFileMount) ([]VolumeBinding, error)`:
     - Direktori penyimpanan: `/var/lib/labuh/app-files/<appID>/` (atau fallback `.labuh/app-files/...`).
     - Setiap file ditulis dengan nama SHA-256 hash dari `MountPath`.
     - File diatur permissions-nya sesuai `FileMode` (default `0644`).

---

## 7. HTTP Endpoints & Handler (`internal/modules/applications/handler.go`)

### 7.1 Routes
- `POST /applications/{id}/volumes` — Tambah volume
- `DELETE /applications/{id}/volumes/{volumeId}` — Hapus volume
- `POST /applications/{id}/file-mounts` — Tambah file mount
- `PUT /applications/{id}/file-mounts/{fileId}` — Update isi file mount
- `DELETE /applications/{id}/file-mounts/{fileId}` — Hapus file mount

### 7.2 Notifikasi & UX
- Setiap operasi mutasi merespons dengan header HTMX `HX-Trigger: showToast` untuk feedback instan.
- Render partial tabel storage untuk update tanpa reload halaman penuh.

---

## 8. UI Layer (`internal/web/pages/applications/list.templ`)

1. **Tab Navigasi Baru**:
   - Tambahkan tab `#storage` di bar navigasi tab:
     ```html
     <a href="#storage" @click="tab = 'storage'" ...>
         <span>Storage</span>
         <span class="badge">{ formatInt(len(volumes) + len(fileMounts)) }</span>
     </a>
     ```
2. **Konten Tab `#storage`**:
   - **Card 1: Persistent Volumes**:
     - Form: Name, Volume Type (Named / Host Bind), Host Path (kondisional jika Bind), Mount Path di Container, Read-only checkbox.
     - Tabel daftar volume terpasang: badge tipe (Named/Bind), nama volume aktual di Docker, path container, status permission, dan tombol hapus.
   - **Card 2: File Mounts (Config Editor)**:
     - Form: Container Path (`/etc/custom.conf`), Textarea monospaced editor untuk isi konfigurasi.
     - Daftar file konfigurasi aktif dengan modal/collapsible editor untuk mengedit isi file secara langsung.

---

## 9. Verification & Testing Plan

1. **Unit Tests**:
   - `ValidateHostPathSecurity`: Menguji validasi path legal vs path sensitif yang diblacklist (`/etc/shadow`, `/var/run/docker.sock`, path traversal `../../`).
   - `PrepareFileMounts`: Menguji pembuatan file di host, penulisan konten teks, dan mode permissions.
   - `Repo`: Menguji CRUD `app_volumes` dan `app_file_mounts` pada SQLite in-memory.
2. **Integration Tests**:
   - Menjalankan container dummy dengan named volume dan memastikan file yang ditulis di dalam container tetap ada saat container di-restart/re-created dengan nama volume yang sama.
   - Menguji file mount terbaca di dalam container sesuai konten yang di-inject.
3. **Test Suite Verification**:
   - Seluruh test di repo harus lulus 100%: `go test ./...`
