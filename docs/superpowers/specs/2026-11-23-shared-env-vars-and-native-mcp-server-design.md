# Sub-Project 4 Design Spec: Shared Environment Variables & Native Go MCP Server

**Status:** Approved  
**Date:** 2026-11-23  
**Authors:** Faiq & Antigravity  
**Target:** get-labuh (PaaS Self-Hosted Deployment Engine in Go)

---

## 1. Overview & Motivation

get-labuh telah berhasil mengimplementasikan kapabilitas deploy modern (Blue-Green, volume mounts, backup/restore, observabilitas host procfs, dan alert proactive). Namun, terdapat dua celah fungsional dan operasional yang signifikan:

1. **Shared Environment Variables**:
   Aplikasi mikroservis di dalam sebuah proyek (misal: frontend, backend API, worker) saat ini harus menduplikasi variabel lingkungan (seperti `DATABASE_URL`, `REDIS_HOST`, `JWT_SECRET`) secara manual di masing-masing konfigurasi aplikasi. get-labuh membutuhkan sistem pewarisan variabel lingkungan berjenjang (*Team* → *Project* → *Application* override) yang aman dan terenkripsi.
2. **Native Model Context Protocol (MCP) Server**:
   Integrasi AI tooling saat ini hanya dapat berinteraksi via REST API atau web scraping UI. Dengan menyediakan server MCP native berbasis Go (`github.com/mark3labs/mcp-go`), pengembang dapat menghubungkan agen AI seperti Claude Code, Cursor, Antigravity, atau Claude Desktop langsung ke get-labuh melalui transport ganda:
   - **HTTP SSE** (`/mcp/sse` via Chi router, diproteksi Bearer API Key).
   - **Stdio** (`labuh mcp` subcommand CLI) untuk eksekusi lokal atau via SSH pipe.

---

## 2. Scope & Non-Goals

### In Scope
- **Part A: Shared Environment Variables**:
  - Migrasi skema database SQLite `032_shared_env_vars.sql`.
  - Model domain `models.SharedEnvVar` dan `models.ResolvedEnvVar`.
  - Repositori `repo.SharedEnvVarRepo` dengan enkripsi AES-256 GCM.
  - Cascading `EnvResolver` engine dengan aturan override: `App > Project > Team`.
  - Integrasi resolver ke `DeployWorker` dan Docker container execution.
  - UI Templ di halaman Project Detail untuk mengelola shared env vars.
- **Part B: Native Go MCP Server**:
  - Integrasi library `github.com/mark3labs/mcp-go`.
  - Package `internal/mcp/` dengan arsitektur modular.
  - Pendaftaran 15 tools inti: manajemen aplikasi, deployment, log streaming, rollback, shared/app env, database backup, dan server host metrics.
  - Dual transport: HTTP SSE endpoint `/mcp/sse` & `/mcp/message` + CLI stdio subcommand `labuh mcp`.
  - Otentikasi API Key untuk HTTP SSE transport.
- Pengujian unit, integrasi, concurrency race check, dan E2E test.

### Non-Goals
- Tidak mendukung rekursif ekspresi sintaks shell kompleks seperti `${VAR_NAME}` pada rilis ini (pure key-value cascading inheritance yang deterministik dan zero-side-effects).
- Tidak mengimplementasikan multi-tenancy auth berjenjang baru di level MCP (semua request diotentikasi menggunakan API Key Labuh yang sudah ada dengan metadata `isReadOnly`/`isDestructive`).

---

## 3. Architecture & Component Diagram

```
┌────────────────────────────────────────────────────────────────────────┐
│                              get-labuh                                 │
├──────────────────────────────────┬─────────────────────────────────────┤
│    PART A: SHARED ENV ENGINE     │       PART B: NATIVE MCP SERVER     │
│                                  │                                     │
│  Tabel: shared_env_vars          │  Client: Claude Desktop / Cursor    │
│  (scope: project | team)         │          │                   │      │
│               │                  │          ▼ (HTTPS SSE)       ▼ (SSH)│
│               ▼                  │    GET /mcp/sse        labuh mcp    │
│  EnvResolver Engine              │    POST /mcp/message    (Stdio)     │
│  Hierarchy: App > Project > Team │          │                   │      │
│               │                  │          └─────────┬─────────┘      │
│               ▼                  │                    ▼                │
│  DeployWorker & Docker           │           internal/mcp/Server       │
│  RunContainerWithVolumes         │        (15 Core Operational Tools)  │
│                                  │                    │                │
│                                  │                    ▼                │
│                                  │        DI Container (Repo/Worker)   │
└──────────────────────────────────┴─────────────────────────────────────┘
```

