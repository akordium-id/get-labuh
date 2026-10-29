# Module: Deploy Pipeline

Package `internal/deploy` — orchestrates the full deployment lifecycle.

## Tanggung Jawab

- Menjalankan pipeline deployment dari awal sampai traffic live
- Mendukung 6 build modes: `git`, `docker_image`, `dockerfile`, `nixpacks`, `docker_compose`, `dropzone`
- Blue-Green deployment dengan Healthcheck Gate (timeout 30s)
- Rollback protektif jika healthcheck gagal

## Tipe Utama

```go
// Job descriptor — dibuat oleh handler, di-queue ke DeployWorker
type DeploymentJob struct {
    DeploymentID   string
    ApplicationID  string
    SourceType     string       // salah satu dari 6 build modes
    RepositoryURL  string
    Branch         string
    DockerfilePath string
    BuildPath      string
    DockerImage    string
    AppPort        int
    EnvVars        []models.AppEnvVar
    LogPath        string
    Priority       int
    AppSlug        string
    CustomDomain   *string
    ExtraLogWriter io.Writer
    StatusCallback func(step string, status string, err error)
}

// Orchestrator — dibuat sekali di main.go, dipakai oleh worker
type Pipeline struct {
    dockerClient *docker.Client
    caddyClient  *caddy.Client
    settingRepo  *repo.SettingRepo
    appRepo      *repo.ApplicationRepo
    deployRepo   *repo.DeploymentRepo
}
```

## Alur Blue-Green (Run method di Pipeline)

```
1. Clone repo (atau skip jika docker_image)
2. Build image (Nixpacks CLI / docker build / direct pull)
3. Start kandidat container di labuh-network (nama: <slug>-candidate)
4. Healthcheck loop (HTTP GET / atau TCP connect, timeout 30s)
   - Berhasil → lanjut ke step 5
   - Gagal    → stop & remove kandidat → rollback (tidak sentuh container aktif)
5. Caddy route switch: arahkan traffic ke IP kandidat (atomic via Caddy API)
6. Stop & remove container versi sebelumnya
7. Rename kandidat → nama resmi (<slug>)
```

## Dependensi Package

- `internal/docker` — Docker SDK wrapper (run, build, stop, remove, healthcheck)
- `internal/caddy` — Caddy Admin API (set/delete upstream route)
- `internal/database/repo` — update status deployment + application
- `internal/models` — DeploymentJob, AppEnvVar, DeployStatus

## Status Enum (models.DeployStatus)

| Status | Keterangan |
|--------|-----------|
| `queued` | Job masuk antrian worker |
| `cloning` | Sedang clone repo |
| `building` | Sedang build image |
| `deploying` | Sedang start container kandidat |
| `success` | Traffic dialihkan, deployment selesai |
| `failed` | Pipeline gagal di step mana pun |
| `cancelled` | Job dibatalkan sebelum diproses |

## Rules untuk AI Agent

1. **Rollback wajib** — jika healthcheck gagal, kandidat container harus selalu di-stop dan di-remove sebelum return error
2. **Jangan buffer log di memory** — gunakan `io.Writer` (ke file log) secara streaming
3. `StatusCallback` dipanggil di setiap transisi step — handler SSE bergantung pada ini
4. Cleanup kandidat juga harus terjadi jika `context` di-cancel (graceful shutdown)
5. Container resource limits (CPU/RAM) diapply saat `docker run` kandidat, bukan saat rename
