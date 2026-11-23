# Design Spec: CI/CD Pipeline, True Zero-Downtime Deployments, & Private Docker Registry

## 1. Context & Objectives

Dalam arsitektur deployment platform Labuh saat ini:
1. **Zero-Downtime Flaw**: Pada `internal/docker/container.go:368-375`, fungsi `WaitForHealthCustom` memiliki bypass hardcoded 3 detik:
   ```go
   if insp, inspErr := c.InspectContainer(ctx, containerID); ... {
       if time.Since(startedAt) >= 3*time.Second {
           return nil
       }
   }
   ```
   Aplikasi modern (Next.js, Rails, Laravel, Go dengan koneksi database/migrasi) seringkali membutuhkan 5–15 detik untuk siap mendengarkan port HTTP. Akibat bypass 3 detik ini, healthcheck mengembalikan `nil` secara prematur, Caddy langsung mengalihkan traffic, dan pengguna mengalami `502 Bad Gateway` saat container belum siap melayani traffic.
2. **Missing Connection Draining**: Ketika Caddy memperbarui route ke container kandidat baru, container lama langsung dieksekusi `StopContainer` dan `RemoveContainer`. Request in-flight yang sedang diproses oleh container lama terputus seketika (connection reset/TCP abort).
3. **CI/CD Webhook Limitations**: Webhook parser `genericWebhookPayload` pada `internal/modules/applications/webhooks.go` hanya mengenali `commit_hash`. Pipeline eksternal (GitHub Actions, GitLab CI, curl) umumnya mengirim payload `{ "commit": "...", "branch": "...", "image_tag": "..." }`. Selain itu, untuk aplikasi bertipe `docker_image`, belum ada mekanisme webhook untuk memperbarui image tag secara dinamis saat image baru di-push oleh CI runner.
4. **No Commit Status Feedback**: Labuh telah memiliki `internal/gitbot/client.go` untuk PR comment preview, namun belum mengintegrasikan Commit Status API (`api.github.com/repos/{owner}/{repo}/statuses/{sha}`) sehingga status CI/CD di GitHub Actions / PR checks tidak mencerminkan status deployment Labuh (`pending`, `success`, `failure`).
5. **Private Docker Registry Authentication**: Labuh saat ini hanya mendukung pull image publik. Pengguna belum dapat menarik image dari repositori private seperti GitHub Container Registry (`ghcr.io`), Docker Hub Private, AWS ECR, GitLab Registry, atau private Harbor registry.

### Tujuan Utama:
1. **True Zero-Downtime Deployments**:
   - Menghapus total bypass 3 detik pada `WaitForHealthCustom`.
   - Menambahkan deteksi dini kegagalan container (fail fast jika container keluar/crash sebelum timeout).
   - Menerapkan **Graceful Connection Draining** (delay terukur 5 detik setelah cutover Caddy sebelum container lama dihentikan dengan grace period 15 detik).
2. **CI/CD Webhook & GitHub Commit Status**:
   - Parser webhook yang fleksibel menerima format `{ "commit": "...", "commit_hash": "...", "branch": "...", "image_tag": "..." }`.
   - Update otomatis image tag untuk `SourceTypeDockerImage`.
   - Feedback loop GitHub Commit Status (`pending` saat antrean/deploy dimulai, `success` saat traffic live, `failure` jika build/healthcheck gagal).
3. **Private Docker Registry Integration**:
   - Skema database dan enkripsi AES-256-GCM untuk kredensial registry.
   - Dukungan autentikasi Docker SDK via base64 encoded `X-Registry-Auth` header.
   - UI manajemen Docker Registries di Settings dan seleksi registry pada konfigurasi aplikasi.

---

## 2. Architecture & Components