---

## 4. Part A: Shared Environment Variables Detail

### 4.1 Database Migration (`032_shared_env_vars.sql`)
```sql
CREATE TABLE shared_env_vars (
    id TEXT PRIMARY KEY,
    scope TEXT NOT NULL CHECK(scope IN ('project', 'team')),
    scope_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    is_secret BOOLEAN DEFAULT false,
    is_encrypted BOOLEAN DEFAULT false,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(scope, scope_id, key)
);

CREATE INDEX idx_shared_env_vars_scope ON shared_env_vars(scope, scope_id);
```

### 4.2 Data Models (`internal/models/shared_env_var.go`)
```go
package models

import "time"

type SharedEnvVarScope string

const (
    ScopeProject SharedEnvVarScope = "project"
    ScopeTeam    SharedEnvVarScope = "team"
)

type SharedEnvVar struct {
    ID          string            `json:"id"`
    Scope       SharedEnvVarScope `json:"scope"`
    ScopeID     string            `json:"scope_id"`
    Key         string            `json:"key"`
    Value       string            `json:"value"`
    IsSecret    bool              `json:"is_secret"`
    IsEncrypted bool              `json:"is_encrypted"`
    CreatedAt   time.Time         `json:"created_at"`
    UpdatedAt   time.Time         `json:"updated_at"`
}

type ResolvedEnvVar struct {
    Key      string            `json:"key"`
    Value    string            `json:"value"`
    IsSecret bool              `json:"is_secret"`
    Source   string            `json:"source"` // "team", "project", atau "app"
}
```

### 4.3 Cascading Resolver Engine (`internal/deploy/env_resolver.go`)
Resolusi dilakukan dengan urutan:
1. Load variabel team berdasar `project.TeamID` (jika project terikat ke team). Dimasukkan ke map dengan label `"team"`.
2. Load variabel project berdasar `app.ProjectID`. Nilai ditimpa ke map dengan label `"project"`.
3. Load variabel aplikasi berdasar `app.ID`. Nilai ditimpa ke map dengan label `"app"`.
4. Jika `is_encrypted == true`, nilai didekripsi dengan AES-256 GCM.
5. Menghasilkan:
   - `ResolveForApplication(ctx, app)` -> `[]models.AppEnvVar` untuk container runtime & build.
   - `ResolveDetailed(ctx, app)` -> `[]models.ResolvedEnvVar` untuk UI inspection & MCP tool.

### 4.4 UI di Level Project (`internal/web/pages/projects/`)
Halaman detail project dilengkapi tab **Shared Environment Variables**:
- Bulk .env text area parser/exporter.
- List view dengan toggle mask secret dan delete.
- Handler di `internal/handler/projects.go` untuk `POST /projects/{id}/env`.

---

## 5. Part B: Native Go MCP Server Detail

### 5.1 Package Layout (`internal/mcp/`)
- `server.go`: Inisialisasi instance `server.NewMCPServer("get-labuh", "1.0.0", ...)`, wiring dependencies, dan pendaftaran tools.
- `tools_apps.go`: Handlers untuk lifecycle aplikasi (`list_applications`, `get_application`, `deploy_application`, `get_deployment_status`, `get_application_logs`, `rollback_application`, `restart_application`, `stop_application`).
- `tools_env.go`: Handlers untuk manajemen variabel lingkungan (`get_application_env`, `set_application_env`, `get_shared_env`, `set_shared_env`).
- `tools_db.go`: Handlers untuk database (`list_databases`, `backup_database`).
- `tools_server.go`: Handlers untuk status host & monitoring (`get_server_metrics`, `list_system_alerts`).
- `transport_http.go`: Adapter SSE handler untuk Chi router.
- `transport_stdio.go`: Stdio runner untuk subcommand CLI `labuh mcp`.

### 5.2 Daftar 15 Core Tools

1. `list_applications`:
   - Input: `{}`
   - Output: Daftar aplikasi (id, name, domain, status, git_branch, port, project_id).
2. `get_application`:
   - Input: `{"id": string}`
   - Output: Detail aplikasi lengkap, konfigurasi volume, build mode, last deployment.
3. `deploy_application`:
   - Input: `{"id": string, "force_rebuild"?: bool}`
   - Output: `{"deployment_id": string, "status": "queued"}` (Memicu `deployWorker.Enqueue`).
