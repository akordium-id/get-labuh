# Module: Application Bootstrap & Lifecycle

Package `internal/app` — entrypoint aplikasi, bootstrap dependensi (IoC Container), config loading, dan routing.

## Tanggung Jawab

- **`config.go`**: Membaca variabel environment (`PORT`, `DATABASE_URL`, `LABUH_BACKUP_DIR`, `LABUH_TLS_*`, dsb.) dengan fallback default.
- **`app.go`**: Mengelola lifecycle server `App` (`New()` dan `Run(ctx)`), setup database, migrations, dan graceful shutdown.
- **`handlers.go`**: Dependency Injection container (`Container` struct) yang menghubungkan database repositori, client infrastruktur (Docker, Caddy), worker background, dan HTTP handlers.
- **`routes.go`**: Konfigurasi router Chi, middleware stack (Security, RateLimit, CSRF, AuditLog, Mode), rute REST API `/api/v1`, dan Web UI routes.
- **`seed.go`**: Inisialisasi seed catalog service bawaan jika database kosong.

## Struktur & Komponen

```go
type App struct {
    cfg       Config
    container *Container
    server    *http.Server
    db        *sql.DB
}

// Inisialisasi:
cfg := app.LoadConfig()
application, err := app.New(cfg)
err := application.Run(ctx)
```

## Rules untuk AI Agent

1. Jika ada endpoint route baru (UI atau API), tambahkan registrasinya di `routes.go`.
2. Jika ada handler atau repo baru, instansiasi dan daftarkan ke dalam `Container` di `handlers.go`.
3. Parameter environment baru harus ditambahkan dan divalidasi di `config.go` (bukan dipanggil via `os.Getenv` sporadis di dalam handler).
4. Entry point `cmd/labuh/main.go` harus dijaga tetap ramping (~30-40 baris).
