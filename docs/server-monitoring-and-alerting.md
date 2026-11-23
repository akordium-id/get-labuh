# Server Observability, Proactive Resource Alerting, & Generic Webhooks

Dokumentasi ini menjelaskan arsitektur pengumpulan metrik sistem host, background daemon pengawas (*proactive monitoring*), mesin state machine pencegah spam notifikasi (*anti-spam cooldown*), serta integrasi Generic Webhook channel di **get-labuh**.

---

## 1. Arsitektur Pengumpulan Metrik Host (Linux Native & Zero-CGO)

get-labuh memonitor resource server fisik/VPS secara langsung tanpa dependensi eksternal, daemon terpisah (seperti Prometheus Node Exporter), atau ketergantungan CGO. Seluruh metrik dikumpulkan secara native melalui Go standard library dan virtual filesystem Linux.

```
       [Virtual Filesystem / Syscall]
                     │
     ┌───────────────┼───────────────┐
     ▼               ▼               ▼
syscall.Statfs   /proc/meminfo    /proc/loadavg
 (Disk Storage)     (RAM)           (CPU)
     │               │               │
     └───────────────┼───────────────┘
                     ▼
       internal/observability/host.go
              (HostStats struct)
```

### 1.1 Disk Storage (`syscall.Statfs`)
Metrik penyimpanan dihitung menggunakan syscall `Statfs` pada mount point host (default `/`):
- **Total Bytes**: `stat.Blocks * stat.Bsize`
- **Available Bytes**: `stat.Bavail * stat.Bsize` (khusus unprivileged user)
- **Used Bytes**: `Total - (stat.Bfree * stat.Bsize)`
- **Used Percent**: `(Used / Total) * 100.0`

### 1.2 Host RAM (`/proc/meminfo`)
Parsing langsung terhadap `/proc/meminfo` untuk mendapatkan gambaran akurat penggunaan memori sistem operasi:
- Membaca `MemTotal`, `MemAvailable`, `MemFree`, `Buffers`, dan `Cached`.
- Jika `MemAvailable` disediakan oleh kernel Linux (kernel >= 3.14), nilai tersebut diprioritaskan sebagai memori bebas riil.
- Nilai yang dihasilkan murni mencerminkan kapasitas fisik host, tanpa mencampurkan alokasi heap Go internal aplikasi.

### 1.3 CPU Load & Core Count (`/proc/loadavg`)
- Membaca nilai beban rata-rata sistem pada jendela 1 menit, 5 menit, dan 15 menit.
- Mendeteksi jumlah core CPU logis via `runtime.NumCPU()`.

---

## 2. Proactive Alert Daemon & Cooldown State Machine

`HostMonitor` berjalan sebagai background daemon mandiri yang mengevaluasi metrik host setiap 60 detik.

```
                  [Every 60s Ticker]
                          │
                          ▼
             [Collect Realtime HostStats]
                          │
                          ▼
            [Evaluate Thresholds vs State]
                          │
    ┌─────────────────────┼─────────────────────┐
    ▼                     ▼                     ▼
[Normal State]      [Warning State]     [Critical State]
    │                     │                     │
 (Recovered?)       (Crossed Warn /       (Crossed Crit /
    │                De-escalated?)        Escalated?)
    ▼                     ▼                     ▼
Dispatch Resolved    Dispatch Alert        Dispatch Alert
 (Reset Cooldown)   (Update Cooldown)     (Update Cooldown)
                          │                     │
                          └──────────┬──────────┘
                                     ▼
                          [Repeat within Cooldown?]
                                     │
                             ┌───────┴───────┐
                            Yes              No
                             │               │
                          Suppress        Dispatch
                          Warning          Alert
```

### 2.1 Tingkat Keparahan (*Alert Severity*)
- **Normal (`SeverityNormal`)**: Pemakaian resource berada di bawah ambang batas warning.
- **Warning (`SeverityWarning`)**: Pemakaian melewati ambang batas warning namun di bawah batas critical.
- **Critical (`SeverityCritical`)**: Pemakaian melewati ambang batas critical. Memerlukan tindakan segera (misalnya disk penuh).

### 2.2 Aturan Transisi State & De-eskalasi
1. **Eskalasi Cepat (*Immediate Escalation*)**:
   - Jika disk melonjak dari `Normal` langsung ke `Critical`, alert Critical segera ditembakkan tanpa jeda cooldown.
2. **De-eskalasi Bersih (*Clean De-escalation*)**:
   - Jika kapasitas disk yang sebelumnya `Critical` (misal 95%) mulai dibersihkan dan turun ke level `Warning` (misal 82%), sistem mengirimkan notifikasi transisi warning untuk menginformasikan bahwa insiden kritis telah mereda tetapi kapasitas masih perlu dipantau.
3. **Penyelesaian Otomatis (*Resolved Event*)**:
   - Ketika metrik kembali normal di bawah batas warning, sistem secara otomatis mengirimkan 1 notifikasi `Resolved` dan mereset timer cooldown ke nol.
