# Design Spec: Automated Docker Disk Cleanup (Scheduled Prune)

## Status: Approved
**Author**: Faiq & Antigravity  
**Date**: 2026-11-06  
**Target Project**: `get-labuh`

---

## 1. Overview & Objective

Fitur **Automated Docker Disk Cleanup (Scheduled Prune)** menyediakan sistem pembersihan otomatis dan manual untuk sumber daya Docker yang tidak terpakai di server host get-labuh (terinspirasi fitur Server Cleanup pada Coolify v4).

Tujuan utama fitur ini:
1. **Mencegah Disk VPS Penuh**: Membersihkan layer build cache Docker (Nixpacks/Dockerfile), dangling images (tanpa tag), stopped candidate containers, dan bridge networks yang tidak terpakai.
2. **Proteksi Cache Instant Rollback (Safe Pruning)**:
   - Image prune secara ketat menggunakan filter `dangling=true`.
   - Tagged images (termasuk 5 cached images versi sebelumnya per aplikasi untuk instant rollback 1-klik) **tidak disentuh** sehingga reliabilitas rollback zero-downtime tetap 100% terjaga.
3. **Fleksibilitas Penjadwalan & Manual Trigger**:
   - Mendukung cron scheduler otomatis (preset harian, mingguan, atau custom cron expression).
   - Menyediakan tombol manual 1-klik *"Run Cleanup Now"* dari dashboard dengan feedback metrik berapa MB/GB ruang disk yang berhasil dibebaskan.

---

## 2. Architecture & Components

```
┌───────────────────────────────────────────────┐
│              UI & HTTP Handlers               │
│ - GET  /settings/cleanup (Dashboard status)   │
│ - POST /settings/cleanup (Save schedule config│
│ - POST /settings/cleanup/run (Manual trigger) │
└──────────────────────┬────────────────────────┘
                       │
       ┌───────────────┴───────────────┐
       ▼                               ▼
┌──────────────────────────┐     ┌──────────────────────────┐
│  Background Scheduler    │     │       Manual Run         │
│  (CleanupScheduler)      │     │       (HTTP Call)        │
│  - Ticker loop (1 min)   │     │                          │
│  - Cron match check      │     │                          │
└──────────────┬───────────┘     └─────────────┬────────────┘
               │                               │
               └───────────────┬───────────────┘
                               ▼
┌───────────────────────────────────────────────────────────┐
│              docker.Client.PruneSystem                    │
│ 1. ContainersPrune (stopped temporary/candidate)          │
│ 2. ImagesPrune (filters: dangling=true)                   │
│ 3. BuildCachePrune (All: true)                            │
│ 4. NetworksPrune (unused bridge networks)                 │
│ 5. Calculate total SpaceReclaimed & format string         │
└──────────────────────────────┬────────────────────────────┘
                               ▼
┌───────────────────────────────────────────────────────────┐
│                    Persistence & Logs                     │
│ - settings table:                                         │
│   * docker_cleanup_last_run (RFC3339)                     │
│   * docker_cleanup_last_reclaimed (e.g. "1.42 GB")        │
│ - audit_logs: "Docker system cleanup: X reclaimed"        │
└───────────────────────────────────────────────────────────┘
```

---

## 3. Data Structures & Docker Prune Subsystem

### 3.1 Prune Options & Report (`internal/docker/cleanup.go`)

```go
package docker

import (
	"context"
	"time"
)

type PruneOptions struct {
	PruneContainers bool `json:"prune_containers"`
	PruneImages     bool `json:"prune_images"`
	PruneBuildCache bool `json:"prune_build_cache"`
	PruneNetworks   bool `json:"prune_networks"`
}

type PruneReport struct {
	ContainersDeleted []string  `json:"containers_deleted"`
	ImagesDeleted     []string  `json:"images_deleted"`
	SpaceReclaimed    uint64    `json:"space_reclaimed_bytes"`
	SpaceFormatted    string    `json:"space_reclaimed_formatted"`
	Timestamp         time.Time `json:"timestamp"`
}

func DefaultPruneOptions() PruneOptions {
	return PruneOptions{
		PruneContainers: true,
		PruneImages:     true,
		PruneBuildCache: true,
		PruneNetworks:   true,
	}
}
```

