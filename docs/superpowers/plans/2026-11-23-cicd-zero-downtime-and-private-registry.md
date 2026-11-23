# CI/CD Pipeline, True Zero-Downtime Deployments, & Private Docker Registry Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement true zero-downtime deployments with healthcheck gating and connection draining, universal CI/CD webhook ingestion with GitHub/GitLab commit status reporting, and private Docker registry authentication (GHCR, ECR, Docker Hub Private).

**Architecture:**
- Remove the 3-second bypass in `WaitForHealthCustom`, enforce active HTTP/TCP polling and container crash detection, and add a 5-second graceful connection draining window after Caddy cutover in `internal/deploy/job.go`.
- Expand `internal/modules/applications/webhooks.go` to accept dynamic `commit`, `branch`, and `image_tag` payloads.
- Integrate GitHub/GitLab Commit Status API in `internal/gitbot/client.go` and hook into deployment pipeline states (`pending`, `success`, `failure`).
- Introduce `docker_registries` table, AES-256-GCM credential encryption, base64 `X-Registry-Auth` injection in Docker SDK `PullImageWithAuth`, and UI in Templ for registry management.

**Tech Stack:** Go (1.23+), Chi v5, Docker Moby SDK, modernc.org/sqlite, AES-256-GCM (`internal/encryption`), Templ, HTMX, Alpine.js.

