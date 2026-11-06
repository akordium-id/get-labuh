# Design Spec: Pre/Post Deployment Commands (Lifecycle Hooks)

## Status: Approved
**Author**: Faiq & Antigravity  
**Date**: 2026-11-06  
**Target Project**: `get-labuh`

---

## 1. Overview & Objective

Fitur **Pre/Post Deployment Commands (Lifecycle Hooks)** menambahkan kemampuan mengeksekusi perintah shell tertentu secara terotomatisasi di dalam container pada tahapan krusial siklus hidup deployment aplikasi (*Blue-Green* zero-downtime):

1. **Pre-deployment Command**:
   - Dieksekusi tepat setelah container kandidat baru (*blue/green*) berhasil di-start oleh Docker Engine, dan **sebelum** *Healthcheck Gate* serta pengalihan rute traffic di Caddy.
   - Contoh kegunaan utama: migrasi database seperti `php artisan migrate --force`, `npx prisma migrate deploy`, atau setup data inisial.
   - **Strict Safety Gate**: Jika command menghasilkan exit code non-zero (gagal) atau timeout:
     - Deployment langsung dibatalkan (status `failed` pada step `pre_deploy`).
     - Container kandidat dihentikan dan dihapus (*rollback* aman).
     - Rute Caddy tidak disentuh dan container lama tetap berjalan melayani traffic produksi tanpa downtime.

2. **Post-deployment Command**:
   - Dieksekusi setelah container kandidat dinyatakan sehat (*healthcheck passed*), rute Caddy berhasil dialihkan ke container baru, dan record status aplikasi di database telah diperbarui.
   - Contoh kegunaan: pembersihan cache aplikasi (`php artisan optimize:clear`, `php artisan queue:restart`), *warmup cache* via request lokal (`curl -f http://localhost:8080/warmup`), atau *post-deploy notification*.
   - **Non-fatal Policy**: Jika post-deployment command gagal, error dicatat secara eksplisit sebagai *Warning* di log deployment, tetapi status deployment tetap dianggap sukses (`done`) karena traffic sudah aktif dan database telah termigrasi.

---

## 2. Architecture & Pipeline Lifecycle

### 2.1 Tahapan Pipeline Deployment

```
[Start Deploy Job]
        │
        ▼
   [Clone / Build]
        │
        ▼
[Run Candidate Container]
        │
        ▼
┌──────────────────────────────────────────────┐
│ Step: "pre_deploy"                           │
│ Cek app.PreDeploymentCommand                 │
│ ├── Kosong -> Lanjut                         │
│ └── Ada -> ExecStream di container kandidat │
│     ├── Error -> Rollback & Abort Deploy     │
│     └── Sukses -> Lanjut                     │
└──────────────────────────────────────────────┘
        │
        ▼
┌──────────────────────────────────────────────┐
│ Step: "healthcheck"                          │
│ WaitForHealthCustom (timeout 30s)            │
│ ├── Gagal -> Rollback & Abort Deploy         │
│ └── Sukses -> Lanjut                         │
└──────────────────────────────────────────────┘
        │
        ▼
[Caddy Route Switch to Candidate Container]
        │
        ▼
[Update DB Application Container ID & Status]
        │
        ▼
┌──────────────────────────────────────────────┐
│ Step: "post_deploy"                          │
│ Cek app.PostDeploymentCommand                │
│ ├── Kosong -> Lanjut                         │
│ └── Ada -> ExecStream di container baru      │
│     ├── Error -> Log Warning (Non-Fatal)     │
│     └── Sukses -> Lanjut                     │
└──────────────────────────────────────────────┘
        │
        ▼
[Stop & Remove Previous Container]
        │
        ▼
[Step: "done" (DeployStatusSuccess)]
```

### 2.2 Execution Specs & Timeout Guards

- **Shell Execution**: Command dieksekusi di dalam container menggunakan shell `sh -c "<command>"` dengan env vars runtime aplikasi yang sudah di-inject oleh Docker.
- **Timeout**: Masing-masing command (pre dan post) memiliki batas waktu maksimal **300 detik (5 menit)** menggunakan `context.WithTimeout`. Jika melebihi 5 menit, proses dihentikan secara paksa (*killed*).
- **Log Streaming**: Output stdout dan stderr dari command dialirkan secara langsung (*streaming*) ke `activeLogWriter` agar dapat dipantau realtime oleh user di UI melalui Server-Sent Events (SSE).

---

## 3. Database Schema & Data Models

### 3.1 Migrasi Database (`migrations/027_deployment_lifecycle_commands.sql`)

```sql
-- Migration 027: Tambahkan kolom lifecycle commands ke tabel applications
ALTER TABLE applications ADD COLUMN pre_deployment_command TEXT;
ALTER TABLE applications ADD COLUMN post_deployment_command TEXT;
```

### 3.2 Domain Models (`internal/models/application.go`)

