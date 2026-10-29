# Module: Data Access Layer (Repositories)

Package `internal/database/repo` — semua akses database via raw SQL (SQLite).

## Aturan

- **Tidak ada business logic** — hanya SQL queries dan mapping row → model
- Return `models.*` atau `error` — jangan return raw `*sql.Rows`
- Gunakan `database/sql` standar, bukan ORM
- Prepared statement via `db.QueryContext` dengan `context.Background()` (atau context dari caller)

## Repo Files

| File | Model yang di-handle |
|------|---------------------|
| `application.go` | `Application`, `AppEnvVar` |
| `deployment.go` | `Deployment` |
| `database.go` | `Database` (container) |
| `backup.go` | `DatabaseBackup`, `StorageDestination` |
| `project.go` | `Project`, `Environment` |
| `user.go` | `User` |
| `session.go` | `Session` |
| `notification.go` | `NotificationChannel` |
| `template.go` | `ServiceTemplate` |
| `compose.go` | `ComposeApp` |
| `deploy_key.go` | `DeployKey` |
| `api_key.go` | `APIKey` |
| `setting.go` | `Setting` |
| `audit.go` | `AuditLog` |
| `team.go` | `Team`, `TeamMember` |
| `server.go` | `Server` |
| `env_var.go` | `AppEnvVar` (CRUD terpisah) |

## Pola Umum

```go
type ApplicationRepo struct {
    db *sql.DB
}

func NewApplicationRepo(db *sql.DB) *ApplicationRepo

func (r *ApplicationRepo) GetByID(id string) (*models.Application, error) {
    row := r.db.QueryRowContext(ctx, "SELECT ... FROM applications WHERE id = ?", id)
    return scanApplication(row)
}

func (r *ApplicationRepo) GetAll() ([]models.Application, error) {
    rows, err := r.db.QueryContext(ctx, "SELECT ... FROM applications ORDER BY created_at DESC")
    // ...
}
```

## Konvensi SQL

- ID: UUID v4 string (bukan auto-increment int)
- Timestamps: `created_at`, `updated_at` dalam format RFC3339
- Soft delete: **tidak digunakan** — hard delete langsung
- Nullable kolom: pointer di Go, `NULL` di SQLite

## Migrations

- File migration di `migrations/` (bukan `internal/database/migrations/`)
- Dijalankan via `database.RunAllMigrations(db)` saat startup
- **Jangan edit migration lama** — selalu tambah file baru

## Rules untuk AI Agent

1. Kalau menambah field baru ke model, **wajib** tambah migration SQL + update query di repo yang bersangkutan
2. Test repo menggunakan SQLite in-memory dari `internal/testing/testdb.go`
3. `SettingRepo.Get(key)` bisa return `("", nil)` kalau key tidak ada — handle dengan benar di caller
4. `SettingRepo.InitializeDefaults()` dipanggil saat startup — tambahkan default setting baru di sana kalau perlu
