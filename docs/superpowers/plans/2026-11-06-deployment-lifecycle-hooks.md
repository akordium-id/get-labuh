# Deployment Lifecycle Hooks (Pre/Post Deployment Commands) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement pre-deployment and post-deployment lifecycle command execution for application containers during Blue-Green zero-downtime deployment in `get-labuh`.

**Architecture:** Application schema is extended with `pre_deployment_command` and `post_deployment_command`. During deployment in `internal/deploy/job.go`, the pre-deployment command runs inside the freshly started candidate container before healthcheck and traffic switch (aborting and rolling back if failed). The post-deployment command runs inside the active container after traffic is switched (logging a warning if failed, without failing the deployment).

**Tech Stack:** Go 1.23+, SQLite (`modernc.org/sqlite`), Docker SDK (`github.com/docker/docker`), Templ, Chi v5.

**Spec:** `docs/superpowers/specs/2026-11-06-deployment-lifecycle-hooks-design.md`

## Global Constraints

- Never mutate models struct signatures without updating all repository callers.
- Do not edit `*_templ.go` directly; always edit `*.templ` and run `templ generate`.
- Database migrations are sequential and append-only (`migrations/027_...`).
- Deployment steps must reflect in DB (`deployRepo.UpdateStep`) for real-time SSE progress.
- Command execution inside containers must be bound by a 5-minute timeout (`context.WithTimeout`).

## Review Focus

- Empty or whitespace-only command inputs must be normalized to `nil` in database.
- A failed pre-deployment command must immediately stop and remove the candidate container, leaving the active container running with 0 downtime.
- Pre-deployment command failure must mark the deployment step as `pre_deploy` with status `failed`.
- Post-deployment command failure must NOT roll back traffic or mark deployment as failed; it must log a visible warning.
- Special shell characters (pipes, `&&`, quotes) in command strings must execute correctly via `sh -c`.

---

### Task 1: Database Migration & Models

**Files:**
- Create: `migrations/027_deployment_lifecycle_commands.sql`
- Modify: `internal/models/application.go`
- Modify: `internal/models/deployment.go`
- Test: `internal/modules/applications/applications_test.go`

**Interfaces:**
- Produces:
  - `models.Application.PreDeploymentCommand (*string)`
  - `models.Application.PostDeploymentCommand (*string)`
  - `models.CreateApplicationInput.PreDeploymentCommand (*string)`
  - `models.CreateApplicationInput.PostDeploymentCommand (*string)`
  - `models.DeployStepPreDeploy = "pre_deploy"`
  - `models.DeployStepPostDeploy = "post_deploy"`

- [ ] **Step 1: Write the failing test for migration and model fields**

In `internal/modules/applications/applications_test.go`, add a test verifying that `Application` contains `PreDeploymentCommand` and `PostDeploymentCommand` and can be migrated on SQLite.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/modules/applications -run TestApplicationLifecycleFields`
Expected: FAIL (fields or migration not defined).

- [ ] **Step 3: Implement migration and update models**

1. Create `migrations/027_deployment_lifecycle_commands.sql`:
   ```sql
   ALTER TABLE applications ADD COLUMN pre_deployment_command TEXT;
   ALTER TABLE applications ADD COLUMN post_deployment_command TEXT;
   ```
2. In `internal/models/application.go`, add `PreDeploymentCommand *string` and `PostDeploymentCommand *string` to `Application` and `CreateApplicationInput`.
3. In `internal/models/deployment.go`, add `DeployStepPreDeploy DeployStep = "pre_deploy"` and `DeployStepPostDeploy DeployStep = "post_deploy"`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/modules/applications -run TestApplicationLifecycleFields`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add migrations/027_deployment_lifecycle_commands.sql internal/models/application.go internal/models/deployment.go internal/modules/applications/applications_test.go
git commit -m "feat(models): add pre and post deployment command fields and migration 027"
```

---

### Task 2: Repository Layer (CRUD & Updates)

**Files:**
- Modify: `internal/modules/applications/repo.go`
- Test: `internal/modules/applications/applications_test.go`

**Interfaces:**
- Consumes: `models.Application` fields from Task 1.
- Produces:
  - `repo.UpdateLifecycleCommands(ctx context.Context, id int64, preCmd, postCmd *string) error`
  - Updated `applicationColumns`, `Create`, `GetByID`, `ListByEnvironment` to read/write lifecycle columns.

- [ ] **Step 1: Write the failing test for UpdateLifecycleCommands and repo scan**

In `internal/modules/applications/applications_test.go`, test creating an application with pre/post deploy commands, reading it back via `GetByID`, and updating them via `UpdateLifecycleCommands`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/modules/applications -run TestUpdateLifecycleCommands`
Expected: FAIL (`UpdateLifecycleCommands` undefined or columns mismatch).

