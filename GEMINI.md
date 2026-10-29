# get-labuh

Platform deployment self-hosted (alternatif Coolify, single-node). Dibangun di Go.

## Stack

| Layer | Tech |
|-------|------|
| HTTP framework | Chi v5 |
| UI | Templ + HTMX + Alpine.js |
| Database | SQLite (via `modernc.org/sqlite`) |
| Container runtime | Docker SDK (`docker/docker`) via `/var/run/docker.sock` |
| Reverse proxy | Caddy (dikontrol via Admin API) |
| Build | Nixpacks CLI atau Dockerfile |
| Auth | Session-based (cookie) + API Key untuk REST |

## Struktur Direktori

```
cmd/labuh/main.go          ← entry point, DI wiring
internal/
  api/                     ← REST API (Chi router, API Key auth)
  auth/                    ← session auth, middleware RequireAuth
  backup/                  ← database backup runner + scheduler
  caddy/                   ← Caddy Admin API client
  database/                ← SQLite connect + migrations
    repo/                  ← data access layer (21 repo files)
  deploy/                  ← deploy pipeline (Blue-Green + build modes)
  docker/                  ← Docker SDK client wrapper
  encryption/              ← AES enkripsi secrets env var
  handler/                 ← HTTP handlers UI (35 files, flat per domain)
  middleware/              ← Security, CSRF, RateLimit, Mode, AuditLog
  models/                  ← domain structs (no external deps)
  notification/            ← Discord & Telegram notifier
  observability/           ← Prometheus metrics, request ID
  runner/                  ← runner mode (background process)
  server/                  ← graceful shutdown helper
  validation/              ← input validation helpers
  web/                     ← Templ UI layer
    components/            ← shared UI components
    layouts/               ← page layouts (AppLayout)
    pages/                 ← pages per domain
migrations/                ← SQL migration files
```

## Peta Domain

| Domain | Handler | Repo | Model |
|--------|---------|------|-------|
| Applications | `handler/applications.go` | `repo/application.go` | `models/application.go` |
| Deployments | `handler/deployments.go` | `repo/deployment.go` | `models/application.go` (Deployment) |
| Databases | `handler/databases.go` | `repo/database.go` | `models/database.go` |
| Compose | `modules/compose/handler.go` | `modules/compose/repo.go` | `models/compose.go` |
| Projects | `modules/projects/handler.go` | `modules/projects/repo.go` | `models/project.go` |
| Notifications | `modules/notifications/handler.go` | `modules/notifications/repo.go` | `models/notification.go` |
| Auth | `modules/auth/handler.go` | `modules/auth/repo.go` | `models/user.go` |
| Settings & Backup | `modules/settings/handler.go` | `modules/settings/repo.go` | `models/setting.go` |

## Konvensi Kode

- **models**: pure domain structs, tidak boleh import package internal lain
- **repo**: hanya query SQL, return model atau error — tidak ada business logic
- **handler**: terima HTTP request → validasi → panggil repo/service → render Templ atau JSON
- **deploy/job.go**: Pipeline struct sebagai orchestrator Blue-Green deployment
- **worker**: DeployWorker queue-based, max 3 concurrent job via semaphore
- Error response UI via HTMX out-of-band swap (`HX-Trigger: showToast`)
- Semua secret env var dienkripsi di DB (`encryption.Encrypt/Decrypt`)

## Deploy Pipeline (5 tahap)

```
Clone → Build → Deploy → Healthcheck → Traffic Live
```

- **Blue-Green**: kandidat container baru dijalankan berdampingan, baru traffic dialihkan via Caddy API setelah healthcheck 30s pass
- Rollback otomatis kalau healthcheck gagal — container aktif tidak dimatikan
- Build modes: `docker_image`, `dockerfile`, `nixpacks`, `git`, `docker_compose`, `dropzone`

## UI Mode

- **Pro** (default): full dashboard developer
- **Lite**: card view aplikasi — toggle via `/settings/mode`, disimpan di cookie `labuh_mode`

## Rules untuk AI Agent

1. Jangan ubah signature fungsi di `models/` tanpa update semua repo yang menggunakannya
2. Migration baru: tambah file di `migrations/`, jangan edit migration lama
3. Handler baru: daftarkan route di `cmd/labuh/main.go` (bagian `r.Group` dengan `authMiddleware`)
4. Templ file selalu punya pasangan `*_templ.go` — jangan edit `*_templ.go` langsung, jalankan `templ generate`
5. Test DB menggunakan SQLite in-memory (`database.Connect(":memory:")`) dari `internal/testing/`
