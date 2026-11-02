# Git PR/MR Preview Comment Bot

Fitur ini menyediakan notifikasi status interaktif langsung di GitHub Pull Request atau GitLab Merge Request setiap kali Ephemeral PR Previews dibuat, diperbarui, berhasil dideploy, gagal, atau dihapus (teardown).

---

## 1. Arsitektur & Siklus Hidup Komentar

```
[GitHub PR / GitLab MR Event]
              │
              ▼
    [Webhooks Handler]
              │──> 1. Status: "Deploying..." (Buat komentar pertama)
              ▼
      [Deploy Worker]
              │
       ┌──────┴──────┐
       ▼             ▼
 [Deploy Sukses]  [Deploy Gagal]
       │             │
       ▼             ▼
  2. Status:     2. Status:
  "Live & Healthy" "Failed" (Error msg)
  (Preview Link)
              │
[PR / MR Ditutup (Closed / Merged)]
              │
              ▼
    [Webhooks Handler]
              │──> 3. Status: "Destroyed" (Environment dihapus)
```

### Anti-Spam Single-Comment Update
Untuk menghindari spamming timeline PR/MR, bot menyisipkan marker HTML tersembunyi `<!-- labuh-pr-preview -->` pada setiap komentar.
- Saat event baru terjadi, bot mencari komentar yang memiliki marker tersebut.
- Jika ditemukan, bot akan **meng-update** komentar tersebut (via `PATCH /repos/{owner}/{repo}/issues/comments/{id}` di GitHub atau `PUT /projects/{id}/merge_requests/{iid}/notes/{note_id}` di GitLab).
- Jika belum ada, bot membuat komentar baru.

---

## 2. Autentikasi & Resolusi Token

Get-Labuh mendukung dua level konfigurasi token:

1. **Override per Aplikasi (Disimpan Enkripsi)**
   - Dapat diatur di dashboard aplikasi: **Tab Advanced → PR Previews (Ephemeral Envs)**.
   - Input: **Git Bot Token (GitHub / GitLab)**.
   - Nilai token langsung dienkripsi menggunakan AES sebelum disimpan di database (`applications.git_token`).

2. **Global Fallback (Dashboard Settings)**
   - Dapat diatur di: **Settings → Caddy & Git Settings → Global Git Bot Tokens**.
   - `github_bot_token`: Token default untuk semua repositori GitHub.
   - `gitlab_bot_token`: Token default untuk semua repositori GitLab.

### Token Permissions / Scopes yang Dibutuhkan:
- **GitHub Personal Access Token (PAT)**:
  - Fine-grained token: Permission `Pull requests: Read and write`.
  - Classic token: Scope `repo` (atau `public_repo` untuk repositori publik).
- **GitLab Access Token**:
  - Scope: `api` (atau `read_api` + role minimal `Reporter`/`Developer` pada project).

---

## 3. Format & Status Indikator

| Status | Emoji / Badge | Deskripsi |
|---|---|---|
| `building` | ⏳ **Deploying...** | Container sedang dibangun dan dipersiapkan |
| `live` | 🚀 **Live & Healthy** | Preview environment aktif dan lulus healthcheck (dilengkapi link domain) |
| `failed` | ❌ **Deployment Failed** | Deployment gagal beserta pesan error teknis |
| `destroyed` | 🛑 **Environment Destroyed** | Environment preview telah dibersihkan setelah PR/MR ditutup |