**Spec:** [docs/superpowers/specs/2026-11-23-cicd-zero-downtime-and-private-registry-design.md](file:///home/orin/code/archive/get-labuh/docs/superpowers/specs/2026-11-23-cicd-zero-downtime-and-private-registry-design.md)

---

## Global Constraints

- Database migrations must be sequentially numbered in `internal/database/migrations/` (`030_docker_registries.sql`). Past migrations must remain untouched.
- Models in `internal/models/` must remain pure domain structs without internal package imports.
- SQLite queries must use parameterized placeholders (`?`) and accept `context.Context`.
- Sensitive passwords and tokens must be encrypted using `internal/encryption` and hidden from JSON (`json:"-"`).
- Tests must execute on SQLite in-memory (`database.Connect(":memory:")`) or mock HTTP clients without external network calls.
- Templ files must always be compiled with `templ generate`. Never edit `*_templ.go` directly.

---

### Task 1: Database Migration `030_docker_registries.sql` & Models

**Files:**
- Create: `internal/database/migrations/030_docker_registries.sql`
- Create: `internal/models/registry.go`
- Modify: `internal/models/application.go`
- Create: `internal/models/registry_test.go`

**Interfaces:**
- Produces: `models.DockerRegistry`, `models.CreateDockerRegistryInput`, `models.UpdateDockerRegistryInput`
- Updates: `models.Application` with `DockerRegistryID *string`, `CreateApplicationInput`, `DeploymentJob`

- [ ] **Step 1: Write model tests verifying `DockerRegistry` struct**

Create `internal/models/registry_test.go` testing instantiation and JSON serialization (ensuring password is not serialized in JSON).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/models -run TestDockerRegistryModel`
Expected: FAIL (models undefined)

- [ ] **Step 3: Define domain models and SQL migration**

- Create `internal/models/registry.go` with `DockerRegistry`, `CreateDockerRegistryInput`, `UpdateDockerRegistryInput`.
- Update `internal/models/application.go` with `DockerRegistryID *string` on `Application`, `CreateApplicationInput`, and `DeploymentJob`.
- Create `internal/database/migrations/030_docker_registries.sql` creating table `docker_registries` and altering `applications`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/models -run TestDockerRegistryModel`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/database/migrations/030_docker_registries.sql internal/models/registry.go internal/models/registry_test.go internal/models/application.go
git commit -m "feat(models): add docker registry models and migration 030"
```

---

### Task 2: Docker Registry Repository CRUD with Enkripsi

**Files:**
- Create: `internal/modules/settings/repo_registry.go`
- Create: `internal/modules/settings/repo_registry_test.go`

**Interfaces:**
- Produces:
  - `(r *RegistryRepo) Create(ctx context.Context, input models.CreateDockerRegistryInput) (*models.DockerRegistry, error)`
  - `(r *RegistryRepo) GetByID(ctx context.Context, id string) (*models.DockerRegistry, error)`
  - `(r *RegistryRepo) List(ctx context.Context) ([]*models.DockerRegistry, error)`
  - `(r *RegistryRepo) Update(ctx context.Context, input models.UpdateDockerRegistryInput) error`
  - `(r *RegistryRepo) Delete(ctx context.Context, id string) error`

- [ ] **Step 1: Write repository tests in `repo_registry_test.go`**

Test CRUD operations with password encryption verification (raw DB contains `ENC:`, decrypted model returns plaintext password).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/modules/settings -run TestRegistryRepo`
Expected: FAIL (types undefined)

- [ ] **Step 3: Implement `RegistryRepo` in `repo_registry.go`**

Implement CRUD methods using SQLite and `encryption.Encrypt` / `encryption.Decrypt`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/modules/settings -run TestRegistryRepo`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/settings/repo_registry.go internal/modules/settings/repo_registry_test.go
git commit -m "feat(settings): add docker registry repository with aes encryption"
```

---

### Task 3: Docker SDK `PullImageWithAuth` & `VerifyRegistryAuth`

**Files:**
- Modify: `internal/docker/client.go`
- Create: `internal/docker/client_auth_test.go`

**Interfaces:**
- Produces:
  - `type RegistryAuth struct { Username, Password, ServerAddress, Email string }`
  - `(c *Client) PullImageWithAuth(ctx context.Context, imageRef string, auth *RegistryAuth) error`
  - `(c *Client) VerifyRegistryAuth(ctx context.Context, auth *RegistryAuth) error`

- [ ] **Step 1: Write test for Docker client auth encoding in `client_auth_test.go`**

Test that `RegistryAuth` correctly serializes and base64 encodes headers expected by Docker Engine API.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/docker -run TestRegistryAuthEncoding`
Expected: FAIL

- [ ] **Step 3: Implement `PullImageWithAuth` and `VerifyRegistryAuth` in `client.go`**

Implement auth header generation using base64 URL encoding and Docker API options.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/docker -run TestRegistryAuthEncoding`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/docker/client.go internal/docker/client_auth_test.go
git commit -m "feat(docker): add authenticated image pull and verification"
```

---

### Task 4: True Zero-Downtime Healthcheck Gate & Graceful Connection Draining

**Files:**
- Modify: `internal/docker/container.go`
- Modify: `internal/deploy/job.go`
- Create: `internal/docker/container_health_test.go`
- Create: `internal/deploy/job_drain_test.go`

**Interfaces:**
- Updates: `WaitForHealthCustom` (remove 3s bypass, early fail on dead container, active HTTP probe)
- Updates: `Pipeline.Execute` (add 5s connection draining window & 15s graceful container stop)

- [ ] **Step 1: Write tests for healthcheck gate in `container_health_test.go`**

Test that:
- Exited container fails immediately without waiting for timeout.
- Healthy container (HTTP 200/302) resolves promptly.
- Non-responding container waits until timeout and returns error.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/docker -run TestWaitForHealthCustom`
Expected: FAIL

- [ ] **Step 3: Update `WaitForHealthCustom` in `container.go` and add draining in `job.go`**

- Remove 3s bypass: `if time.Since(startedAt) >= 3*time.Second { return nil }`.
- Check `insp.State.Running == false` -> return error immediately.
- Probe HTTP endpoint; if status < 500 return nil.
- Fallback to TCP dial if path is empty.
- In `job.go`, after `AddRouteWithOptions`, add 5s draining sleep before stopping old container.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -v ./internal/docker -run TestWaitForHealthCustom && go test -v ./internal/deploy -run TestJob`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/docker/container.go internal/docker/container_health_test.go internal/deploy/job.go internal/deploy/job_drain_test.go
git commit -m "feat(deploy): implement true zero-downtime healthcheck gate and connection draining"
```

---

### Task 5: GitHub & GitLab Commit Status in GitBot Client

**Files:**
- Modify: `internal/gitbot/client.go`
- Modify: `internal/deploy/job.go`
- Create: `internal/gitbot/commit_status_test.go`

**Interfaces:**
- Produces:
  - `(c *Client) SetGitHubCommitStatus(ctx context.Context, repoURL, commitSHA, token, state, targetURL, description, contextName string) error`
  - `(c *Client) SetGitLabCommitStatus(ctx context.Context, repoURL, commitSHA, token, state, targetURL, description, contextName string) error`

- [ ] **Step 1: Write test for commit status reporting in `commit_status_test.go`**

Use mock HTTP server to assert correct endpoint `POST /repos/{owner}/{repo}/statuses/{sha}` and JSON payload.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/gitbot -run TestCommitStatus`
Expected: FAIL

- [ ] **Step 3: Implement commit status methods and wire into deployment pipeline**

- Implement `SetGitHubCommitStatus` and `SetGitLabCommitStatus` in `client.go`.
- In `internal/deploy/job.go`, call commit status on start (`pending`), failure (`failure`), and completion (`success`).

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/gitbot -run TestCommitStatus`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/gitbot/client.go internal/gitbot/commit_status_test.go internal/deploy/job.go
git commit -m "feat(gitbot): add github and gitlab commit status reporting"
```

---

### Task 6: CI/CD Webhook Enhancements (`commit`, `branch`, `image_tag`)

**Files:**
- Modify: `internal/modules/applications/webhooks.go`
- Modify: `internal/modules/applications/repo.go`
- Create: `internal/modules/applications/webhooks_test.go`

**Interfaces:**
- Produces:
  - `(r *Repo) UpdateDockerImage(ctx context.Context, id, dockerImage string) error`
  - Universal payload parser recognizing `commit`, `commit_hash`, `branch`, `image_tag`.

- [ ] **Step 1: Write tests for enhanced webhook parser in `webhooks_test.go`**

Verify that:
- `{ "commit": "abc", "branch": "main" }` is parsed identically to `{ "commit_hash": "abc" }`.
- `{ "image_tag": "myrepo/app:v1.2" }` updates image for `docker_image` application.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/modules/applications -run TestUniversalWebhook`
Expected: FAIL

- [ ] **Step 3: Implement payload enhancements and `UpdateDockerImage`**

- In `webhooks.go`, expand `genericWebhookPayload` and handler logic.
- In `repo.go`, implement `UpdateDockerImage(ctx, id, dockerImage)`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/modules/applications -run TestUniversalWebhook`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/applications/webhooks.go internal/modules/applications/repo.go internal/modules/applications/webhooks_test.go
git commit -m "feat(applications): support flexible ci/cd webhook payloads and dynamic image tags"
```

---

### Task 7: Docker Registry HTTP Handlers & Routes in Settings

**Files:**
- Modify: `internal/modules/settings/handler.go`
- Modify: `internal/modules/settings/module.go`
- Create: `internal/modules/settings/registries_handler_test.go`

**Interfaces:**
- Produces routes:
  - `GET /settings/registries`
  - `POST /settings/registries`
  - `POST /settings/registries/{id}/delete`
  - `POST /settings/registries/test`

- [ ] **Step 1: Write handler tests in `registries_handler_test.go`**

Test registry listing, creation with form values, test connection, and deletion.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/modules/settings -run TestRegistriesHandler`
Expected: FAIL

- [ ] **Step 3: Implement handlers and register routes in `module.go`**

Add registry handlers to `internal/modules/settings/handler.go` and wire them into `RegisterRoutes` in `module.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/modules/settings -run TestRegistriesHandler`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/modules/settings/handler.go internal/modules/settings/module.go internal/modules/settings/registries_handler_test.go
git commit -m "feat(settings): add docker registries management handlers and routes"
```

---

### Task 8: UI in Templ (`settings/registries.templ` & `applications/list.templ`)

**Files:**
- Create: `internal/web/pages/settings/registries.templ`
- Modify: `internal/web/pages/applications/list.templ`
- Modify: `internal/modules/applications/handler.go` (pass registries list to detail view)
- Generate: `templ generate`

- [ ] **Step 1: Create `registries.templ`**

Build clean UI with list of configured registries, Add Registry form, connection test button, and delete confirmation.

- [ ] **Step 2: Update `applications/list.templ`**

Add Docker Registry selection dropdown in Application Configuration / Docker Image card.

- [ ] **Step 3: Run `templ generate` and verify build**

Run: `templ generate && go test -v ./internal/web/...`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/web/pages/settings/registries.templ internal/web/pages/settings/registries_templ.go internal/web/pages/applications/list.templ internal/web/pages/applications/list_templ.go internal/modules/applications/handler.go
git commit -m "feat(ui): add docker registries page and application registry selector"
```

---

### Task 9: End-to-End Integration Tests & Full Regression Suite

**Files:**
- Create: `internal/testing/cicd_zerodowntime_registry_test.go`

- [ ] **Step 1: Write end-to-end integration tests**

Simulate:
- Webhook trigger with dynamic `image_tag`.
- Registry authentication injection during pull.
- True zero-downtime gate waiting for container to become healthy.
- Verification that old container is drained and cleaned up without dropping requests.
- Commit status notification sent to mock Git server.

- [ ] **Step 2: Run all tests in the repository**

Run: `go test -v ./...`
Expected: ALL PACKAGES PASS

- [ ] **Step 3: Commit**

```bash
git add internal/testing/cicd_zerodowntime_registry_test.go
git commit -m "test(integration): add end-to-end tests for cicd, zero-downtime, and registry auth"
```
