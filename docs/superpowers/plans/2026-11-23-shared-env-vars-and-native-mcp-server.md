# Shared Environment Variables & Native Go MCP Server Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build cascading Shared Environment Variables (Team/Project inheritance) and a Native Go Model Context Protocol (MCP) Server with dual transport (HTTP SSE on Chi router + Stdio CLI) exposing 15 core operational tools for AI developer assistants.

**Architecture:** 
- Part A implements `shared_env_vars` SQLite table with AES-256 GCM encryption, a cascading `EnvResolver` engine enforcing `App > Project > Team` priority order, and Project-level UI management.
- Part B builds `internal/mcp` using `github.com/mark3labs/mcp-go`, wiring 15 tools directly to get-labuh's existing DI container, mounted as an authenticated HTTP SSE endpoint (`/mcp`) on Chi and as a CLI stdio subcommand (`labuh mcp`).

**Tech Stack:** Go 1.26, Chi v5, SQLite (`modernc.org/sqlite`), Docker SDK, `github.com/mark3labs/mcp-go`, Templ + Alpine.js.

**Spec:** [docs/superpowers/specs/2026-11-23-shared-env-vars-and-native-mcp-server-design.md](file:///home/orin/code/archive/get-labuh/docs/superpowers/specs/2026-11-23-shared-env-vars-and-native-mcp-server-design.md)

## Global Constraints

- 100% Zero-CGO compatibility (`CGO_ENABLED=0 go build`).
- AES-256 GCM encryption for all sensitive environment variable values.
- Thread-safe cascading variable resolution with strict deterministic override order (`App > Project > Team`).
- HTTP SSE transport must be protected by `middleware.RequireAPIKey`.
- Stdio CLI transport must direct internal operational logs to `os.Stderr` so JSON-RPC on `os.Stdout` is not corrupted.

## Review Focus

1. **Circular or Broken Inheritance Fallback**: An application whose project has no team ID must still resolve project and app variables cleanly without nil-pointer panics.
2. **Secret Decryption Failure**: A malformed or corrupted ciphertext in `shared_env_vars` must return a descriptive error without crashing the resolver or deploying unencrypted garbage.
3. **MCP Tool Argument Type Coercion**: A client passing optional arguments as null or omitting them (e.g. `force_rebuild` in `deploy_application`, `tail` in `get_application_logs`) must default gracefully.
4. **SSE Session Disconnect**: Client aborting the SSE connection mid-flight must cleanly unregister the session channel without leaking goroutines.
5. **Non-blocking Deployment Dispatch**: `deploy_application` tool call must enqueue the job asynchronously and return immediately with `queued` status, never blocking the MCP response until build completes.

---

### Task 1: Database Migration `032_shared_env_vars.sql` & Domain Models

**Files:**
- Create: `internal/database/migrations/032_shared_env_vars.sql`
- Create: `internal/models/shared_env_var.go`
- Test: `internal/database/repo/migration_test.go` (or inline migration runner test)

**Interfaces:**
- Produces:
  - `models.SharedEnvVarScope` (`ScopeProject = "project"`, `ScopeTeam = "team"`)
  - `models.SharedEnvVar` (`ID`, `Scope`, `ScopeID`, `Key`, `Value`, `IsSecret`, `IsEncrypted`, `CreatedAt`, `UpdatedAt`)
  - `models.ResolvedEnvVar` (`Key`, `Value`, `IsSecret`, `Source`)

- [ ] **Step 1: Write the failing test**

```go
// internal/database/repo/shared_env_migration_test.go
package repo_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
)

func TestMigration032_SharedEnvVars(t *testing.T) {
	db, err := database.Connect(":memory:")
	require.NoError(t, err)
	defer db.Close()

	var count int
	err = db.QueryRowContext(context.Background(), "SELECT count(*) FROM shared_env_vars").Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 0, count)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/database/repo -run TestMigration032_SharedEnvVars`
Expected: FAIL with "no such table: shared_env_vars"

- [ ] **Step 3: Implement migration and domain model**

1. Create `internal/database/migrations/032_shared_env_vars.sql`:
```sql
CREATE TABLE IF NOT EXISTS shared_env_vars (
    id TEXT PRIMARY KEY,
    scope TEXT NOT NULL CHECK(scope IN ('project', 'team')),
    scope_id TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    is_secret BOOLEAN DEFAULT false,
    is_encrypted BOOLEAN DEFAULT false,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(scope, scope_id, key)
);

CREATE INDEX IF NOT EXISTS idx_shared_env_vars_scope ON shared_env_vars(scope, scope_id);
```
2. Create `internal/models/shared_env_var.go` defining `SharedEnvVarScope`, `SharedEnvVar`, and `ResolvedEnvVar`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/database/repo -run TestMigration032_SharedEnvVars`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/database/migrations/032_shared_env_vars.sql internal/models/shared_env_var.go internal/database/repo/shared_env_migration_test.go
git commit -m "feat: add shared_env_vars migration and domain models"
```

---

### Task 2: Shared Environment Variables Repository & AES-256 Encryption

**Files:**
- Create: `internal/database/repo/shared_env_var_repo.go`
- Test: `internal/database/repo/shared_env_var_repo_test.go`

**Interfaces:**
- Consumes: `models.SharedEnvVar`, `encryption.Encrypt`, `encryption.Decrypt`
- Produces:
  - `repo.SharedEnvVarRepo`:
    - `ListByScope(ctx context.Context, scope models.SharedEnvVarScope, scopeID string) ([]*models.SharedEnvVar, error)`
    - `GetByKey(ctx context.Context, scope models.SharedEnvVarScope, scopeID, key string) (*models.SharedEnvVar, error)`
    - `Set(ctx context.Context, item *models.SharedEnvVar) error`
    - `Delete(ctx context.Context, id string) error`
    - `DeleteByKey(ctx context.Context, scope models.SharedEnvVarScope, scopeID, key string) error`
    - `BulkUpsert(ctx context.Context, scope models.SharedEnvVarScope, scopeID string, items []*models.SharedEnvVar) error`

- [ ] **Step 1: Write the failing test**

```go
// internal/database/repo/shared_env_var_repo_test.go
package repo_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
)

func TestSharedEnvVarRepo_CRUD(t *testing.T) {
	db, err := database.Connect(":memory:")
	require.NoError(t, err)
	defer db.Close()

	encKey := "0123456789abcdef0123456789abcdef" // 32-byte key
	r := repo.NewSharedEnvVarRepo(db, encKey)
	ctx := context.Background()

	// 1. Set item
	item := &models.SharedEnvVar{
		Scope:    models.ScopeProject,
		ScopeID:  "proj-123",
		Key:      "SHARED_SECRET",
		Value:    "super-secret-token",
		IsSecret: true,
	}
	err = r.Set(ctx, item)
	require.NoError(t, err)

	// 2. List & Verify Decryption
	list, err := r.ListByScope(ctx, models.ScopeProject, "proj-123")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "SHARED_SECRET", list[0].Key)
	assert.Equal(t, "super-secret-token", list[0].Value)
	assert.True(t, list[0].IsEncrypted)

	// 3. Upsert update
	item.Value = "updated-token"
	err = r.Set(ctx, item)
	require.NoError(t, err)

	updated, err := r.GetByKey(ctx, models.ScopeProject, "proj-123", "SHARED_SECRET")
	require.NoError(t, err)
	assert.Equal(t, "updated-token", updated.Value)

	// 4. Delete
	err = r.DeleteByKey(ctx, models.ScopeProject, "proj-123", "SHARED_SECRET")
	require.NoError(t, err)

	listAfter, err := r.ListByScope(ctx, models.ScopeProject, "proj-123")
	require.NoError(t, err)
	assert.Empty(t, listAfter)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/database/repo -run TestSharedEnvVarRepo_CRUD`
Expected: FAIL with undefined `repo.NewSharedEnvVarRepo`

- [ ] **Step 3: Implement `SharedEnvVarRepo` in `internal/database/repo/shared_env_var_repo.go`**

Implement UPSERT with SQLite `ON CONFLICT(scope, scope_id, key) DO UPDATE SET value=excluded.value, is_secret=excluded.is_secret, is_encrypted=excluded.is_encrypted, updated_at=CURRENT_TIMESTAMP`. Automatically encrypt values using `encryption.EncryptAESGCM` when `encKey != ""`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/database/repo -run TestSharedEnvVarRepo_CRUD`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/database/repo/shared_env_var_repo.go internal/database/repo/shared_env_var_repo_test.go
git commit -m "feat: implement SharedEnvVarRepo with AES-256 GCM encryption"
```

---

### Task 3: Cascading EnvResolver Engine

**Files:**
- Create: `internal/deploy/env_resolver.go`
- Test: `internal/deploy/env_resolver_test.go`

**Interfaces:**
- Consumes: `repo.SharedEnvVarRepo`, `repo.AppEnvVarRepo`, `repo.ProjectRepo`
- Produces:
  - `deploy.NewEnvResolver(sharedRepo, appEnvRepo, projectRepo, encKey string) *EnvResolver`
  - `(r *EnvResolver) ResolveForApplication(ctx context.Context, app *models.Application) ([]models.AppEnvVar, error)`
  - `(r *EnvResolver) ResolveDetailed(ctx context.Context, app *models.Application) ([]models.ResolvedEnvVar, error)`

- [ ] **Step 1: Write the failing test**

```go
// internal/deploy/env_resolver_test.go
package deploy_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/deploy"
	"github.com/akordium-id/get-labuh/internal/models"
)

func TestEnvResolver_CascadingPrecedence(t *testing.T) {
	db, err := database.Connect(":memory:")
	require.NoError(t, err)
	defer db.Close()

	encKey := "0123456789abcdef0123456789abcdef"
	sharedRepo := repo.NewSharedEnvVarRepo(db, encKey)
	appEnvRepo := repo.NewAppEnvVarRepo(db)
	projectRepo := repo.NewProjectRepo(db)

	ctx := context.Background()

	// Setup Project with Team
	projectID := "proj-alpha"
	teamID := "team-1"
	err = projectRepo.CreateWithTeam(ctx, &models.Project{ID: projectID, Name: "Alpha", TeamID: &teamID})
	require.NoError(t, err)

	// 1. Team Level: FOO=team_foo, SHARED_ALL=team_shared
	require.NoError(t, sharedRepo.Set(ctx, &models.SharedEnvVar{
		Scope: models.ScopeTeam, ScopeID: teamID, Key: "FOO", Value: "team_foo",
	}))
	require.NoError(t, sharedRepo.Set(ctx, &models.SharedEnvVar{
		Scope: models.ScopeTeam, ScopeID: teamID, Key: "SHARED_ALL", Value: "team_shared",
	}))

	// 2. Project Level: FOO=project_foo (overrides team), PROJ_ONLY=project_val
	require.NoError(t, sharedRepo.Set(ctx, &models.SharedEnvVar{
		Scope: models.ScopeProject, ScopeID: projectID, Key: "FOO", Value: "project_foo",
	}))
	require.NoError(t, sharedRepo.Set(ctx, &models.SharedEnvVar{
		Scope: models.ScopeProject, ScopeID: projectID, Key: "PROJ_ONLY", Value: "project_val",
	}))

	// 3. App Level: FOO=app_foo (overrides project & team), APP_ONLY=app_val
	appID := "app-web"
	app := &models.Application{ID: appID, Name: "Web", ProjectID: &projectID}
	require.NoError(t, appEnvRepo.Set(ctx, &models.AppEnvVar{
		ApplicationID: appID, Key: "FOO", Value: "app_foo",
	}))
	require.NoError(t, appEnvRepo.Set(ctx, &models.AppEnvVar{
		ApplicationID: appID, Key: "APP_ONLY", Value: "app_val",
	}))

	resolver := deploy.NewEnvResolver(sharedRepo, appEnvRepo, projectRepo, encKey)

	// Test ResolveForApplication
	merged, err := resolver.ResolveForApplication(ctx, app)
	require.NoError(t, err)

	toMap := func(vars []models.AppEnvVar) map[string]string {
		m := make(map[string]string)
		for _, v := range vars {
			m[v.Key] = v.Value
		}
		return m
	}
	resMap := toMap(merged)

	assert.Equal(t, "app_foo", resMap["FOO"])        // App overrides Project & Team
	assert.Equal(t, "project_val", resMap["PROJ_ONLY"]) // Inherited from Project
	assert.Equal(t, "team_shared", resMap["SHARED_ALL"]) // Inherited from Team
	assert.Equal(t, "app_val", resMap["APP_ONLY"])     // App specific

	// Test ResolveDetailed with sources
	detailed, err := resolver.ResolveDetailed(ctx, app)
	require.NoError(t, err)
	sourceMap := make(map[string]string)
	for _, d := range detailed {
		sourceMap[d.Key] = d.Source
	}
	assert.Equal(t, "app", sourceMap["FOO"])
	assert.Equal(t, "project", sourceMap["PROJ_ONLY"])
	assert.Equal(t, "team", sourceMap["SHARED_ALL"])
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/deploy -run TestEnvResolver_CascadingPrecedence`
Expected: FAIL with undefined `deploy.NewEnvResolver`

- [ ] **Step 3: Implement `EnvResolver` in `internal/deploy/env_resolver.go`**

Implement deterministic cascading merge with map iteration and sorting by key. Gracefully handle nil `app.ProjectID` or nil `project.TeamID`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/deploy -run TestEnvResolver_CascadingPrecedence`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/deploy/env_resolver.go internal/deploy/env_resolver_test.go
git commit -m "feat: implement cascading EnvResolver with App > Project > Team precedence"
```

---

### Task 4: Pipeline & Worker Integration with EnvResolver

**Files:**
- Modify: `internal/worker/deploy_worker.go`
- Modify: `internal/app/handlers.go` (or DI wiring container)
- Test: `internal/worker/deploy_worker_test.go`

**Interfaces:**
- Consumes: `deploy.EnvResolver`
- Produces: `DeployWorker.SetEnvResolver(resolver *deploy.EnvResolver)`

- [ ] **Step 1: Write the failing test**

```go
// Add test case in internal/worker/deploy_worker_test.go
func TestDeployWorker_ResolvesInheritedEnvVars(t *testing.T) {
    // Assert BuildDeploymentJobWithResolver passes inherited variables into Job.EnvVars
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/worker -run TestDeployWorker_ResolvesInheritedEnvVars`
Expected: FAIL

- [ ] **Step 3: Integrate EnvResolver into Worker**

In `internal/worker/deploy_worker.go`:
1. Add `envResolver *deploy.EnvResolver` field to `DeployWorker`.
2. When dequeuing and processing a deployment, call `w.envResolver.ResolveForApplication(ctx, app)` so `job.EnvVars` contains the resolved cascading variables.
3. Wire `EnvResolver` in `internal/app/handlers.go`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/worker`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/worker/deploy_worker.go internal/app/handlers.go internal/worker/deploy_worker_test.go
git commit -m "feat: integrate cascading EnvResolver into DeployWorker pipeline"
```

---

### Task 5: Project Shared Variables UI & HTTP Handlers

**Files:**
- Create: `internal/web/pages/project_env.templ`
- Modify: `internal/handler/projects.go`
- Modify: `internal/web/pages/projects.templ` (or project detail view)
- Test: `internal/handler/projects_test.go`

**Interfaces:**
- Produces:
  - Route `GET /projects/{id}/env`: renders shared variables tab
  - Route `POST /projects/{id}/env`: handles single variable upsert
  - Route `POST /projects/{id}/env/bulk`: handles bulk `.env` format upsert
  - Route `POST /projects/{id}/env/delete`: handles variable deletion

- [ ] **Step 1: Write the failing test**

```go
// internal/handler/projects_env_test.go
package handler_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/handler"
	"github.com/akordium-id/get-labuh/internal/models"
)

func TestProjectsHandler_SharedEnv(t *testing.T) {
	db, err := database.Connect(":memory:")
	require.NoError(t, err)
	defer db.Close()

	encKey := "0123456789abcdef0123456789abcdef"
	sharedRepo := repo.NewSharedEnvVarRepo(db, encKey)
	projRepo := repo.NewProjectRepo(db)

	proj := &models.Project{ID: "p1", Name: "Project 1"}
	require.NoError(t, projRepo.Create(context.Background(), proj))

	h := handler.NewProjectsHandler(projRepo, sharedRepo)

	// POST /projects/p1/env
	form := url.Values{}
	form.Set("key", "COMMON_URL")
	form.Set("value", "https://api.example.com")
	form.Set("is_secret", "false")

	req := httptest.NewRequest(http.MethodPost, "/projects/p1/env", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.SetSharedEnv(w, req)
	assert.Equal(t, http.StatusSeeOther, w.Code)

	list, err := sharedRepo.ListByScope(context.Background(), models.ScopeProject, "p1")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "COMMON_URL", list[0].Key)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/handler -run TestProjectsHandler_SharedEnv`
Expected: FAIL

- [ ] **Step 3: Implement handler methods and Templ components**

1. Implement `SetSharedEnv`, `BulkSetSharedEnv`, and `DeleteSharedEnv` in `internal/handler/projects.go`.
2. Create `internal/web/pages/project_env.templ` with tab navigation and `.env` bulk import modal.
3. Run `templ generate` to compile templates.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/handler -run TestProjectsHandler_SharedEnv`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/handler/projects.go internal/web/pages/project_env* internal/handler/projects_test.go
git commit -m "feat: add Project Shared Environment Variables UI and handlers"
```

---

### Task 6: Native Go MCP Server Core & 15 Tools Implementation

**Files:**
- Create: `internal/mcp/server.go`
- Create: `internal/mcp/tools_apps.go`
- Create: `internal/mcp/tools_env.go`
- Create: `internal/mcp/tools_db.go`
- Create: `internal/mcp/tools_server.go`
- Test: `internal/mcp/server_test.go`
- Test: `internal/mcp/tools_test.go`

**Interfaces:**
- Consumes: `mcp-go/server`, `repo.ApplicationRepo`, `repo.DeploymentRepo`, `repo.SharedEnvVarRepo`, `repo.DatabaseServiceRepo`, `deploy.EnvResolver`, `worker.DeployWorker`, `docker.ClientPool`, `observability.GetHostStats`
- Produces:
  - `mcp.NewServer(deps ServerDeps) *MCPServer`
  - `(s *MCPServer) GetInternalServer() *server.MCPServer`
  - 15 Tools: `list_applications`, `get_application`, `deploy_application`, `get_deployment_status`, `get_application_logs`, `rollback_application`, `restart_application`, `stop_application`, `get_application_env`, `set_application_env`, `get_shared_env`, `set_shared_env`, `list_databases`, `backup_database`, `get_server_metrics`, `list_system_alerts`

- [ ] **Step 1: Write the failing test**

```go
// internal/mcp/server_test.go
package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	labuhmcp "github.com/akordium-id/get-labuh/internal/mcp"
	"github.com/akordium-id/get-labuh/internal/models"
)

func TestMCPServer_ListApplicationsAndMetrics(t *testing.T) {
	db, err := database.Connect(":memory:")
	require.NoError(t, err)
	defer db.Close()

	appRepo := repo.NewApplicationRepo(db)
	ctx := context.Background()

	require.NoError(t, appRepo.Create(ctx, &models.Application{
		ID: "app-test-1", Name: "Test App", Domain: "test.example.com", Status: "running",
	}))

	server := labuhmcp.NewServer(labuhmcp.ServerDeps{
		AppRepo: appRepo,
	})

	// Call list_applications via JSON-RPC HandleMessage
	reqJSON := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_applications","arguments":{}}}`
	res := server.GetInternalServer().HandleMessage(ctx, []byte(reqJSON))
	require.NotNil(t, res)

	resBytes, err := json.Marshal(res)
	require.NoError(t, err)
	assert.Contains(t, string(resBytes), "app-test-1")
	assert.Contains(t, string(resBytes), "test.example.com")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/mcp`
Expected: FAIL with undefined `labuhmcp`

- [ ] **Step 3: Implement MCP Server and 15 Tool Handlers**

1. Create `internal/mcp/server.go`: Initialize `server.NewMCPServer("get-labuh", "1.0.0", server.WithToolCapabilities(true), server.WithLogging())`.
2. Implement tools in modular files (`tools_apps.go`, `tools_env.go`, `tools_db.go`, `tools_server.go`).
3. Handle error checks, missing ID lookups, and format outputs cleanly using `mcp.NewToolResultText()`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/mcp`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/ internal/mcp/server_test.go internal/mcp/tools_test.go
git commit -m "feat: implement native Go MCP server with 15 core operational tools"
```

---

### Task 7: Dual Transports: HTTP SSE on Chi Router & Stdio CLI Subcommand

**Files:**
- Create: `internal/mcp/transport_http.go`
- Create: `internal/mcp/transport_stdio.go`
- Modify: `cmd/labuh/main.go`
- Test: `internal/mcp/transport_http_test.go`
- Test: `internal/mcp/transport_stdio_test.go`

**Interfaces:**
- Consumes: `mcp.MCPServer`, `middleware.RequireAPIKey`, `chi.Router`
- Produces:
  - `(s *MCPServer) MountHTTP(r chi.Router, apiKeyRepo *repo.ApiKeyRepo)`
  - `(s *MCPServer) ServeStdio() error`
  - Subcommand `labuh mcp`

- [ ] **Step 1: Write the failing test**

```go
// internal/mcp/transport_http_test.go
package mcp_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	labuhmcp "github.com/akordium-id/get-labuh/internal/mcp"
)

