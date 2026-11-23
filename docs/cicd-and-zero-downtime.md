# CI/CD Pipeline, True Zero-Downtime Deployments, & Private Docker Registry

Dokumentasi ini menjelaskan arsitektur deployment pipeline, mekanisme zero-downtime berbasis Caddy & healthcheck, integrasi webhook CI/CD eksternal, pelaporan commit status, serta autentikasi Private Docker Registry di **get-labuh**.

---

## 1. True Zero-Downtime Deployment & Connection Draining

Deployment pipeline di get-labuh mengadopsi model Blue-Green single-node yang menjamin nol downtime (*zero-downtime*) dan proteksi rollback otomatis jika aplikasi mengalami crash saat booting.

### Alur Eksekusi Pipeline

```
           [Deploy Trigger: Webhook / UI / Git Push]
                             │
                             ▼
                 [Pull Image / Build Source]
      (Menggunakan X-Registry-Auth jika private registry)
                             │
                             ▼
         [Run Candidate Container (Container Baru)]
                             │
                             ▼
              [Healthcheck Gate & Fail-Fast]
        - Native Docker HEALTHCHECK status == "healthy"?
        - HTTP Probe (Status < 500) / TCP Dial Probe?
        - Cek Crashloop/Exit (Status restarting/exited)?
             │                               │
       (Gagal / Crash)                   (Healthy)
             │                               │
             ▼                               ▼
    [Rollback Otomatis]         [Atomic Route Switch di Caddy]
    - Hapus kandidat             (Traffic publik beralih instan)
    - Container lama tetap live              │
    - Status: "failure"                      ▼
                                 [Graceful Connection Draining]
                                 - Window 5 detik (interruptible)
                                 - In-flight request lama selesai
                                             │
                                             ▼
                                 [Stop & Cleanup Container Lama]
                                 - Docker Stop (grace 15s)
                                 - Remove Container lama
                                 - Status: "success"
```

### Komponen Kunci Zero-Downtime
1. **Eliminasi Premature Bypass**: Tidak ada lagi asumsi 3 detik langsung healthy. Container wajib benar-benar lulus uji Docker native healthcheck atau HTTP response status `< 500`.
2. **Fail-Fast Crash Detection**: Jika container kandidat mengalami crash saat startup (`State.Restarting == true`, `State.Status == "restarting"`, atau `State.Status == "exited"`), pipeline langsung gagal dalam 1 detik tanpa membuang waktu menunggu batas timeout (30s). Container lama tidak pernah dimatikan.
3. **Atomic Switch & Graceful Draining**: Pembaruan reverse proxy Caddy dilakukan terlebih dahulu. Setelah traffic baru diarahkan ke container kandidat, container lama diberi jeda *draining* 5 detik untuk menyelesaikan request yang sedang berjalan (*in-flight*), dilanjutkan dengan penghentian anggun Docker 15 detik (*SIGTERM* sebelum *SIGKILL*).

---

## 2. Universal CI/CD Webhooks & Dynamic Image Updates

Aplikasi di get-labuh menyediakan endpoint webhook universal yang kompatibel dengan berbagai runner CI/CD (GitHub Actions, GitLab CI, Woodpecker, Jenkins, dsb).

### Endpoint Webhook
```
POST /webhooks/deploy/{application_id}
POST /webhooks/github/{application_id}
POST /webhooks/gitlab/{application_id}
```

### Autentikasi
Kirimkan Webhook Secret melalui salah satu metode:
- HTTP Header: `Authorization: Bearer <webhook_secret>`
- Query Param: `?token=<webhook_secret>`
- HMAC Header: `X-Hub-Signature-256: sha256=<signature>`

### Format Payload Dinamis
Endpoint webhook menerima JSON fleksibel:
```json
{
  "image_tag": "ghcr.io/org/repo:v2.5.0",
  "commit": "a1b2c3d4e5f67890abcdef",
  "commit_message": "feat: release v2.5.0 [skip ci]",
  "branch": "main"
}
```

- **Dynamic Image Tag**: Untuk aplikasi dengan source type `docker_image`, jika field `image_tag` atau `image` dikirim, get-labuh otomatis memperbarui field image aplikasi di database dan men-deploy tag baru tersebut secara instan.
- **Commit Metadata**: Field `commit`, `commit_hash`, atau `sha` dicatat langsung ke history deployment record dan diteruskan ke pipeline.

### Contoh GitHub Actions Workflow
```yaml
name: Build and Deploy to Labuh

on:
  push:
    tags:
      - 'v*'

jobs:
  build-and-deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and Push
        uses: docker/build-push-action@v5
        with:
          push: true
          tags: ghcr.io/${{ github.repository }}:${{ github.ref_name }}

      - name: Trigger Labuh Webhook
        run: |
          curl -X POST "https://labuh.domain.com/webhooks/deploy/${{ secrets.LABUH_APP_ID }}" \
            -H "Authorization: Bearer ${{ secrets.LABUH_WEBHOOK_SECRET }}" \
            -H "Content-Type: application/json" \
            -d '{
              "image_tag": "ghcr.io/${{ github.repository }}:${{ github.ref_name }}",
              "commit": "${{ github.sha }}",
              "commit_message": "${{ github.event.head_commit.message }}"
            }'
```

---

## 3. GitHub & GitLab Commit Status Feedback Loop

get-labuh secara otomatis melaporkan status eksekusi deployment langsung ke commit SHA pada repositori GitHub atau GitLab:

```
[Start Deploy]  ──> Report Commit Status: "pending" ("Deploying in Labuh...")
[Deploy Error]  ──> Report Commit Status: "failure" / "failed" ("Deployment failed: ...")
[Deploy Success]──> Report Commit Status: "success" ("Deployed successfully without downtime")
```

### Resolusi Kredensial Token
1. **Per-Application Token**: Diambil dari konfigurasi `git_token` (Personal Access Token) di tab Settings aplikasi.
2. **Global Fallback**: Jika tidak disetel per aplikasi, menggunakan token bot global dari `Settings -> Git Settings` (`github_bot_token` atau `gitlab_bot_token`).

Status disematkan dengan context identifier `labuh/deploy` dan tautan langsung ke detail log deployment di dashboard get-labuh.

---

## 4. Private Docker Registry Authentication

get-labuh mendukung pull image dari registry privat manapun (GitHub Container Registry / GHCR, Docker Hub Private, AWS ECR, GitLab Registry, Harbor, dsb).

### Keamanan Kredensial
- Password dan Access Token registry disimpan di database SQLite menggunakan enkripsi terautentikasi **AES-256-GCM** dengan envelope prefix `ENC:`.
- Password disembunyikan dari serialisasi JSON (`json:"-"`).

### Penggunaan
1. **Kelola Registry**:
   - Buka menu **Settings** &rarr; **Docker Registries** (`/settings/registries`).
   - Masukkan nama, server address (contoh: `ghcr.io` atau `index.docker.io/v1/`), username, dan personal access token / password.
   - Klik **Uji Koneksi** untuk memvalidasi kredensial ke Docker daemon secara langsung.
2. **Tautkan ke Aplikasi**:
   - Pada halaman detail aplikasi tipe `docker_image`, buka card **Docker Image & Private Registry**.
   - Pilih registry privat yang telah didaftarkan.
   - Saat deployment berjalan, get-labuh menginjeksi kredensial ke header `X-Registry-Auth` pada panggilan Docker Engine SDK `ImagePull`.