Tambahkan field pointer opsional pada struct `Application`:
```go
type Application struct {
    // ... field yang sudah ada ...
    PreDeploymentCommand  *string `json:"pre_deployment_command,omitempty"`
    PostDeploymentCommand *string `json:"post_deployment_command,omitempty"`
}

type CreateApplicationInput struct {
    // ... field yang sudah ada ...
    PreDeploymentCommand  *string `json:"pre_deployment_command,omitempty"`
    PostDeploymentCommand *string `json:"post_deployment_command,omitempty"`
}
```

Tambahkan step baru pada `internal/models/deployment.go`:
```go
const (
    DeployStepInit        DeployStep = "init"
    DeployStepClone       DeployStep = "clone"
    DeployStepBuild       DeployStep = "build"
    DeployStepPush        DeployStep = "push"
    DeployStepDeploy      DeployStep = "deploy"
    DeployStepPreDeploy   DeployStep = "pre_deploy"
    DeployStepHealthcheck DeployStep = "healthcheck"
    DeployStepPostDeploy  DeployStep = "post_deploy"
    DeployStepDone        DeployStep = "done"
)
```

---

## 4. Repository & Data Access (`internal/modules/applications/repo.go`)

1. Update query konstanta `applicationColumns` agar mencakup `pre_deployment_command` dan `post_deployment_command`.
2. Update scan `ScanRow` pada method pembacaan (`GetByID`, `ListByEnvironment`, dll.).
3. Tambahkan method pembaruan spesifik:
   ```go
   func (r *Repo) UpdateLifecycleCommands(ctx context.Context, id int64, preCmd, postCmd *string) error
   ```

---

## 5. Deployment Pipeline Engine (`internal/deploy/job.go`)

1. Di dalam `Pipeline.runApplicationDeploy`:
   - Ambil nilai `app.PreDeploymentCommand` dan `app.PostDeploymentCommand` dari `appRepo.GetByID`.
   - Setelah `targetDockerClient.RunContainerWithCustomName` sukses:
     - Jika `preDeploymentCommand != ""` :
       - Panggil `updateStep("pre_deploy", models.DeployStatusDeploying, "")`.
       - Tulis log: `Executing pre-deployment command: <cmd>`.
       - Jalankan command menggunakan Docker SDK `targetDockerClient.ExecStream` (atau helper baru yang mengalirkan log ke `activeLogWriter`) dengan timeout 5 menit.
       - Jika error: tulis log kegagalan, stop & remove `candidateContainerID`, panggil `updateStep("pre_deploy", models.DeployStatusFailed, errMsg)`, dan return `&DeployError{Step: "pre_deploy", Cause: err, Output: errMsg}`.
   - Jalankan `healthcheck` step.
   - Lakukan Caddy route update dan DB record update.
   - Jika `postDeploymentCommand != ""` :
     - Panggil `updateStep("post_deploy", models.DeployStatusDeploying, "")`.
     - Tulis log: `Executing post-deployment command: <cmd>`.
     - Jalankan command dengan timeout 5 menit.
     - Jika error: catat peringatan `[WARNING] Post-deployment command failed: <err>`, namun **jangan batalkan** deployment.
   - Lanjutkan cleanup container lama dan mark status `"done"`.

---

## 6. User Interface & Handler

1. **Templ Component (`internal/web/pages/applications/show.templ` atau tab Advanced di `list.templ`)**:
   - Tambahkan Card Form **Lifecycle Hooks** di tab *Advanced* aplikasi.
   - Input field:
     - `pre_deployment_command` (textarea atau input monospaced, placeholder: `php artisan migrate --force`).
     - `post_deployment_command` (textarea atau input monospaced, placeholder: `php artisan optimize:clear`).
   - Tombol "Save Lifecycle Commands" yang mengirimkan HTTP POST via HTMX ke `/apps/{id}/lifecycle-commands`.
2. **HTTP Handler (`internal/modules/applications/handler.go`)**:
   - Daftarkan route handler `POST /apps/{id}/lifecycle-commands`.
   - Parsing form, sanitasi input (string kosong diubah ke `nil`), simpan ke database, dan kirimkan toast event feedback sukses `HX-Trigger: {"showToast": {"message": "Lifecycle commands updated successfully", "type": "success"}}`.

---

## 7. Testing & Verification Plan

1. **Unit Test Pipeline (`internal/deploy/job_test.go`)**:
   - Test simulasi pre-deployment command sukses → lanjut ke healthcheck → done.
   - Test simulasi pre-deployment command gagal → candidate container di-stop/remove, deployment gagal pada step `pre_deploy`, container lama tetap jalan.
   - Test simulasi post-deployment command gagal → log warning tercatat, status akhir deployment tetap `success`.
2. **Repository Unit Tests (`internal/modules/applications/applications_test.go`)**:
   - Test migrasi database dan method `UpdateLifecycleCommands`.
3. **Handler Unit Tests**:
   - Test validasi form dan response toast.
4. **Full Test Suite Verification**:
   - Jalankan `go test ./...` untuk memastikan tidak ada regresi di modul lain.