func TestMCPTransportHTTP_AuthRejection(t *testing.T) {
	db, err := database.Connect(":memory:")
	require.NoError(t, err)
	defer db.Close()

	apiKeyRepo := repo.NewApiKeyRepo(db)
	mcpServer := labuhmcp.NewServer(labuhmcp.ServerDeps{})

	r := chi.NewRouter()
	mcpServer.MountHTTP(r, apiKeyRepo)

	// Unauthorized GET request to /mcp/sse
	req := httptest.NewRequest(http.MethodGet, "/mcp/sse", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/mcp -run TestMCPTransportHTTP_AuthRejection`
Expected: FAIL

- [ ] **Step 3: Implement HTTP SSE and Stdio Transports**

1. In `internal/mcp/transport_http.go`:
   Create `server.NewSSEServer(s.internalServer, server.WithBasePath("/mcp"))` and mount to Chi router protected by `middleware.RequireAPIKey(apiKeyRepo)`.
2. In `internal/mcp/transport_stdio.go`:
   Implement `ServeStdio()` wrapping `server.ServeStdio(s.internalServer)`.
3. In `cmd/labuh/main.go`:
   Check `if len(os.Args) > 1 && os.Args[1] == "mcp"` to launch stdio mode; otherwise mount `/mcp` in the main web server.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v ./internal/mcp`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/mcp/transport* cmd/labuh/main.go
git commit -m "feat: add dual MCP transports (HTTP SSE on Chi and Stdio CLI)"
```

---

### Task 8: End-to-End Integration Suite & Concurrency Verification

**Files:**
- Create: `internal/testing/shared_env_and_mcp_e2e_test.go`

**Interfaces:**
- Tests full flow: Set Shared Project Env -> Trigger Deployment via MCP Tool -> Verify Cascading Env Resolution & Job Enqueue.

- [ ] **Step 1: Write E2E Integration Test**

```go
// internal/testing/shared_env_and_mcp_e2e_test.go
package testing_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/deploy"
	labuhmcp "github.com/akordium-id/get-labuh/internal/mcp"
	"github.com/akordium-id/get-labuh/internal/models"
)

func TestSharedEnvAndMCP_EndToEnd(t *testing.T) {
	db, err := database.Connect(":memory:")
	require.NoError(t, err)
	defer db.Close()

	encKey := "0123456789abcdef0123456789abcdef"
	sharedRepo := repo.NewSharedEnvVarRepo(db, encKey)
	appEnvRepo := repo.NewAppEnvVarRepo(db)
	projectRepo := repo.NewProjectRepo(db)
	appRepo := repo.NewApplicationRepo(db)
	deployRepo := repo.NewDeploymentRepo(db)

	ctx := context.Background()

	// 1. Create Project & Shared Env via MCP tool
	projID := "proj-fintech"
	require.NoError(t, projectRepo.Create(ctx, &models.Project{ID: projID, Name: "Fintech"}))

	resolver := deploy.NewEnvResolver(sharedRepo, appEnvRepo, projectRepo, encKey)
	mcpServer := labuhmcp.NewServer(labuhmcp.ServerDeps{
		AppRepo:       appRepo,
		DeployRepo:    deployRepo,
		SharedEnvRepo: sharedRepo,
		AppEnvRepo:    appEnvRepo,
		EnvResolver:   resolver,
	})

	// Call set_shared_env via MCP
	setEnvJSON := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"set_shared_env","arguments":{"scope":"project","scope_id":"proj-fintech","key":"DB_CLUSTER","value":"postgres-cluster-01"}}}`
	res := mcpServer.GetInternalServer().HandleMessage(ctx, []byte(setEnvJSON))
	require.NotNil(t, res)

	// 2. Create Application under Project
	appID := "app-api"
	require.NoError(t, appRepo.Create(ctx, &models.Application{
		ID: appID, Name: "Fintech API", ProjectID: &projID, Status: "stopped",
	}))

	// 3. Query get_application_env via MCP with include_inherited: true
	getEnvJSON := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_application_env","arguments":{"id":"app-api","include_inherited":true}}}`
	resEnv := mcpServer.GetInternalServer().HandleMessage(ctx, []byte(getEnvJSON))
	require.NotNil(t, resEnv)

	resEnvBytes, err := json.Marshal(resEnv)
	require.NoError(t, err)
	assert.Contains(t, string(resEnvBytes), "DB_CLUSTER")
	assert.Contains(t, string(resEnvBytes), "postgres-cluster-01")
	assert.Contains(t, string(resEnvBytes), "project")
}
```

- [ ] **Step 2: Run E2E test to verify it passes**

Run: `go test -v -race ./internal/testing -run TestSharedEnvAndMCP_EndToEnd`
Expected: PASS

- [ ] **Step 3: Run full repository regression and race verification**

Run: `go test -race ./...`
Expected: All packages PASS with zero data races.

- [ ] **Step 4: Verify Zero-CGO binary build**

Run: `CGO_ENABLED=0 go build -v -o /dev/null ./cmd/labuh`
Expected: Clean build with exit code 0.

- [ ] **Step 5: Commit**

```bash
git add internal/testing/shared_env_and_mcp_e2e_test.go
git commit -m "test: add end-to-end integration suite for shared env vars and native mcp server"
```
