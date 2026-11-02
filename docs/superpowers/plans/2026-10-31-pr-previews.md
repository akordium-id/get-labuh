# PR Previews (Ephemeral Environments) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Menambahkan kapabilitas Git Webhook Auto-Deploy untuk Pull Request (PR) Previews (Ephemeral Environments) dengan siklus hidup otomatis (deploy on PR open/sync, teardown container & domain on PR close/merge).

**Architecture:** Memanfaatkan relasi parent-child di tabel `applications`, mengkloning environment variables dari aplikasi induk, membuat subdomain Caddy dinamis, dan menghubungkan event webhook GitHub (`pull_request`) serta GitLab (`Merge Request Hook`).

**Tech Stack:** Go (Chi router, Docker SDK, Caddy Admin API, SQLite, modernc.org/sqlite, Templ, Alpine.js).

**Spec:** `docs/superpowers/specs/2026-10-31-pr-previews-design.md`

## Global Constraints
- SQLite migration tidak boleh mengedit migration yang sudah ada; gunakan `migrations/025_ephemeral_pr_previews.sql`.
- Model domain `models.Application` murni tanpa dependensi eksternal.
- Templ files harus selalu di-generate via `export PATH=$PATH:$(go env GOPATH)/bin; templ generate`.

## Review Focus
- Webhook HMAC signature verification harus tetap terjaga sebelum payload PR dievaluasi.
- PR number & ParentAppID harus unik agar tidak ada duplikasi preview application untuk PR yang sama.
- Saat PR closed, pastikan container dihentikan & dihapus, route Caddy dicabut, dan data di-cleanup.
- Jika `PRPreviewsEnabled` bernilai `false`, event PR harus di-ignore secara anggun (`{"status":"ignored"}`).
- Kloning env vars harus menyertakan secret dan encryption flag dengan benar.

---

### Task 1: Database Migration & Model Extensions
**Files:**
- Create: `internal/database/migrations/025_ephemeral_pr_previews.sql`
- Modify: `internal/models/application.go`
- Modify: `internal/modules/applications/repo.go`

**Interfaces:**
- Produces: `app.ParentAppID`, `app.PRNumber`, `app.IsPreview`, `app.PreviewTemplateDomain`, `app.PRPreviewsEnabled`

- [ ] **Step 1: Write migration SQL**
Tambah kolom `parent_app_id`, `pr_number`, `is_preview`, `preview_template_domain`, dan `pr_previews_enabled` ke tabel `applications`.

- [ ] **Step 2: Update `models/application.go`**
Tambah field-field tersebut ke struct `Application` dan `CreateApplicationInput`.

- [ ] **Step 3: Update `applications/repo.go`**
Perbarui scan query `SELECT` dan insert query di `Repo` untuk menyertakan 5 field baru, serta tambahkan method `GetByParentAndPR(ctx, parentID, prNumber)` dan `ListPreviews(ctx, parentID)`.

- [ ] **Step 4: Verify test suite**
Run: `go test -count=1 ./internal/modules/applications`
Expected: PASS

---

### Task 2: Webhook Engine PR Lifecycle (GitHub & GitLab)
**Files:**
- Modify: `internal/modules/applications/webhooks.go`
- Test: `internal/modules/applications/webhooks_test.go`

**Interfaces:**
- Consumes: `appRepo.GetByParentAndPR`, `appRepo.Create`, `deployWorker.Enqueue`, `caddyClient.RemoveRoute`, `dockerClient.RemoveContainer`
- Produces: Webhook handling untuk `pull_request` (open/sync/close) dan `Merge Request Hook`.

- [ ] **Step 1: Write test for PR Open & Synchronize**
Tulis unit test simulasi GitHub payload event `pull_request` dengan action `opened`.

- [ ] **Step 2: Implement PR Open logic in `handleGitHub`**
Parsing payload PR (`number`, `head.ref`, `head.sha`, `action`), cek `PRPreviewsEnabled`, generate slug preview, kloning env vars, buat deployment, dan enqueue job.

- [ ] **Step 3: Write test for PR Close (Teardown)**
Tulis unit test simulasi payload event `pull_request` dengan action `closed`.

- [ ] **Step 4: Implement PR Close teardown logic**
Stop & remove Docker container, remove route Caddy via `caddyClient.RemoveRoute`, dan delete record preview app.

- [ ] **Step 5: Implement GitLab Merge Request Hook**
Adaptasi logika serupa untuk event GitLab `Merge Request Hook`.

- [ ] **Step 6: Run tests and verify**
Run: `go test -count=1 ./internal/modules/applications`
Expected: PASS

---

### Task 3: UI Tab "PR Previews" & Config Settings
**Files:**
- Modify: `internal/web/pages/applications/list.templ`
- Modify: `internal/modules/applications/handler.go`
- Modify: `internal/app/routes.go` (jika ada action manual teardown)

**Interfaces:**
- Consumes: `appRepo.ListPreviews(app.ID)`
- Produces: Render tab PR Previews dan toggle config di tab Advanced.

- [ ] **Step 1: Update Application Detail Handler**
Query `previews, _ := h.appRepo.ListPreviews(r.Context(), app.ID)` dan oper ke template `ApplicationDetailPage`.

- [ ] **Step 2: Add PR Previews Tab in `list.templ`**
Tambah tab `#previews` dengan tabel daftar ephemeral environments (PR #, branch, commit, preview URL, status, tombol Destroy).

- [ ] **Step 3: Add Config Form in Tab `#advanced`**
Checkbox untuk mengaktifkan PR Previews dan input template domain (`{app}-pr-{pr}.domain.com`).

- [ ] **Step 4: Re-generate Templ & Format**
Run: `export PATH=$PATH:$(go env GOPATH)/bin; templ generate && go fmt ./...`

- [ ] **Step 5: Verify Full Test Suite**
Run: `go test -count=1 ./...`
Expected: PASS
