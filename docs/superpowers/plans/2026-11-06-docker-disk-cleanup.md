# Automated Docker Disk Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement automated and manual Docker disk cleanup (pruning dangling images, build cache, stopped candidate containers, and unused networks) while safeguarding tagged rollback cache images in `get-labuh`.

**Architecture:** Docker client subsystem is enhanced with `PruneSystem(ctx, opts)` utilizing native Docker SDK prune APIs with `dangling=true` filtering for safe image removal. A background `CleanupScheduler` in the settings module periodically checks cron expressions stored in SQLite `settings` table and triggers cleanup, with on-demand manual trigger via HTTP endpoint and Templ dashboard view.

**Tech Stack:** Go 1.23+, Docker SDK (`github.com/docker/docker`), Chi v5, Templ, SQLite (`modernc.org/sqlite`).

**Spec:** `docs/superpowers/specs/2026-11-06-docker-disk-cleanup-design.md`

## Global Constraints

- Never mutate models struct signatures without updating all repository callers.
- Do not edit `*_templ.go` directly; always edit `*.templ` and run `templ generate`.
- Image pruning MUST use `dangling=true` to protect the 5 cached images per app used by the instant rollback feature.
- Cleanup scheduler must stop gracefully when the application terminates.

## Review Focus

- Dormant or uninitialized Docker client must return a clear error or safe empty report rather than panicking.
- Space reclaimed calculation must handle uint64 overflow and properly format bytes into B, KB, MB, and GB.
- Manual trigger (`/settings/cleanup/run`) must immediately record last run timestamp and space reclaimed in the `settings` table.
- Cron scheduler must gracefully handle malformed or empty cron expressions by defaulting or logging warnings without crashing.
- Audit log must record the cleanup execution with the total space reclaimed.

---

### Task 1: Docker Client Prune API & Helper

**Files:**
- Create: `internal/docker/cleanup.go`
- Create: `internal/docker/cleanup_test.go`

**Interfaces:**
- Produces:
  - `docker.PruneOptions`
  - `docker.PruneReport`
  - `docker.FormatBytes(bytes uint64) string`
  - `(*Client).PruneSystem(ctx context.Context, opts PruneOptions) (*PruneReport, error)`

- [ ] **Step 1: Write failing test for PruneSystem and FormatBytes**

In `internal/docker/cleanup_test.go`, test `FormatBytes` across various sizes (0 B, 1024 B, 1.5 MB, 2.5 GB) and `PruneSystem` with dormant and mocked/live Docker client.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/docker -run TestFormatBytes`
Expected: FAIL (`FormatBytes` undefined).

- [ ] **Step 3: Implement PruneSystem and FormatBytes in `internal/docker/cleanup.go`**

Implement options struct, report struct, `FormatBytes`, and `PruneSystem` using `ContainersPrune`, `ImagesPrune(dangling=true)`, `BuildCachePrune(All=true)`, and `NetworksPrune`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/docker -run "TestFormatBytes|TestPruneSystem"`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/docker/cleanup.go internal/docker/cleanup_test.go
git commit -m "feat(docker): add PruneSystem and FormatBytes helper"
```

---

### Task 2: Background CleanupScheduler & Settings Service

**Files:**
- Create: `internal/modules/settings/scheduler.go`
- Create: `internal/modules/settings/scheduler_test.go`

**Interfaces:**
- Consumes:
  - `docker.Client.PruneSystem`
  - `repo.SettingRepo`
  - `repo.AuditLogRepo`
- Produces:
  - `NewCleanupScheduler(...) *CleanupScheduler`
  - `(*CleanupScheduler).Start()`
  - `(*CleanupScheduler).Stop()`
  - `(*CleanupScheduler).RunCleanup(ctx context.Context) (*docker.PruneReport, error)`

- [ ] **Step 1: Write failing test for CleanupScheduler**

In `internal/modules/settings/scheduler_test.go`, test `RunCleanup` to verify that `PruneSystem` is executed, audit log is written, and settings `docker_cleanup_last_run` and `docker_cleanup_last_reclaimed` are updated in SQLite.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/modules/settings -run TestCleanupScheduler_RunCleanup`
Expected: FAIL (`CleanupScheduler` undefined).