4. **Pencegahan Spam (*Anti-Spam Cooldown*)**:
   - Jika resource tetap tinggi (misal RAM stabil di 92% selama berjam-jam), sistem tidak akan mengirimkan alert setiap menit. Alert pengingat hanya dikirim ulang setelah durasi cooldown (`host_alert_cooldown_minutes`, default 30 menit) terlampaui.

### 2.3 Goroutine Lifecycle & Graceful Shutdown
Daemon mengelola channel shutdown dan goroutine worker menggunakan `sync.WaitGroup`:
- `Start()` bersifat idempoten dan aman dipanggil kembali (*restartable*).
- `Stop()` mematikan ticker, menutup channel stop, dan menunggu goroutine worker menyelesaikan siklus pengecekan yang sedang berlangsung sebelum aplikasi berhenti total (`defer a.container.HostMonitor.Stop()` pada `App.Run`).

---

## 3. Generic Webhook Notification Channel

Selain Discord dan Telegram, **get-labuh** mendukung channel notifikasi `webhook` generik yang dapat diintegrasikan dengan Slack webhook, n8n, Zapier, Opsgenie, custom internal API, atau alerting endpoint lainnya.

### 3.1 Skema Payload JSON (HTTP POST)
Setiap event notifikasi dikirimkan dengan payload JSON terstruktur:

```json
{
  "event": "server_resource_critical",
  "title": "⚠️ Host Storage Alert [CRITICAL]: 92.4% Used",
  "description": "Kapasitas disk host pada path `/` telah mencapai 92.4% (Sisa: 7.6 GB dari total 100.0 GB).",
  "status": "failed",
  "project_id": null,
  "timestamp": "2026-11-23T14:40:00Z",
  "fields": {
    "Storage Path": "/",
    "Disk Used": "92.4%",
    "Disk Free": "7.6 GB",
    "Disk Total": "100.0 GB",
    "Severity": "critical"
  }
}
```

### 3.2 Header & Autentikasi
Pengguna dapat memasukkan secret token pada field `webhook_secret`. Sistem akan menyematkan header:
- `Content-Type: application/json`
- `User-Agent: Labuh-Webhook-Notifier/1.0`
- `X-Webhook-Secret: <secret>`
- `Authorization: Bearer <secret>` (atau langsung nilai token jika diawali `Bearer `)

### 3.3 Konkurensi & Resiliensi Pengiriman
- **Fan-Out Dispatch**: Notifikasi dikirimkan ke seluruh channel aktif secara paralel (*goroutine fan-out*), masing-masing dengan timeout 10 detik. Channel yang lambat atau timeout tidak akan memperlambat atau menggagalkan channel lain.
- **Connection Draining**: Response body dari server penerima webhook selalu dikuras (*drained*) hingga selesai agar koneksi TCP underlying pada pool Keep-Alive dapat digunakan kembali.
- **Error Snippet Logging**: Jika server tujuan mengembalikan status HTTP non-2xx, 1024 byte pertama dari response body dicatat pada log untuk memudahkan penelusuran masalah.

---

## 4. Konfigurasi Settings & Validasi Input

Pengaturan observabilitas server dapat dikelola melalui UI di menu **Settings → Monitoring** (`/settings/monitoring`).

### 4.1 Parameter Konfigurasi

| Parameter | Kunci Setting di Database | Default | Keterangan |
|---|---|---|---|
| Status Monitoring | `host_monitoring_enabled` | `true` | Mengaktifkan/menonaktifkan daemon pengawas |
| Disk Warning Threshold | `host_disk_warning_pct` | `80` | Batas persentase kapasitas disk status warning |
| Disk Critical Threshold | `host_disk_critical_pct` | `90` | Batas persentase kapasitas disk status critical |
| RAM Warning Threshold | `host_ram_warning_pct` | `90` | Batas persentase pemakaian RAM status warning |
| Alert Cooldown | `host_alert_cooldown_minutes` | `30` | Interval jeda minimal antar notifikasi berulang (menit) |

### 4.2 Validasi Server-Side
Sistem menerapkan validasi ketat pada endpoint `POST /settings/monitoring`:
- Nilai persentase harus berada di rentang numerik `1` hingga `100`.
- Ambang batas warning disk wajib lebih kecil dari critical disk (`diskWarn < diskCrit`).
- Durasi cooldown minimal `1` menit dan maksimal `1440` menit (24 jam) untuk mencegah pemborosan resource atau banjir notifikasi.
- Endpoint pembuatan webhook channel (`POST /settings/notifications`) memvalidasi bahwa URL harus diawali dengan skema `http://` atau `https://`.

### 4.3 Manual Test Alert Trigger
Pada halaman `/settings/monitoring`, tombol **"Test Alert Notification"** (`POST /settings/monitoring/test`) disediakan untuk memicu simulasi alert instan ke semua notification channel yang mengaktifkan opsi `on_server_alert` tanpa perlu menunggu kondisi overload server yang sebenarnya.
