# Module: Notifications

Package `internal/modules/notifications` — manajemen notification channels (Discord Webhook, Telegram Bot API) dan dispatch event alerting.

## Tanggung Jawab

- **`repo.go`**: CRUD channel notifikasi (`notification_channels` table) dengan scope Global atau Project override.
- **`notifier.go`**: Client engine untuk mengirim notifikasi Discord Embed dan pesan Telegram secara non-blocking via goroutine.
- **`handler.go`**: Endpoint UI settings untuk membuat, menghapus, menguji webhook (`/settings/notifications`).
- **`module.go`**: Factory `New(db, projectProvider)` dan `RegisterRoutes(r chi.Router)`.

## Kontrak Eksternal

```go
type ProjectProvider interface {
    GetAll() ([]models.Project, error)
}
```

Modul lain yang memerlukan trigger notifikasi (misal: DeployWorker atau DatabaseBackupRunner) hanya perlu memanggil `notifier.Dispatch(event models.NotificationEvent)`.

## Rules untuk AI Agent

1. Selalu jalankan `Dispatch` secara non-blocking agar tidak menahan job worker deploy atau proses HTTP request.
2. Saat menambahkan channel type baru (misal Slack, Email), tambahkan case baru di `SendToChannel` dalam `notifier.go`.