```
+---------------------------------------------------------------------------------------+
|                                    CI/CD PIPELINE                                     |
|  (GitHub Actions / GitLab CI / Git Push / Webhook POST /applications/{id}/deploy)    |
+---------------------------------------------------------------------------------------+
                                           |
                                           v
+---------------------------------------------------------------------------------------+
|                             WebhooksHandler (applications)                            |
|  - Parse payload: commit / commit_hash, branch, image_tag                             |
|  - Validasi secret HMAC / token                                                       |
|  - Enqueue deployment job                                                             |
+---------------------------------------------------------------------------------------+
                                           |
                                           v
+---------------------------------------------------------------------------------------+
|                               Pipeline (internal/deploy)                              |
|  1. Init: Report Commit Status -> "pending" (labuh/deploy)                            |
|  2. Pull/Build: Inject Docker Registry Auth (ghcr.io, ECR, Docker Hub) jika private   |
|  3. Run Candidate Container: Launch paralel tanpa mengganggu container aktif          |
|  4. Pre-Deploy Hook: Lifecycle validation                                             |
|  5. True Healthcheck Gate: HTTP/TCP probing aktif hingga siap (tanpa bypass 3s)       |
|  6. Traffic Cutover: Caddy Route Update secara atomik                                |
|  7. Post-Deploy Hook: Post-migration / cache warm-up                                  |
|  8. Graceful Connection Draining: 5s pause agar in-flight requests selesai            |
|  9. Cleanup: Stop & remove old container (15s Docker stop grace period)               |
| 10. Done: Report Commit Status -> "success" (labuh/deploy)                            |
+---------------------------------------------------------------------------------------+
        |                                  |                                |
        v                                  v                                v
+------------------+             +-------------------+            +---------------------+
|   Caddy Proxy    |             |  Docker Engine    |            |   GitBot Client     |
| (Atomic Cutover) |             |  (Pull with Auth, |            | (Commit Status API  |
|                  |             |   Drain, Health)  |            |  GitHub & GitLab)   |
+------------------+             +-------------------+            +---------------------+
```

### Rincian Komponen:

1. **`internal/docker/container.go`**:
   - `WaitForHealthCustom(ctx context.Context, containerID string, port int, path string, timeout time.Duration) error`:
     - Cek apakah container mati/crash di tengah jalan (`insp.State.Running == false`) -> fail fast tanpa menunggu timeout.
     - Evaluasi native Docker Healthcheck jika image mendefinisikannya (`insp.State.Health.Status == "healthy"`).
     - Probing HTTP `http://<ip>:<port><path>` dengan timeout 2s per request dan interval polling 1s. Menerima status code `< 500`.
     - Fallback ke TCP probe jika path kosong atau protocol non-HTTP.
     - Menghapus kode `time.Since(startedAt) >= 3*time.Second`.

2. **`internal/deploy/job.go`**:
   - **Connection Draining**: Setelah Caddy cutover, berikan window draining selama 5 detik (`time.Sleep(5 * time.Second)`) sebelum memanggil `StopContainer(ctx, oldContainerID, 15)`.
   - **Commit Status Integration**: Panggil `gitbotClient.SetGitHubCommitStatus` pada awal deploy (`pending`), saat sukses (`success`), dan saat error (`failure`).
   - **Registry Auth Injection**: Sebelum `PullImage`, periksa apakah `app.DockerRegistryID` terpasang. Ambil kredensial, dekripsi password via `encryption.Decrypt`, dan panggil `PullImageWithAuth`.

3. **`internal/docker/client.go`**:
   - `type RegistryAuth struct { Username, Password, ServerAddress, Email string }`
   - `PullImageWithAuth(ctx context.Context, imageRef string, auth *RegistryAuth) error`:
     - Serialisasi `RegistryAuth` ke JSON, lalu base64 URL-encode ke `image.PullOptions{RegistryAuth: encodedAuth}`.
   - `VerifyRegistryAuth(ctx context.Context, auth *RegistryAuth) error`:
     - Menjalankan `c.cli.RegistryLogin(ctx, registry.AuthConfig{...})` untuk memvalidasi kredensial saat pengguna menekan "Test Connection".