- [ ] **Step 3: Implement repository updates in `internal/modules/applications/repo.go`**

1. Update `applicationColumns` constant to include `pre_deployment_command, post_deployment_command`.
2. Update scan dest arguments in `scanApplication` / `GetByID` / `ListByEnvironment`.
3. Update `INSERT INTO applications` query in `Create` to persist `pre_deployment_command` and `post_deployment_command`.
4. Implement:
   ```go
   func (r *Repo) UpdateLifecycleCommands(ctx context.Context, id int64, preCmd, postCmd *string) error {
       query := `UPDATE applications SET pre_deployment_command = ?, post_deployment_command = ?, updated_at = ? WHERE id = ?`
       _, err := r.db.ExecContext(ctx, query, preCmd, postCmd, time.Now(), id)
       return err
   }
   ```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/modules/applications/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/modules/applications/repo.go internal/modules/applications/applications_test.go
git commit -m "feat(repo): support lifecycle commands query and UpdateLifecycleCommands"
```

---

### Task 3: Deployment Pipeline Lifecycle Execution

**Files:**
- Modify: `internal/deploy/job.go`
- Test: `internal/deploy/job_test.go`

**Interfaces:**
- Consumes:
  - `models.Application.PreDeploymentCommand`
  - `models.Application.PostDeploymentCommand`
  - `targetDockerClient.ExecStream(ctx, containerID, cmd, env, writer)`
  - `deployRepo.UpdateStep(jobID, step)`
- Produces: Zero-downtime execution with candidate rollback on pre-deploy failure, and non-fatal warning on post-deploy failure.

- [ ] **Step 1: Write failing unit tests for pipeline lifecycle hooks**

In `internal/deploy/job_test.go`:
1. `TestPipeline_PreDeployCommand_Success`: Pre-deploy command runs and succeeds, proceeds to healthcheck and done.
2. `TestPipeline_PreDeployCommand_Failure`: Pre-deploy command returns error; candidate container is removed; deployment fails with step `pre_deploy`; active container remains intact.
3. `TestPipeline_PostDeployCommand_Failure`: Post-deploy command returns error; deployment logs warning but successfully marks deploy step `done`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -v ./internal/deploy -run TestPipeline_PreDeployCommand`
Expected: FAIL.

- [ ] **Step 3: Implement lifecycle hooks in `internal/deploy/job.go`**

In `Pipeline.runApplicationDeploy`:
1. Retrieve `app.PreDeploymentCommand` and `app.PostDeploymentCommand` from `appRepo.GetByID`.
2. After candidate container starts (`targetDockerClient.RunContainerWithCustomName`):
   ```go
   if preCmd != "" {
       updateStep("pre_deploy", models.DeployStatusDeploying, "")
       writeLog(activeLogWriter, fmt.Sprintf("Running pre-deployment command: %s", preCmd))
       cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
       defer cancel()
       err := targetDockerClient.ExecStream(cmdCtx, candidateContainerID, []string{"sh", "-c", preCmd}, nil, activeLogWriter)
       if err != nil {
           errMsg := fmt.Sprintf("Pre-deployment command failed: %v. Rolling back...", err)
           writeLog(activeLogWriter, errMsg)
           _ = targetDockerClient.StopContainer(ctx, candidateContainerID, 5)
           _ = targetDockerClient.RemoveContainer(ctx, candidateContainerID)
           updateStep("pre_deploy", models.DeployStatusFailed, errMsg)
           return &DeployError{Step: "pre_deploy", Cause: err, Output: errMsg}
       }
       writeLog(activeLogWriter, "Pre-deployment command completed successfully.")
   }
   ```
