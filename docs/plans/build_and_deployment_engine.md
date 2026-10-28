# Arsitektur Build & Deployment Engine (Single-Node)

Dokumen ini mencatat keputusan arsitektur, moda build paritas Coolify, dan lifecycle container yang diimplementasikan pada Labuh Single-Node.

---

## 1. Moda Build (Coolify Parity)

Labuh mendukung 5 moda build & deployment pada single-node host:

| Moda Source | Karakteristik | Alur Eksekusi |
| :--- | :--- | :--- |
| **`docker_image`** | Direct Pull | Pull image tag dari Docker Registry (Docker Hub, GHCR, dsb.) langsung ke host, tanpa tahap kompilasi kode. |
| **`dockerfile`** | Custom Containerfile | Clone git repo shallow (`--depth 1`), bungkus build context menjadi tarball stream, dan kirim ke Docker daemon API (`ImageBuild`). |
| **`nixpacks`** | Zero-Config Build | Eksekusi CLI `nixpacks build <dir> --name <tag>` dengan passing environment variables. Auto-detect bahasa/framework (Node.js, Go, PHP, Python, Rust, Ruby, dll.). |
| **`git`** | Intelligent Auto-detect | Cek apakah ada file `Dockerfile` di root repo: jika ada gunakan moda Dockerfile; jika tidak ada arahkan ke Nixpacks secara transparan. |
| **`docker_compose`** | Multi-Container Stack | Eksekusi `docker compose -p labuh-<id> -f ... up -d --build`. |

---

## 2. Docker Daemon & Networking

- **Socket**: Terhubung ke `/var/run/docker.sock` menggunakan library resmi Moby SDK (`github.com/docker/docker/client`) dengan opsi `client.WithAPIVersionNegotiation()`.
- **Bridge Network**: Setiap container otomatis tersambung ke Docker network internal bernama `labuh-network`. Jika network belum ada, Labuh membuatnya secara otomatis via driver `bridge`.
- **Naming Convention**: Container dinamai mengikuti format `labuh-<envSlug>-<appSlug>-<shortAppID>`.
- **Restart Policy**: Menggunakan `unless-stopped` untuk memastikan keandalan service jika server reboot.

---

## 3. Alur Deployment Pipeline Zero-Downtime (`internal/deploy/job.go`)

```
[ Trigger Deployment ]
          │
          ▼
   [ Create Log File ] (Mendukung streaming SSE ke UI dasbor)
          │
          ▼
   [ Source Resolution & Build ]
   ├── Image   -> Docker Pull
   ├── Git     -> Git Clone -> Check Dockerfile?
   │                               ├─ Yes -> Docker Tar Build
   │                               └─ No  -> Nixpacks Build
   ├── Nixpack -> Git Clone -> Nixpacks Build
   └── Dockerf -> Git Clone -> Docker Tar Build
          │
          ▼
   [ Blue-Green Candidate Container ]
   └── Jalankan candidate container baru berdampingan dengan nama bertimestamp
       di bridge network `labuh-network`
          │
          ▼
   [ Healthcheck Gate (Timeout 30s) ]
   ├── Polling HTTP GET atau TCP ping ke IP internal container pada port aplikasi
   ├── Jika Gagal: Stop & remove candidate container (Rollback protektif, container lama tetap hidup)
   └── Jika Sehat (HTTP 2xx/3xx/4xx atau TCP connect): Lanjut ke routing switch
          │
          ▼
   [ Atomic Caddy Route Switch ]
   └── Re-route upstream reverse proxy Caddy secara atomik dari container lama ke candidate container
          │
          ▼
   [ Cleanup Previous Container ]
   └── Stop dan prune container versi sebelumnya setelah trafik baru berhasil dialihkan (0 Downtime)
          │
          ▼
   [ Mark Finished ] (Status: Success / Failed)
```

---

## 4. One-Click Database Provisioning Engine

Labuh menyediakan provisioning database terisolasi siap pakai dalam bridge network `labuh-network`:

| Engine | Image Default | Direktori Data Persistent Volume | Connection String Format |
| :--- | :--- | :--- | :--- |
| **PostgreSQL** | `postgres:15-alpine` | `/var/lib/postgresql/data` | `postgres://<user>:<pass>@<container_name>:5432/<dbname>?sslmode=disable` |
| **MySQL** | `mysql:8` | `/var/lib/mysql` | `mysql://<user>:<pass>@<container_name>:3306/<dbname>` |
| **Redis** | `redis:7-alpine` | `/data` | `redis://:<pass>@<container_name>:6379` |
| **MongoDB** | `mongo:6` | `/data/db` | `mongodb://<user>:<pass>@<container_name>:27017/<dbname>?authSource=admin` |

### Fitur Otomatisasi Database:
1. **Auto Password & Port**: Jika user tidak mengisi kredensial, Labuh men-generate random secure password dan menetapkan default port standar.
2. **Persistent Named Volume**: Data disimpan pada Docker Named Volume berformat `labuh-vol-<envSlug>-<dbSlug>` sehingga data aman saat container restart / stop.
3. **Internal Hostname**: Container bernama `labuh-db-<envSlug>-<dbSlug>` yang langsung dapat di-resolve oleh aplikasi lain di network `labuh-network` tanpa membuka port ke publik.
4. **Lifecycle Control**: Mendukung Start, Stop, dan Delete (termasuk cleanup container di Docker host).

---

## 5. Real-Time Deployment Log Streaming & Interactive Terminal

Antarmuka deployment Labuh menghadirkan pengalaman live streaming log modern setara Coolify / Railway via Server-Sent Events (SSE) dan Alpine.js:

### Arsitektur Aliran Log:
1. **Endpoint SSE (`/deployments/{id}/logs/stream`)**:
   - Membaca file log output secara inkremental via byte offset cursor tanpa re-read dari awal.
   - Mengirim event terstruktur:
     - `log-message`: baris log baru beserta timestamp.
     - `step-changed`: notifikasi transisi tahap build (`clone` → `build` → `deploy` → `healthcheck` → `done`).
     - `done`: status akhir deployment (`success` / `failed`) dan error trace.
2. **Interactive Terminal UI ([internal/web/pages/deployments/detail.templ](file:///home/orin/code/archive/get-labuh/internal/web/pages/deployments/detail.templ))**:
   - **Live Progress Stepper**: Indikator visual 5 tahap (Clone, Build, Deploy, Healthcheck, Traffic Live) dengan animasi pulsa/spinner saat aktif dan centang hijau saat selesai.
   - **Terminal Console**: Dark mode console dengan syntax highlight otomatis (baris error/merah, sukses/hijau, progress/biru, warning/kuning).
   - **Controls**: Auto-scroll toggle, Copy all logs, Clear log window, dan Reconnect stream.
   - **Graceful Fallback**: Jika deployment sudah selesai saat halaman dibuka, logs otomatis di-fetch dari static endpoint (`/deployments/{id}/logs`).

---

## 6. Persyaratan Host

- **Docker Engine**: Docker CE versi 24+ dengan user Labuh tergabung dalam group `docker`.
- **Git**: Binary `git` terinstall di sistem host.
- **Nixpacks (Opsional untuk zero-config)**:
  ```bash
  curl -sSL https://nixpacks.com/install.sh | bash
  ```