4. **`internal/gitbot/client.go`**:
   - `SetGitHubCommitStatus(ctx context.Context, repoURL, commitSHA, token, state, targetURL, description, contextName string) error`:
     - Endpoint: `POST https://api.github.com/repos/{owner}/{repo}/statuses/{sha}`
     - Request body:
       ```json
       {
         "state": "pending|success|failure|error",
         "target_url": "https://labuh.domain.com/applications/xxx",
         "description": "Deployment is live and healthy",
         "context": "labuh/deploy"
       }
       ```

5. **`internal/modules/applications/webhooks.go`**:
   - Tambahkan pembacaan field `commit`, `commit_hash`, `branch`, dan `image_tag` pada generic JSON parser.
   - Jika `image_tag` dikirim dan `app.SourceType == SourceTypeDockerImage`, update field `DockerImage` pada aplikasi dan deploy job.

6. **`internal/modules/settings/`**:
   - `RegistryRepo`: CRUD operasi untuk tabel `docker_registries`.
   - Handlers untuk halaman `/settings/registries` (List, Create, Delete, Test).

---

## 3. Database Schema & Domain Models

### Migration: `internal/database/migrations/030_docker_registries.sql`

```sql
CREATE TABLE IF NOT EXISTS docker_registries (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    server_address TEXT NOT NULL,
    username TEXT NOT NULL,
    password TEXT NOT NULL,
    email TEXT DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_docker_registries_server_address ON docker_registries(server_address);

ALTER TABLE applications ADD COLUMN docker_registry_id TEXT REFERENCES docker_registries(id) ON DELETE SET NULL;
```

### Domain Models (`internal/models/registry.go`)

