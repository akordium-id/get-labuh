# Module: HTTP Handlers (UI)

Package `internal/handler` — HTTP handlers untuk web UI (server-side rendering via Templ + HTMX).

## Peta Handler

| File | Domain | Route prefix |
|------|--------|-------------|
| `applications.go` | App lifecycle, deploy trigger, env vars, domain, resource limits | `/applications/`, `/projects/{id}/environments/{env_id}/applications` |
| `databases.go` | Database provisioning (Postgres/MySQL/Redis/MongoDB), lifecycle | `/projects/{id}/environments/{env_id}/databases` |
| `deployments.go` | Deployment list + detail | `/deployments/` |
| `logs.go` | Build logs SSE stream, container logs | `/deployments/{id}/logs/stream` |
| `webhooks.go` | GitHub/GitLab webhook auto-deploy | `/webhooks/` |
| `projects.go` | Project CRUD | `/projects/` |
| `backup.go` | Backup config + trigger manual | `/backup/` |
| `compose.go` | Docker Compose app management | `/compose-apps/` |
| `notifications.go` | Notification channel config | `/notifications/` |
| `templates.go` | One-Click Service Catalog | `/templates/` |
| `terminal.go` | WebSocket container exec (xterm.js) | `/terminal/` |
| `monitoring.go` | Live stats (CPU/RAM) via SSE | `/applications/{id}/stats` |
| `storage_destinations.go` | S3 backup destinations | `/storage-destinations/` |
| `settings.go` | System settings, mode switch | `/settings/` |
| `auth.go` | Login/logout | `/auth/` |
| `deploy_keys.go` | SSH deploy keys per project | `/projects/{id}/deploy-keys` |
| `api_keys.go` | API key management | `/api-keys/` |
| `teams.go` | Team management | `/teams/` |
| `audit.go` | Audit log viewer | `/audit/` |
| `toast.go` | HTMX toast trigger helper | — |
| `dropzone.go` | Magic Dropzone ZIP upload + deploy | `/dropzone/` |
| `servers.go` | Remote server management (future) | `/servers/` |

## Pola Umum Handler

```go
type ApplicationsHandler struct {
    appRepo      *repo.ApplicationRepo
    deployRepo   *repo.DeploymentRepo
    // ... dependensi lain
    deployWorker DeployWorkerInterface  // interface, bukan concrete type
}

func NewApplicationsHandler(appRepo, projectRepo, deployRepo, ...) *ApplicationsHandler

// Pattern render Templ:
func (h *ApplicationsHandler) Get(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    app, err := h.appRepo.GetByID(id)
    if err != nil { /* handle 404/500 */ }
    templ.Handler(layouts.AppLayout(pages.ApplicationDetail(app))).ServeHTTP(w, r)
}

// Pattern HTMX partial (return fragment HTML, bukan full page):
func (h *ApplicationsHandler) Status(w http.ResponseWriter, r *http.Request) {
    // return hanya komponen kecil, bukan full page
    templ.Handler(components.StatusBadge(app.Status)).ServeHTTP(w, r)
}
```

## HTMX Conventions

- **Toast notification**: trigger via response header `HX-Trigger: {"showToast": {"type":"success","message":"..."}}`
- **Partial update**: endpoint khusus yang return fragment Templ (bukan full page)
- **Polling**: gunakan `hx-trigger="every 2s"` di Templ component
- **SSE**: endpoint `**/stream` menggunakan `text/event-stream`, tidak melalui HTMX biasa

## Rules untuk AI Agent

1. Handler baru wajib didaftarkan di `cmd/labuh/main.go` (bukan auto-register)
2. Jangan akses `docker`, `caddy`, atau `encryption` langsung dari handler — melalui repo atau service layer
3. **Exception**: `ApplicationsHandler`, `DatabasesHandler`, `TerminalHandler` boleh pegang `*docker.Client` karena butuh real-time container ops
4. CSRF token sudah dihandle oleh `middleware.CSRFProtectionMiddleware` — jangan tambahkan manual
5. Audit log otomatis via `middleware.AuditLoggingMiddleware` — tidak perlu dipanggil dari handler
6. Untuk response error di HTMX context, gunakan `handler.RespondError(w, r, statusCode, message)` bukan `http.Error`