4. `get_deployment_status`:
   - Input: `{"deployment_id": string}`
   - Output: Status proses deploy, step stepper aktif, logs build.
5. `get_application_logs`:
   - Input: `{"id": string, "tail"?: int}`
   - Output: Cuplikan log container runtime via Docker SDK.
6. `rollback_application`:
   - Input: `{"id": string}`
   - Output: `{"status": "success", "message": "Rolled back to previous image"}`.
7. `restart_application`:
   - Input: `{"id": string}`
   - Output: `{"status": "restarted"}`.
8. `stop_application`:
   - Input: `{"id": string}`
   - Output: `{"status": "stopped"}`.
9. `get_application_env`:
   - Input: `{"id": string, "include_inherited"?: bool}`
   - Output: Array of `{key, value, source, is_secret}`.
10. `set_application_env`:
    - Input: `{"id": string, "key": string, "value": string, "is_secret"?: bool}`
    - Output: `{"status": "saved"}`.
11. `get_shared_env`:
    - Input: `{"scope": "project"|"team", "scope_id": string}`
    - Output: Array of `{id, key, value, is_secret}`.
12. `set_shared_env`:
    - Input: `{"scope": "project"|"team", "scope_id": string, "key": string, "value": string, "is_secret"?: bool}`
    - Output: `{"status": "saved"}`.
13. `list_databases`:
    - Input: `{}`
    - Output: Daftar managed database (id, name, type, status, port, volume).
14. `backup_database`:
    - Input: `{"id": string}`
    - Output: `{"backup_id": string, "status": "completed"}`.
15. `get_server_metrics`:
    - Input: `{}`
    - Output: Metrik procfs host realtime (cpu_percent, ram_used_mb, ram_total_mb, disk_used_gb, disk_total_gb).
16. `list_system_alerts`:
    - Input: `{}`
    - Output: Status terkini host monitor (state: OK/Warning/Critical, resource metrics, last alert).

### 5.3 Transports

#### A. HTTP SSE Transport
- Route:
  - `GET /mcp/sse` -> Membuka koneksi stream SSE ke client.
  - `POST /mcp/message` -> Menerima JSON-RPC payload dari client.
- Middleware: `middleware.RequireAPIKey(apiKeyRepo)`. Client menyertakan header `Authorization: Bearer <api_key>`.

#### B. Stdio CLI Subcommand
- Command: `labuh mcp [--db path]`
- Menjalankan `server.ServeStdio()` membaca `os.Stdin` dan menulis ke `os.Stdout`. Log internal dialihkan ke `os.Stderr` agar tidak mengganggu framing JSON-RPC.

---

## 6. Security & Concurrency Considerations

- **Secrets Encryption**: Semua nilai variabel lingkungan di tabel `shared_env_vars` yang bertanda `is_secret` (atau default) dienkripsi dengan AES-256 GCM sebelum ditulis ke SQLite.
- **Audit Logging**: Perubahan shared env vars dan aksi destruktif via MCP dicatat di tabel `audit_logs`.
- **Concurrency & Race Conditions**:
  - `EnvResolver` bersifat thread-safe dan membaca data via transaksi read-only SQLite.
  - `MCPServer` membungkus pemanggilan background worker secara non-blocking via `deployWorker.Enqueue()`.
- **Zero-CGO & Minimal Dependencies**:
  - Menggunakan library Go murni `github.com/mark3labs/mcp-go`.
  - Tetap 100% kompatibel dengan `CGO_ENABLED=0`.

---

## 7. Testing & Verification Plan

1. **Unit Tests**:
   - `internal/database/repo/shared_env_var_repo_test.go`: CRUD, Unique Constraint, AES-256 GCM encryption.
   - `internal/deploy/env_resolver_test.go`: Cascading precedence (App > Project > Team), masking secret.
   - `internal/mcp/tools_test.go`: Unit test untuk 15 tool MCP dengan in-memory database mock.
2. **Integration Tests**:
   - `internal/mcp/transport_http_test.go`: Pengujian koneksi SSE dan dispatch message dengan otentikasi API Key.
   - `internal/testing/shared_env_and_mcp_e2e_test.go`: End-to-end deployment dengan pewarisan variabel dan trigger deploy via tool MCP `deploy_application`.
3. **Quality Gates**:
   - `go test -race ./...` tanpa data race.
   - `CGO_ENABLED=0 go build ./cmd/labuh`.