```go
package models

import "time"

type DockerRegistry struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	ServerAddress string    `json:"server_address"`
	Username      string    `json:"username"`
	Password      string    `json:"-"` // Terenkripsi di DB, disembunyikan dari JSON
	Email         string    `json:"email"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CreateDockerRegistryInput struct {
	Name          string `json:"name"`
	ServerAddress string `json:"server_address"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	Email         string `json:"email"`
}

type UpdateDockerRegistryInput struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	ServerAddress string  `json:"server_address"`
	Username      string  `json:"username"`
	Password      *string `json:"password,omitempty"`
	Email         string  `json:"email"`
}
```

### Update `internal/models/application.go`

Tambahkan field `DockerRegistryID *string` pada:
- `Application`
- `CreateApplicationInput`
- `DeploymentJob`

---

## 4. Flow & Detailed Logic

### Flow 1: True Zero-Downtime Deployment & Connection Draining

```
Pipeline.Execute(job):
  1. targetDockerClient.RunContainerWithVolumes(...) -> candidateContainerID
  2. Lifecycle Hook Pre-Deploy (jika ada)
  3. WaitForHealthCustom(ctx, candidateContainerID, port, path, timeout):
     - Loop polling setiap 1s:
       a. InspectContainer: jika !State.Running -> fail fast (container crashed)
       b. Jika State.Health != nil && State.Health.Status == "healthy" -> return nil
       c. HTTP GET http://<ip>:<port><path> -> jika resp.StatusCode < 500 -> return nil
       d. Jika path == "" -> TCP dial test -> return nil
       e. Jika ctx.Done() atau timeout lewat -> return ErrHealthcheckTimeout
  4. Caddy.AddRouteWithOptions(domain, candidateName, port, opts) -> Traffic Live
  5. DB UpdateContainerAndStatus(appID, candidateContainerID, "running")
  6. Lifecycle Hook Post-Deploy (jika ada)
  7. Connection Draining:
     if oldContainerID != "" && oldContainerID != candidateContainerID:
       writeLog("Draining in-flight connections on previous container for 5s...")
       time.Sleep(5 * time.Second)
       targetDockerClient.StopContainer(ctx, oldContainerID, 15) // 15s Docker stop grace
       targetDockerClient.RemoveContainer(ctx, oldContainerID)
```

### Flow 2: CI/CD Webhook & Dynamic Image Tag

```
POST /applications/{id}/deploy:
  Body JSON:
  {
    "branch": "main",
    "commit": "a1b2c3d4e5",
    "image_tag": "ghcr.io/org/repo:sha-a1b2c3d"
  }

  - Unmarshal ke struct yang menerima `commit` maupun `commit_hash`.
  - Jika `app.SourceType == "docker_image"` dan `image_tag != ""`:
      appRepo.UpdateDockerImage(app.ID, image_tag)
      job.DockerImage = image_tag
  - Enqueue deployment dengan `CommitHash = commit`, `Branch = branch`.
```

### Flow 3: GitHub Commit Status Feedback Loop

```
DeployWorker / Pipeline:
  - Cari token GitHub: periksa app.GitToken, jika kosong fallback ke setting global "github_bot_token".
  - Jika token ada && job.CommitHash != "":
      a. Saat mulai deploy:
         SetGitHubCommitStatus(ctx, repoURL, job.CommitHash, token, "pending", appURL, "Deployment in progress...", "labuh/deploy")
      b. Jika deploy gagal:
         SetGitHubCommitStatus(ctx, repoURL, job.CommitHash, token, "failure", appURL, "Deployment failed: " + errMsg, "labuh/deploy")
      c. Jika deploy sukses:
         SetGitHubCommitStatus(ctx, repoURL, job.CommitHash, token, "success", appURL, "Deployment is live and healthy", "labuh/deploy")
```

### Flow 4: Private Docker Registry Authentication

```
Pull Image Flow:
  - Jika app.DockerRegistryID != nil:
      registry, err := registryRepo.GetByID(ctx, *app.DockerRegistryID)
      decryptedPassword, _ := encryption.Decrypt(registry.Password)
      auth := &RegistryAuth{
          Username: registry.Username,
          Password: decryptedPassword,
          ServerAddress: registry.ServerAddress,
          Email: registry.Email,
      }
      err := dockerClient.PullImageWithAuth(ctx, imageName, auth)
  - Else:
      err := dockerClient.PullImage(ctx, imageName)
```

---

## 5. Security & Edge Cases

1. **Secret Encryption**: Seluruh password/token private registry dienkripsi menggunakan AES-256-GCM via `internal/encryption`. Kolom `password` di DB diawali prefix `ENC:`.
2. **Leak Prevention**: Kredensial password registry tidak pernah ditampilkan di Templ view maupun JSON API (`json:"-"`).
3. **Flapping Container Detection**: Jika container crash berulang kali saat start (exit code 1, 137, 139), `WaitForHealthCustom` langsung mendeteksi `!insp.State.Running` pada detik pertama dan melakukan rollback tanpa menunggu 30 detik.
4. **Zero-Downtime Rollback**: Jika container kandidat gagal healthcheck atau pre-deploy hook, container lama TIDAK pernah disentuh, traffic Caddy tetap mengalir ke container lama, dan container kandidat dibersihkan secara aman.
5. **Draining Timeout Safety**: Context deployment membawa deadline yang cukup untuk mengakomodasi 5s connection draining dan 15s graceful container stop tanpa terpotong timeout.

---

## 6. Testing & Verification

1. **Unit Tests**:
   - `WaitForHealthCustom`: Verifikasi container crash langsung return error; container HTTP 200 return healthy; timeout return error.
   - `WebhooksHandler`: Verifikasi parsing `commit` vs `commit_hash`, `branch` matching, dan update `image_tag`.
   - `GitBot Client`: Verifikasi pembuatan HTTP request status ke GitHub API dengan mock HTTP server.
   - `RegistryRepo`: Test CRUD & otomatis enkripsi/dekripsi password SQLite in-memory.
2. **Integration Tests**:
   - Skenario deployment Blue-Green dengan container sleep & port listener: memastikan traffic tidak switch sebelum port aktif.
   - Skenario pull image dengan mock auth header base64.
3. **Regression Test**:
   - Jalankan `go test ./...` untuk memastikan seluruh fitur yang ada (Database Backup, Volume Backup, Ephemeral PR Preview, Caddy, dsb.) tetap 100% lulus tanpa regresi.
