# Git PR/MR Preview Comment Bot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menambahkan bot komentar otomatis pada GitHub Pull Requests dan GitLab Merge Requests yang mempublikasikan status build, link live preview environment, error log, dan status teardown secara real-time dan anti-spam.

**Architecture:** Modul baru `internal/gitbot` menangani interaksi API GitHub & GitLab dengan strategi *anti-spam single comment update* via marker `<!-- labuh-pr-preview -->`. Token akses didapat secara berjenjang (override aplikasi terenkripsi di `applications.git_token`, lalu fallback ke global settings `github_bot_token` / `gitlab_bot_token`). Integrasi dipicu pada event webhook (`webhooks.go`) dan penyelesaian deploy job (`deploy_worker.go`).

**Tech Stack:** Go 1.23, Chi v5, SQLite (`modernc.org/sqlite`), GitHub REST API v3, GitLab REST API v4, Templ.

**Spec:** [`docs/superpowers/specs/2026-11-02-git-bot-comment-design.md`](file:///home/orin/code/archive/get-labuh/docs/superpowers/specs/2026-11-02-git-bot-comment-design.md)

## Global Constraints
- Token di database harus dienkripsi dengan `encryption.Encrypt` dan didekripsi dengan `encryption.Decrypt`.
- Marker komentar wajib: `<!-- labuh-pr-preview -->`.
- Jika token tidak dikonfigurasi, pengiriman komentar diabaikan tanpa melempar error ke deployment worker atau webhook response.
- Seluruh unit test suite harus tetap 100% PASS.

## Review Focus
- Format repository URL (HTTPS, SSH, custom domain gitlab) dapat di-parse dengan aman tanpa panic.
- Kegagalan HTTP dari GitHub/GitLab API tidak boleh menggagalkan alur build/deployment aplikasi.
- Update komentar harus mencari komentar yang dibuat oleh bot get-labuh (matching marker), jika belum ada baru melakukan POST.

---

### Task 1: Database Migration & Model Extensions for Git Token

**Files:**
- Create: `internal/database/migrations/026_git_bot_tokens.sql`
- Modify: `internal/models/application.go`
- Modify: `internal/modules/applications/repo.go`
- Modify: `internal/database/repo/setting_repo.go` (jika perlu inisialisasi default keys)

**Interfaces:**
- Produces: `models.Application.GitToken *string`, `Repo.UpdateGitTokenWithContext(ctx, id, token *string) error`

- [ ] **Step 1: Write migration file `026_git_bot_tokens.sql`**
```sql
ALTER TABLE applications ADD COLUMN git_token TEXT;
```

- [ ] **Step 2: Update models `Application` & `CreateApplicationInput`**
Tambahkan field `GitToken *string` pada struct di `internal/models/application.go`.

- [ ] **Step 3: Update `applications/repo.go`**
Perbarui `applicationColumns`, `scanApplication`, query `INSERT`, dan tambahkan method `UpdateGitTokenWithContext(ctx context.Context, id string, gitToken *string) error`.

- [ ] **Step 4: Run tests to verify model & migration**
Run: `go test -v -count=1 ./internal/modules/applications -run TestApplicationRepo_CRUD`
Expected: PASS

- [ ] **Step 5: Commit changes**
```bash
git add internal/database/migrations/026_git_bot_tokens.sql internal/models/application.go internal/modules/applications/repo.go
git commit -m "feat(db): add git_token to applications table and repo"
```

---

### Task 2: GitBot Client Package (`internal/gitbot`)

**Files:**
- Create: `internal/gitbot/client.go`
- Create: `internal/gitbot/parser.go`
- Create: `internal/gitbot/gitbot_test.go`

**Interfaces:**
- Produces: 
  - `gitbot.ParseRepo(rawURL string) (provider, ownerOrPath, repoName string, err error)`
  - `gitbot.Client` struct with `PostOrUpdateGitHubComment(ctx, repoURL string, prNumber int, token, body string) error` and `PostOrUpdateGitLabNote(ctx, repoURL string, mrIID int, token, body string) error`

- [ ] **Step 1: Write unit tests with mock HTTP server in `gitbot_test.go`**
Test:
1. `TestParseRepo`: parse berbagai variasi git URL (`https://github.com/org/repo.git`, `git@github.com:org/repo.git`, `https://gitlab.com/group/subgroup/project.git`).
2. `TestGitHub_CreateAndPatchComment`: mock GitHub issue comments API (cek POST jika tidak ada marker, cek PATCH jika marker ditemukan).
3. `TestGitLab_CreateAndPatchNote`: mock GitLab merge request notes API.

- [ ] **Step 2: Run test to verify it fails**
Run: `go test -v -count=1 ./internal/gitbot`
Expected: FAIL (package/functions not defined)

- [ ] **Step 3: Implement `parser.go` & `client.go`**
Implementasikan logic HTTP client untuk GitHub dan GitLab API dengan marker `<!-- labuh-pr-preview -->`.

- [ ] **Step 4: Run test to verify it passes**
Run: `go test -v -count=1 ./internal/gitbot`
Expected: PASS

- [ ] **Step 5: Commit changes**
```bash
git add internal/gitbot/
git commit -m "feat(gitbot): implement GitHub and GitLab PR/MR comment client"
```

---

### Task 3: Token Resolver Service & Webhooks Trigger

**Files:**
- Modify: `internal/modules/applications/webhooks.go`
- Modify: `internal/modules/applications/module.go`
- Modify: `internal/modules/applications/applications_test.go`

**Interfaces:**
- Consumes: `gitbot.Client`, `encryption.Decrypt`, `settingRepo.Get`
- Produces: Otomatis kirim komentar PR/MR pada status *Building...* dan *Destroyed* (teardown).

- [ ] **Step 1: Write failing test in `applications_test.go`**
Mock server GitHub API dan uji webhook `pull_request` opened mengirim komentar *Building...* dan closed mengirim komentar *Destroyed*.

- [ ] **Step 2: Run test to verify failure**
Run: `go test -v -count=1 -run TestApplications_PRPreviews_GitBot ./internal/modules/applications`
Expected: FAIL

- [ ] **Step 3: Implement token resolver & comment trigger in `webhooks.go`**
1. Helper `resolveGitToken(app *models.Application, provider string) string`.
2. Panggil `gitbot.PostOrUpdateComment` di `handleGitHubPullRequest` dan `handleGitLabMergeRequest`.

- [ ] **Step 4: Run test to verify it passes**
Run: `go test -v -count=1 -run TestApplications_PRPreviews_GitBot ./internal/modules/applications`
Expected: PASS

- [ ] **Step 5: Commit changes**
```bash
git add internal/modules/applications/webhooks.go internal/modules/applications/module.go internal/modules/applications/applications_test.go
git commit -m "feat(webhooks): integrate gitbot comments on PR lifecycle events"
```

---

### Task 4: Deploy Worker Deployment Completion Hook

**Files:**
- Modify: `internal/worker/deploy_worker.go`
- Modify: `internal/worker/deploy_worker_test.go` (atau test terkait)

**Interfaces:**
- Consumes: `gitbot.Client`, `app.IsPreview`, `app.ParentAppID`, `app.PRNumber`

- [ ] **Step 1: Add gitbot comment dispatch in `deploy_worker.go`**
Saat deploy job selesai:
Jika aplikasi adalah preview environment (`app.IsPreview && app.PRNumber != nil`), trigger komentar *Live & Healthy* (jika deploy sukses) atau *Deployment Failed* (jika deploy gagal) beserta URL preview.

- [ ] **Step 2: Run worker tests**
Run: `go test -v -count=1 ./internal/worker`
Expected: PASS

- [ ] **Step 3: Commit changes**
```bash
git add internal/worker/deploy_worker.go
git commit -m "feat(worker): update PR comment on preview deployment success or failure"
```

---

### Task 5: UI Configuration Forms (Application Advanced Tab & Settings)

**Files:**
- Modify: `internal/web/pages/applications/list.templ`
- Modify: `internal/web/pages/settings/` (atau page settings terkait)
- Modify: `internal/modules/applications/handler.go`
- Modify: `internal/modules/settings/handler.go`

- [ ] **Step 1: Update Application Advanced Tab in `list.templ`**
Tambahkan input field password/text *Git Provider Access Token (Optional Override)* di form PR Previews.

- [ ] **Step 2: Update Application Handler in `handler.go`**
Update `UpdatePRPreviewsConfig` untuk menyimpan `git_token` (dienkripsi dengan `encryption.Encrypt`).

- [ ] **Step 3: Add Global Bot Tokens in Settings Page**
Tambahkan field *GitHub Bot Token* & *GitLab Bot Token* di halaman pengaturan server.

- [ ] **Step 4: Generate templ & run tests**
Run: `export PATH=$PATH:$(go env GOPATH)/bin; templ generate && go test -count=1 ./...`
Expected: PASS

- [ ] **Step 5: Commit changes**
```bash
git add internal/web/ internal/modules/applications/handler.go internal/modules/settings/
git commit -m "feat(ui): add Git bot token fields to application settings and global settings"
```