3. Run healthcheck gate and Caddy route switch.
4. After Caddy route switch and DB update:
   ```go
   if postCmd != "" {
       updateStep("post_deploy", models.DeployStatusDeploying, "")
       writeLog(activeLogWriter, fmt.Sprintf("Running post-deployment command: %s", postCmd))
       cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
       defer cancel()
       if err := targetDockerClient.ExecStream(cmdCtx, candidateContainerID, []string{"sh", "-c", postCmd}, nil, activeLogWriter); err != nil {
           writeLog(activeLogWriter, fmt.Sprintf("[WARNING] Post-deployment command failed: %v (non-fatal)", err))
       } else {
           writeLog(activeLogWriter, "Post-deployment command completed successfully.")
       }
   }
   ```
5. Continue with old container cleanup and mark `done`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/deploy/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/deploy/job.go internal/deploy/job_test.go
git commit -m "feat(deploy): execute pre/post deploy lifecycle hooks in pipeline"
```

---

### Task 4: HTTP Handler & Routing for Lifecycle Commands

**Files:**
- Modify: `internal/modules/applications/handler.go`
- Modify: `internal/modules/applications/module.go` (if routes are registered in module router)
- Test: `internal/modules/applications/applications_test.go`

**Interfaces:**
- Consumes: `repo.UpdateLifecycleCommands`
- Produces: `POST /apps/{id}/lifecycle-commands` endpoint returning HTMX toast notification.

- [ ] **Step 1: Write failing HTTP handler test**

In `internal/modules/applications/applications_test.go`:
Send a POST request to `/apps/{id}/lifecycle-commands` with `pre_deployment_command` and `post_deployment_command`. Assert 200 OK and database updated.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/modules/applications -run TestHandleUpdateLifecycleCommands`
Expected: FAIL.

- [ ] **Step 3: Implement handler and route registration**

1. In `internal/modules/applications/handler.go`, add `HandleUpdateLifecycleCommands`:
   - Parse form values: `pre_deployment_command`, `post_deployment_command`.
   - Trim whitespace. If empty, convert to `nil`.
   - Call `h.repo.UpdateLifecycleCommands(r.Context(), appID, preCmd, postCmd)`.
   - Set HTMX trigger header `HX-Trigger: {"showToast": {"message": "Lifecycle commands updated", "type": "success"}}`.
   - Render or redirect with success.
2. Register route `r.Post("/{id}/lifecycle-commands", h.HandleUpdateLifecycleCommands)` in module route setup.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/modules/applications -run TestHandleUpdateLifecycleCommands`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/modules/applications/handler.go internal/modules/applications/module.go internal/modules/applications/applications_test.go
git commit -m "feat(apps): add endpoint to update application lifecycle commands"
```

---

### Task 5: UI Form in Templ & View Regeneration

**Files:**
- Modify: `internal/web/pages/applications/list.templ` (or `show.templ` in Advanced tab)
- Generate: `internal/web/pages/applications/list_templ.go`
- Test: `go test -v ./internal/web/...`

**Interfaces:**
- Renders: Card UI for "Lifecycle Hooks" with form posting to `/apps/{id}/lifecycle-commands`.

- [ ] **Step 1: Add Lifecycle Hooks Card to Templ file**

In the Advanced settings tab of the application detail page, add:
- Input / Textarea for `Pre-deployment Command` (`name="pre_deployment_command"`) with helper text: *"Executed inside candidate container before healthcheck and traffic switch (e.g. php artisan migrate --force)"*.
- Input / Textarea for `Post-deployment Command` (`name="post_deployment_command"`) with helper text: *"Executed after traffic switch (e.g. php artisan optimize:clear)"*.
- Submit button with HTMX post and loading indicator.

- [ ] **Step 2: Generate Templ Go code**

Run: `templ generate`
Expected: `list_templ.go` updated without errors.

- [ ] **Step 3: Verify build and UI tests**

Run: `go build ./...`
Expected: Exit code 0, no compilation errors.

- [ ] **Step 4: Commit**

```bash
git add internal/web/pages/applications/list.templ internal/web/pages/applications/list_templ.go
git commit -m "feat(ui): add lifecycle hooks form in application advanced settings"
```

---

### Task 6: Full Verification & Integration Check

**Files:**
- All touched files

- [ ] **Step 1: Run full test suite across all modules**

Run: `go test -v ./...`
Expected: All tests PASS.

- [ ] **Step 2: Verify binary build**

Run: `go build -o /dev/null ./cmd/labuh`
Expected: Successful compilation.

- [ ] **Step 3: Update documentation and personal-memory checkpoint**

Save checkpoint to `personal-memory` with status and next steps.
