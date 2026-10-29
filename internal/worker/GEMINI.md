# Module: Deploy Worker

Package `internal/worker` — antrian job deployment berbasis channel Go.

## Tanggung Jawab

- Menerima `models.DeploymentJob` dari handler dan menjalankannya secara async
- Membatasi concurrency via semaphore (default max 3 job bersamaan)
- Mendispatch notifikasi (Discord/Telegram) setelah job selesai/gagal
- Mendukung graceful shutdown (drain antrian sebelum exit)

## Konfigurasi (diset di main.go)

```go
deployWorker := worker.NewDeployWorker(10)   // buffer antrian 10 job
deployWorker.SetMaxConcurrent(3)              // max 3 job berjalan bersamaan
deployWorker.SetWorkerCount(1)               // jumlah goroutine consumer
deployWorker.SetNotifier(notifier, projectRepo)
deployWorker.Start(dockerClient, caddyClient, settingRepo, appRepo, deployRepo)
defer deployWorker.Stop()
```

## Interface yang Diekspos

```go
// Interface yang dipakai oleh handler — jangan import *DeployWorker langsung
type DeployWorkerInterface interface {
    Enqueue(job models.DeploymentJob) error
}
```

## Alur Internal

```
handler.Deploy()
  → deployWorker.Enqueue(job)         // non-blocking, buffer channel
  → goroutine worker picks up job
  → acquire semaphore (max concurrent)
  → deploy.Pipeline.Run(job)          // panggil pipeline Blue-Green
  → release semaphore
  → dispatch notification (goroutine terpisah, non-blocking)
```

## Rules untuk AI Agent

1. Handler harus gunakan **interface** `DeployWorkerInterface`, bukan `*worker.DeployWorker` langsung
2. `Enqueue` harus non-blocking — kalau antrian penuh, return error ke user bukan block
3. Notifikasi di-dispatch via goroutine terpisah — **tidak boleh** block alur utama worker
4. `Stop()` harus wait semua job running selesai (via `sync.WaitGroup`) sebelum return