### 3.2 Method `PruneSystem` Execution Details
- Menggunakan `c.cli.ContainersPrune(ctx, filters.NewArgs())`.
- Menggunakan `c.cli.ImagesPrune(ctx, filters.NewArgs(filters.Arg("dangling", "true")))`.
- Menggunakan `c.cli.BuildCachePrune(ctx, types.BuildCachePruneOptions{All: true})`.
- Menggunakan `c.cli.NetworksPrune(ctx, filters.NewArgs())`.
- Menghitung akumulasi byte ruang disk yang dibebaskan (`report.SpaceReclaimed`) dan memformat ke format terstruktur (`B`, `KB`, `MB`, `GB`).

---

## 4. Background Scheduler Daemon (`internal/modules/settings/scheduler.go`)

- **Struct**:
  ```go
  type CleanupScheduler struct {
      dockerClient  *docker.Client
      settingRepo   *repo.SettingRepo
      auditRepo     *repo.AuditLogRepo
      ticker        *time.Ticker
      stopCh        chan struct{}
      running       bool
      mu            sync.Mutex
  }
  ```
- **Lifecycle**:
  - `Start()`: memulai background goroutine per 1 menit.
  - `Stop()`: menghentikan ticker secara graceful.
  - `RunCleanup(ctx context.Context) (*docker.PruneReport, error)`: dapat dipanggil baik oleh scheduler maupun HTTP manual trigger.
- **Konfigurasi Settings**:
  - `docker_cleanup_enabled`: `"true"` (default) / `"false"`.
  - `docker_cleanup_schedule`: `"0 0 * * *"` (default harian jam 00:00).
  - `docker_cleanup_last_run`: Timestamp RFC3339.
  - `docker_cleanup_last_reclaimed`: String ukuran ruang disk yang dibebaskan.

---

## 5. Web Interface & Handlers

### 5.1 Route Handlers (`internal/modules/settings/handler.go`)
- `GET /settings/cleanup`: Menampilkan halaman `CleanupPage`.
- `POST /settings/cleanup`: Menyimpan konfigurasi toggle `docker_cleanup_enabled` dan jadwal `docker_cleanup_schedule`.
- `POST /settings/cleanup/run`: Menjalankan pembersihan instan secara manual, mengembalikan toast notification `"Pembersihan selesai: X ruang disk berhasil dibebaskan"` dan reload/redirect ke `/settings/cleanup`.

### 5.2 Templ View (`internal/web/pages/settings/cleanup.templ`)
- Sub-navigasi / sidebar settings: Link ke `Docker Cleanup`.
- Card 1: **Status & Riwayat**:
  - Waktu eksekusi terakhir.
  - Ruang disk yang berhasil dibebaskan pada pembersihan terakhir.
  - Tombol manual *"Run Cleanup Now"* dengan loading feedback.
- Card 2: **Jadwal Otomatisasi (Cron Prune)**:
  - Toggle aktifkan pembersihan berkala.
  - Pilihan preset jadwal:
    - Daily at Midnight (`0 0 * * *`)
    - Weekly on Sunday (`0 0 * * 0`)
    - Custom Cron Expression.
  - Form submit dengan feedback toast.

---

## 6. Testing Plan

1. **Docker Subsystem Unit Tests (`internal/docker/cleanup_test.go`)**:
   - Test method `PruneSystem` ketika Docker client initialized vs dormant.
   - Test helper `FormatBytes`.
2. **Scheduler Unit Tests (`internal/modules/settings/scheduler_test.go`)**:
   - Test eksekusi `RunCleanup` dan penyimpanan setting `last_run` serta `last_reclaimed`.
3. **HTTP Handler Unit Tests (`internal/modules/settings/settings_test.go`)**:
   - Test `GET /settings/cleanup`.
   - Test `POST /settings/cleanup` (simpan opsi).
   - Test `POST /settings/cleanup/run` (manual trigger).
4. **Full Suite Verification**:
   - `go test ./...` dan `go build ./cmd/labuh`.
