# Design Spec: Git PR/MR Preview Comment Bot (GitHub & GitLab)

## Status: Approved
**Author**: Faiq & Antigravity  
**Date**: 2026-11-02  
**Target Project**: `get-labuh`

---

## 1. Overview & Objective
Enhancement untuk fitur *Git Webhook Auto-Deploy with PR Previews (Ephemeral Environments)* pada get-labuh. Fitur ini menambahkan bot komentar otomatis pada Pull Request GitHub dan Merge Request GitLab untuk mengabarkan status preview environment secara real-time:
1. Saat PR/MR dibuka/diperbarui: komentar dibuat/di-update dengan status *Deploying...*.
2. Saat deployment selesai (healthcheck pass): komentar diperbarui dengan link URL preview yang live.
3. Saat deployment gagal: komentar diperbarui dengan informasi error.
4. Saat PR/MR ditutup: komentar diperbarui menjadi *Destroyed* (teardown).

Integrasi dirancang dengan pendekatan **Anti-Spam Single-Comment Update** menggunakan marker `<!-- labuh-pr-preview -->` agar tidak membanjiri timeline PR/MR.

---

## 2. Authentication & Token Resolution
Menggunakan model **Kombinasi (Global Default + Override per Aplikasi)**:
- **Application Level**: Kolom `git_token` (dienkripsi menggunakan `encryption.Encrypt/Decrypt`) di tabel `applications`.
- **Global Settings Level**: Kunci `github_bot_token` dan `gitlab_bot_token` di tabel `settings`.
- **Resolution Strategy**:
  1. Jika aplikasi memiliki `git_token` (setelah didekripsi), gunakan token tersebut.
  2. Jika kosong atau null, cek tipe repo (`github.com` vs `gitlab.com` atau provider custom). Ambil `github_bot_token` atau `gitlab_bot_token` dari `settings`.
  3. Jika tidak ada token yang tersedia, lewati proses notifikasi secara *silent* (tidak menghentikan proses deploy).

---

## 3. Provider Integrations

### 3.1 GitHub REST API
- **Endpoint List Comments**: `GET https://api.github.com/repos/{owner}/{repo}/issues/{pull_number}/comments`
- **Endpoint Create Comment**: `POST https://api.github.com/repos/{owner}/{repo}/issues/{pull_number}/comments`
- **Endpoint Update Comment**: `PATCH https://api.github.com/repos/{owner}/{repo}/issues/comments/{comment_id}`
- **Authentication**: `Authorization: Bearer <token>`
- **Headers**: `Accept: application/vnd.github+json`

### 3.2 GitLab REST API
- **Endpoint List Notes**: `GET https://gitlab.com/api/v4/projects/{project_id_or_url_encoded_path}/merge_requests/{mr_iid}/notes`
- **Endpoint Create Note**: `POST https://gitlab.com/api/v4/projects/{project_id_or_url_encoded_path}/merge_requests/{mr_iid}/notes`
- **Endpoint Update Note**: `PUT https://gitlab.com/api/v4/projects/{project_id_or_url_encoded_path}/merge_requests/{mr_iid}/notes/{note_id}`
- **Authentication**: `PRIVATE-TOKEN: <token>`

---

## 4. Lifecycle & Comment Templates

Setiap komentar wajib memuat hidden HTML marker: `<!-- labuh-pr-preview -->`.

### Template: Building / In Progress
```markdown
<!-- labuh-pr-preview -->
### ⛵ Labuh Preview Deployment
Deployment environment untuk PR ini sedang diproses...
- **Commit**: `{{commit_hash}}`
- **Branch**: `{{branch}}`
- **Status**: 🟡 *Building & Deploying...*
```

### Template: Live & Healthy
```markdown
<!-- labuh-pr-preview -->
### ⛵ Labuh Preview Deployment
Preview environment telah siap diakses!
- **Preview URL**: [{{preview_url}}]({{preview_url}})
- **Commit**: `{{commit_hash}}`
- **Status**: 🟢 *Live & Healthy*
- **Updated At**: `{{timestamp}}`
```

### Template: Failed
```markdown
<!-- labuh-pr-preview -->
### ⛵ Labuh Preview Deployment
Deployment preview environment mengalami kegagalan.
- **Commit**: `{{commit_hash}}`
- **Status**: 🔴 *Deployment Failed*
- **Error**: `{{error_message}}`
```

### Template: Teardown / Closed
```markdown
<!-- labuh-pr-preview -->
### ⛵ Labuh Preview Deployment
Pull Request telah ditutup. Preview environment telah dinonaktifkan (teardown).
- **Status**: ⚪ *Destroyed*
```

---

## 5. Architectural Changes

### 5.1 Database Migration
- File: `internal/database/migrations/026_git_bot_tokens.sql`
  - `ALTER TABLE applications ADD COLUMN git_token TEXT;`
  - Inisialisasi default settings keys jika belum ada: `github_bot_token`, `gitlab_bot_token`.

### 5.2 GitBot Service
- Package: `internal/gitbot`
  - Helper untuk parse repository URL (`owner/repo` untuk GitHub, `path/to/project` untuk GitLab).
  - Client dengan method:
    - `PostOrUpdateGitHubComment(ctx, repoURL, prNumber, token, markdownBody)`
    - `PostOrUpdateGitLabNote(ctx, repoURL, mrIID, token, markdownBody)`

### 5.3 Webhook Trigger & Deploy Worker Wiring
- Di `internal/modules/applications/webhooks.go`:
  - Saat webhook PR `opened`/`synchronize`: panggil GitBot dengan status *Building...*.
  - Saat webhook PR `closed`: panggil GitBot dengan status *Destroyed*.
- Di `internal/worker/deploy_worker.go`:
  - Jika `app.IsPreview`: ambil parent app, resolve token, panggil GitBot dengan status *Live* (sukses) atau *Failed* (gagal).

### 5.4 UI Form
- Di tab `Advanced` aplikasi: input field *Git Access Token (Override)*.
- Di halaman `/settings`: input field *Global GitHub Bot Token* & *Global GitLab Bot Token*.

---

## 6. Testing Strategy
- Unit test mock HTTP server untuk GitHub REST API (cek matching `<!-- labuh-pr-preview -->`, verifikasi `POST` saat baru dan `PATCH` saat update).
- Unit test mock HTTP server untuk GitLab REST API.
- End-to-end integration test pada webhook workflow.
