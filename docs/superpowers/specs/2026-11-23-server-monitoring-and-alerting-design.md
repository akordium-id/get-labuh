# Design Specification: Sub-Project 3 (Server Observability, Proactive Resource Alerting, & Extended Notifications)

**Author:** Antigravity (Backend Developer Agent)  
**Date:** 2026-11-23  
**Status:** Approved / In Design  
**Project:** `get-labuh`

---

## 1. Problem Statement & Motivation

Saat ini get-labuh sudah memiliki `CleanupScheduler` untuk membersihkan container dan image Docker yang tidak terpakai secara terjadwal. Namun, get-labuh **belum memiliki sistem pemantauan ambang batas (threshold alert) yang proaktif** terhadap kesehatan host server (VPS) maupun container aktif:
1. Jika kapasitas disk host terisi penuh (> 85% atau > 95%), database SQLite dapat terkunci/rusak dan build image gagal tanpa ada peringatan awal kepada developer.
2. Channel notifikasi get-labuh saat ini terbatas pada Discord Webhook dan Telegram Bot. Banyak tim atau integrasi pihak ketiga (misalnya Slack incoming webhook, PagerDuty, Grafana OnCall, n8n, atau webhook internal) memerlukan pengiriman event notifikasi berupa **Generic HTTP Webhook** terstandarisasi.
3. Notifikasi harus memiliki **Anti-Spam / Cooldown Engine** agar tidak membanjiri channel komunikasi ketika suatu ambang batas terlampaui selama beberapa siklus pemantauan berturut-turut, serta dapat mengirimkan notifikasi pemulihan (*resolved*) saat kapasitas kembali normal.

---

## 2. Goals & Non-Goals

### Goals
1. **Host Resource Monitoring**:
   - Pengambilan metrik host realtime: Kapasitas Disk (Total, Used, Free, Used %), Memori RAM (Total, Used, Free, Used %), dan CPU Load Average (1m, 5m, 15m).
   - Implementasi murni di Go tanpa dependency biner eksternal, memanfaatkan `syscall.Statfs` untuk storage dan `/proc/meminfo` + `/proc/loadavg` untuk memori dan CPU (dengan fallback ramah multi-platform).
2. **Proactive Resource Alert Daemon**:
   - Background ticker independen yang mengevaluasi metrik host terhadap ambang batas yang dikonfigurasi:
     - `host_disk_warning_pct` (default 80%)
     - `host_disk_critical_pct` (default 90%)
     - `host_ram_warning_pct` (default 90%)
   - State machine pendeteksi transisi: `Normal` &rarr; `Warning` &rarr; `Critical` &rarr; `Resolved`.
   - Mekanisme Cooldown (default 30 menit) untuk mencegah alert berulang pada kondisi yang sama, serta pengiriman pesan *Resolved* ketika pemakaian kembali di bawah ambang batas warning.
3. **Generic Webhook Notification Channel**:
   - Menambahkan tipe `webhook` pada `models.NotificationChannel`.
   - Mengirimkan payload JSON terstruktur via HTTP POST dengan dukungan autentikasi opsional (`X-Webhook-Secret` atau `Authorization: Bearer <secret>`).
   - Penambahan flag event `on_server_alert` pada model channel.
4. **UI Settings & Status Indicator**:
   - Halaman `/settings/monitoring` untuk melihat visualisasi status realtime Host Disk & RAM dan konfigurasi ambang batas alert.
   - Pilihan `Generic Webhook` dan checkbox event `Server Health & Resource Alerts` pada form `/settings/notifications`.

### Non-Goals (Opsi B / Masa Depan)
- Pemantauan multi-node terdistribusi (Labuh berfokus pada single-node PaaS).
- SMTP Email Notifier (ditunda setelah Opsi A).
- Container Crash Watcher di luar healthcheck deployment (ditunda ke iterasi berikutnya).

---

## 3. Database Schema & Migration

### Migration `031_observability_alerts_and_webhook_channel.sql`
```sql
-- Tambah kolom untuk channel webhook generik dan event server alert
ALTER TABLE notification_channels ADD COLUMN webhook_url TEXT;
ALTER TABLE notification_channels ADD COLUMN webhook_secret TEXT;
ALTER TABLE notification_channels ADD COLUMN on_server_alert BOOLEAN NOT NULL DEFAULT 1;
```

### Konfigurasi Sistem (`settings` table)
| Key | Default | Deskripsi |
|-----|---------|-----------|
| `host_monitoring_enabled` | `true` | Mengaktifkan/menonaktifkan daemon monitoring |
| `host_disk_warning_pct` | `80` | Persentase disk untuk memicu Warning alert |
| `host_disk_critical_pct` | `90` | Persentase disk untuk memicu Critical alert |
| `host_ram_warning_pct` | `90` | Persentase RAM untuk memicu Warning alert |
| `host_check_interval_seconds` | `60` | Interval polling daemon (detik) |
| `host_alert_cooldown_minutes` | `30` | Cooldown antar alert berulang jika masih di atas batas |

---

## 4. Component Architecture & Data Flow

```
   ┌──────────────────────────────────────────────────────────────┐
   │               HostMonitorDaemon (Background)                 │
   │  (Runs every 60s via time.Ticker, gracefully interruptible)  │
   └───────────────┬──────────────────────────────┬───────────────┘
                   │                              │
                   ▼                              ▼
    [internal/observability/host.go]     [settings.SettingRepo]
    - syscall.Statfs ("/")               - Reads thresholds & cooldown
    - /proc/meminfo                      - Reads enabled flag
    - /proc/loadavg
                   │
                   ▼
    [Alert Evaluator & State Machine]
    - Check Warning/Critical thresholds
    - Check Cooldown Window (lastAlertTime)
    - Detect Normal -> Warning / Critical
    - Detect Warning/Critical -> Normal (Resolved)
                   │
                   ▼ (Triggers NotificationEvent)
    [internal/modules/notifications/notifier.go]
                   │
     ┌─────────────┼────────────────────────────┐
     ▼             ▼                            ▼
 [Discord]    [Telegram]               [Generic Webhook]
  Webhook      Bot API               POST JSON to URL
                                    (Bearer/Secret Auth)
```

---

## 5. Security & Reliability Considerations

1. **Anti-Spam & Cooldown**: Mencegah insiden *notification fatigue* bagi developer. Satu insiden disk penuh hanya memicu 1 alert pertama, lalu masuk cooldown 30 menit sebelum notifikasi pengingat dikirim.
2. **Graceful Fallback**: Pengecekan `/proc/meminfo` dan `/proc/loadavg` tidak boleh menyebabkan panic jika dijalankan di environment non-Linux (misalnya macOS developer environment saat testing lokal).
3. **Non-Blocking Dispatch**: Pengiriman notifikasi ke webhook/Discord/Telegram selalu berjalan secara asinkron (`go n.Dispatch(...)`) dengan timeout context 20 detik, sehingga kegagalan jaringan eksternal tidak mengganggu kestabilan daemon monitoring.
4. **Generic Webhook Security**: Secret disimpan aman dan disematkan via HTTP headers standar. Payload dibatasi JSON valid tanpa data credential sensitif.
