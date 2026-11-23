# Database 1-Click Restore & Application Volume Backup / Safe Restore

Dokumentasi ini menjelaskan arsitektur, siklus hidup data, dan mekanisme pemulihan bencana (*disaster recovery*) untuk Database Mandiri dan Persistent Volumes Aplikasi di **get-labuh**.

---

## 1. 1-Click Database Restore

Fitur restore database memungkinkan pemulihan snapshot backup lokal atau S3/R2 ke container database secara instan dengan streaming dump tanpa perlu menyimpan file intermediate di host disk.

### Mode Restore

```
                       [Pilih Snapshot Backup]
                                 │
                   ┌─────────────┴─────────────┐
                   ▼                           ▼
          [1. In-Place Restore]       [2. Restore to New Database]
                   │                           │
                   │                           ├─> Buat record database baru
                   │                           ├─> RunDatabaseContainer di labuh-network
                   │                           ├─> Polling readiness (max 30s)
                   │                           └─> Update connection string
                   ▼                           ▼
       [Buka Stream Gzip Dump (Disk / S3 SigV4 GetObject)]
                                 │
                                 ▼
         [ExecStreamWithStdin ke Target Container CLI]
         - Postgres: psql -v ON_ERROR_STOP=1 -U ... -d ...
         - MySQL   : mysql -u... -p... ...
         - Mongo   : mongorestore --drop --archive ...
         - Redis   : cat > /data/dump.rdb && redis-cli shutdown nosave
                                 │
                                 ▼
                     [Notifikasi Sukses / Gagal]
```

#### A. In-Place Restore
- Mengalirkan dump langsung ke kontainer aktif yang ada.
- Menimpa schema dan data aktif saat ini.
- Dilindungi oleh modal konfirmasi dengan peringatan operasi kritis dan CSRF token.

#### B. Restore to New Database
- Membuat instance database baru yang terpisah tanpa menyentuh container aktif.
- Dihubungkan otomatis ke bridge network `labuh-network` dan persistent volume baru (`labuh-vol-{env}-{slug}`).
- Menjalankan polling status readiness container (menunggu engine siap menerima koneksi, misal PostgreSQL `initdb` selesai).
- Jika eksekusi restore gagal, status ditandai `failed` untuk mencegah orphaned running service yang rusak.

### Pengerasan Keamanan Restore Engine
- **Anti-Command Injection**: Password Redis dilewatkan secara aman melalui environment variable `REDISCLI_AUTH`, bukan interpolasi shell string.
- **Fail-Fast Error Handling**: `psql` dijalankan dengan flag `-v ON_ERROR_STOP=1` agar error SQL saat restore segera menghentikan proses dan tidak dilaporkan sukses palsu.
- **IDOR Protection**: Setiap pemanggilan endpoint memvalidasi bahwa `backupRecord.DatabaseID == currentDB.ID`.

---

## 2. Application Persistent Volume Backup & Safe Restore

Fitur ini menyediakan snapshot terkompresi `.tar.gz` yang mencakup seluruh persistent volumes (Docker Named Volume maupun Bind Mount) dan file mounts (injected config) aplikasi.

### Arsitektur Archive Layout

Untuk menghindari manipulasi filesystem host langsung (`/var/lib/docker/volumes`), get-labuh menggunakan Docker Engine Native Archive API (`CopyFromContainer` dan `CopyToContainer`):

```
Container Filesystem:
  ├── /app/uploads/image.png
  └── /etc/nginx/nginx.conf

Docker CopyFromContainer Stream:
  ├── uploads/image.png
  └── nginx.conf

AppVolumeBackupRunner Tar Re-pathing (filepath.Dir):
  ├── app/uploads/image.png     <-- prefix "app" ditambahkan dari parent directory
  └── etc/nginx/nginx.conf      <-- prefix "etc/nginx" ditambahkan dari parent directory

Hasil Restore ke Root (/):
  Diekstrak tepat ke path semula (/app/uploads/image.png & /etc/nginx/nginx.conf)
```

### Siklus Safe Restore

Safe Restore dirancang agar integritas file terjamin tanpa adanya *file write lock contention* saat aplikasi berjalan:

1. **Graceful Stop**: Container dihentikan sementara (`StopContainer(ctx, id, 10)`).
2. **Guaranteed Restart via Detached Context**: Menggunakan `context.WithTimeout(context.Background(), 45*time.Second)` di dalam blok `defer`. Container dijamin menyala kembali bahkan jika koneksi HTTP klien terputus atau context caller mengalami timeout.
3. **Archive Extraction**: Stream gzip dibuka dan dialirkan langsung via `CopyToContainer(ctx, id, "/", gzReader)`.
4. **Automatic Start**: Container aplikasi kembali hidup dengan data volume yang sudah pulih.

---

## 3. Retention & Otomatisasi S3

- **Scheduler**: Scheduler periodik mengeksekusi cron expression snapshot.
- **Multi-Destination**: Mendukung Local Disk (`/var/lib/labuh/backups`) dan S3-compatible cloud storage (Cloudflare R2, AWS S3, MinIO).
- **Pruning**: Menghapus file lokal dan remote object di S3 ketika jumlah snapshot melebihi `retention_count`.
- **Integrasi Manual Delete**: Penghapusan snapshot melalui dashboard otomatis membersihkan file di server lokal sekaligus remote object di S3 bucket.
