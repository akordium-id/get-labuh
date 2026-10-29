# Module: Domain Models

Package `internal/models` — pure domain structs tanpa external dependencies.

## Aturan Ketat

- **Tidak boleh import package internal lain** (zero deps, hanya stdlib)
- Tidak ada business logic di sini — hanya struct, const, dan type alias
- Semua file di package ini dibaca oleh hampir semua package lain

## File & Isinya

| File | Isi |
|------|-----|
| `application.go` | `Application`, `AppEnvVar`, `Deployment`, enum `SourceType`, `AppStatus`, `DeployStatus` |
| `database.go` | `Database` (PostgreSQL/MySQL/Redis/MongoDB container) |
| `backup.go` | `DatabaseBackup`, `StorageDestination`, `BackupConfig` |
| `notification.go` | `NotificationChannel`, `NotificationEvent` |
| `project.go` | `Project` |
| `team.go` | `Team`, `TeamMember` |
| `user.go` | `User` |
| `session.go` | `Session` |
| `setting.go` | `Setting` |
| `template.go` | `ServiceTemplate` (One-Click Service Catalog) |
| `compose.go` | `ComposeApp` |
| `deploy_key.go` | `DeployKey` |
| `api_key.go` | `APIKey` |
| `audit.go` | `AuditLog` |
| `server.go` | `Server` (remote node — future) |

## Enum Penting

### SourceType (build mode)
```go
SourceTypeGit           = "git"           // auto-detect Dockerfile atau Nixpacks
SourceTypeDockerImage   = "docker_image"  // direct pull, tanpa build
SourceTypeDockerfile    = "dockerfile"    // shallow clone + build context
SourceTypeNixpacks      = "nixpacks"      // zero-config build via nixpacks CLI
SourceTypeDockerCompose = "docker_compose"
SourceTypeDropzone      = "dropzone"      // upload ZIP dari browser
```

### AppStatus (status container aplikasi)
```go
AppStatusIdle     = "idle"
AppStatusBuilding = "building"
AppStatusRunning  = "running"
AppStatusStopped  = "stopped"
AppStatusFailed   = "failed"
```

### DeployStatus (status job deployment)
```go
DeployStatusQueued    = "queued"
DeployStatusCloning   = "cloning"
DeployStatusBuilding  = "building"
DeployStatusDeploying = "deploying"
DeployStatusSuccess   = "success"
DeployStatusFailed    = "failed"
DeployStatusCancelled = "cancelled"
```

## Rules untuk AI Agent

1. **Jangan tambahkan method** ke struct di package ini — tetap plain data
2. Kalau perlu menambah field baru ke `Application` atau `Deployment`, juga update:
   - `repo/application.go` (scan + insert query)
   - `migrations/` (tambah kolom baru via migration file baru)
3. Field pointer (`*string`, `*int64`) artinya nullable di DB — jangan ubah jadi value type sembarangan
4. `Input` struct (misal `CreateApplicationInput`) dipakai oleh handler → repo, bukan untuk membawa data dari DB