- [ ] **Step 3: Implement `CleanupScheduler` in `internal/modules/settings/scheduler.go`**

Implement ticker loop, cron matching logic, `RunCleanup`, and persistence updates.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/modules/settings -run TestCleanupScheduler_RunCleanup`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/modules/settings/scheduler.go internal/modules/settings/scheduler_test.go
git commit -m "feat(settings): add background CleanupScheduler daemon"
```

---

### Task 3: HTTP Handlers & Routing in Settings Module

**Files:**
- Modify: `internal/modules/settings/handler.go`
- Modify: `internal/modules/settings/module.go`
- Modify: `internal/modules/settings/settings_test.go`

**Interfaces:**
- Consumes: `CleanupScheduler.RunCleanup`
- Produces:
  - `GET /settings/cleanup`
  - `POST /settings/cleanup`
  - `POST /settings/cleanup/run`

- [ ] **Step 1: Write failing handler tests**

In `internal/modules/settings/settings_test.go`, write tests for saving cleanup settings and running manual cleanup via HTTP POST.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/modules/settings -run TestSettingsHandler_Cleanup`
Expected: FAIL.

- [ ] **Step 3: Implement handlers and register routes**

Add `GetCleanup`, `UpdateCleanupSettings`, and `RunCleanupNow` to `Handler` and register in `RegisterRoutes`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/modules/settings -run TestSettingsHandler_Cleanup`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/modules/settings/handler.go internal/modules/settings/module.go internal/modules/settings/settings_test.go
git commit -m "feat(settings): add cleanup endpoints and routing"
```

---

### Task 4: UI Templ View & Navigation

**Files:**
- Create: `internal/web/pages/settings/cleanup.templ`
- Modify: `internal/web/layouts/app.templ`
- Generate: `internal/web/pages/settings/cleanup_templ.go`
- Test: `go build ./...`

**Interfaces:**
- Renders:
  - Metrics card (Last run, Space reclaimed, Status).
  - 1-Click "Run Cleanup Now" button.
  - Scheduling form (Enabled toggle, Preset/Custom cron).

- [ ] **Step 1: Implement `cleanup.templ` and update app layout**

Add the cleanup page template and add a navigation link under Settings sidebar.

- [ ] **Step 2: Generate Templ code**

Run: `go run github.com/a-h/templ/cmd/templ@latest generate`
Expected: `cleanup_templ.go` generated without errors.

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: Exit code 0.

- [ ] **Step 4: Commit**

```bash
git add internal/web/pages/settings/cleanup.templ internal/web/pages/settings/cleanup_templ.go internal/web/layouts/app.templ internal/web/layouts/app_templ.go
git commit -m "feat(ui): add Docker Cleanup page and navigation"
```

---

### Task 5: App Container Lifecycle Wiring

**Files:**
- Modify: `internal/app/handlers.go`
- Modify: `internal/app/app.go`
- Test: `internal/app/`

**Interfaces:**
- Wires:
  - Start `cleanupScheduler` in `handlers.go`.
  - Graceful stop via `defer cleanupScheduler.Stop()` in `app.go`.

- [ ] **Step 1: Wire up scheduler startup and shutdown**

In `internal/app/handlers.go` instantiate and start `CleanupScheduler`. In `internal/app/app.go` defer `Stop()`.

- [ ] **Step 2: Verify compilation and tests**

Run: `go test -v ./internal/app/...`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/app/handlers.go internal/app/app.go
git commit -m "feat(app): wire up CleanupScheduler in app container startup and shutdown"
```

---

### Task 6: Full Verification & Integration Check

**Files:**
- All touched files

- [ ] **Step 1: Run full test suite across all modules**

Run: `go test ./...`
Expected: All tests PASS.

- [ ] **Step 2: Verify binary build**

Run: `go build -o /dev/null ./cmd/labuh`
Expected: 0 errors.

- [ ] **Step 3: Save personal-memory checkpoint**
