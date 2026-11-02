# Design Spec: Git Webhook Auto-Deploy with PR Previews (Ephemeral Environments)

## 1. Overview
Fitur **PR Previews (Ephemeral Environments)** memungkinkan get-labuh untuk secara otomatis membuat, mendeploy, dan menghancurkan (*teardown*) lingkungan preview terisolasi saat Pull Request / Merge Request dibuka atau ditutup di GitHub/GitLab. Setiap PR mendapatkan subdomain unik dan mengkloning environment variables dari aplikasi induk.

---

## 2. Architecture & Data Model

### 2.1 Database Migration (`025_ephemeral_pr_previews.sql`)
```sql
ALTER TABLE applications ADD COLUMN parent_app_id TEXT REFERENCES applications(id) ON DELETE CASCADE;
ALTER TABLE applications ADD COLUMN pr_number INTEGER;
ALTER TABLE applications ADD COLUMN is_preview BOOLEAN DEFAULT FALSE;
ALTER TABLE applications ADD COLUMN preview_template_domain TEXT;
ALTER TABLE applications ADD COLUMN pr_previews_enabled BOOLEAN DEFAULT FALSE;

CREATE INDEX idx_applications_parent_pr ON applications(parent_app_id, pr_number);
```

### 2.2 Go Models (`internal/models/application.go`)
Perluasan struct `Application` dan `CreateApplicationInput`:
- `ParentAppID *string`
- `PRNumber *int`
- `IsPreview bool`
- `PreviewTemplateDomain *string`
- `PRPreviewsEnabled bool`

---

## 3. Webhook Lifecycle Engine (`internal/modules/applications/webhooks.go`)

### 3.1 GitHub Event: `pull_request`
Membaca header `X-GitHub-Event: pull_request`:
- **Action `opened` / `reopened` / `synchronize`**:
  1. Validasi secret HMAC `X-Hub-Signature-256`.
  2. Periksa apakah `app.PRPreviewsEnabled` aktif pada aplikasi induk.
  3. Cek apakah entitas preview untuk `parent_app_id = app.ID` dan `pr_number = payload.Number` sudah ada.
  4. Jika belum ada:
     - Generate slug: `<app.Slug>-pr-<number>`.
     - Hitung domain preview: jika `preview_template_domain` diisi (misal: `pr-{pr}.app.local`), ganti token `{pr}` dan `{slug}`. Fallback ke `<app.Slug>-pr-<number>.preview.local`.
     - Kloning aplikasi dengan `appRepo.CreatePreviewApp(...)`.
     - Kloning env vars dari parent app via `envVarRepo`.
  5. Buat deployment record baru dan masukkan ke antrean `deployWorker.Enqueue(...)` dengan branch PR (`head.ref`) dan commit hash (`head.sha`).
- **Action `closed`**:
  1. Ambil preview app berdasarkan `parent_app_id` dan `pr_number`.
  2. Hentikan container Docker jika sedang running (`dockerClient.StopContainer`, `dockerClient.RemoveContainer`).
  3. Hapus upstream route dari Caddy Admin API (`caddyClient.RemoveRoute(previewDomain)`).
  4. Hapus record aplikasi preview dari database (`appRepo.DeleteWithContext`).

### 3.2 GitLab Event: `Merge Request Hook`
Membaca header `X-Gitlab-Event: Merge Request Hook`:
- Mendukung action: `open`, `reopen`, `update` (deploy preview), dan `close` / `merge` (teardown preview).

---

## 4. User Interface & Dashboard Experience

### 4.1 Tab "PR Previews" di `ApplicationDetailPage`
- Daftar preview aktif untuk aplikasi ini:
  - PR # Number & Title
  - Source branch & commit
  - Status running/building
  - Link langsung ke URL preview
  - Tombol aksi manual: *Redeploy* & *Teardown (Hancurkan Preview)*

### 4.2 Tab "Advanced" di `ApplicationDetailPage`
- Form konfigurasi:
  - Checkbox: "Enable Automatic PR Previews"
  - Input: "Preview Subdomain Template" (e.g. `{app}-pr-{pr}.yourdomain.com`)

---

## 5. Verification & Testing Strategy
- Unit test `TestWebhooks_GitHub_PullRequest_Open` dan `TestWebhooks_GitHub_PullRequest_Closed`.
- Unit test `TestWebhooks_GitLab_MergeRequest_Open_Close`.
- Verifikasi `templ generate`, `go fmt ./...`, dan `go test -count=1 ./...`.
