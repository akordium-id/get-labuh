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

## 4. Persyaratan Host

- **Docker Engine**: Docker CE versi 24+ dengan user Labuh tergabung dalam group `docker`.
- **Git**: Binary `git` terinstall di sistem host.
- **Nixpacks (Opsional untuk zero-config)**:
  ```bash
  curl -sSL https://nixpacks.com/install.sh | bash
  ```
