package testing_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	composeengine "github.com/akordium-id/get-labuh/internal/compose"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/deploy"
	"github.com/akordium-id/get-labuh/internal/models"
	composemod "github.com/akordium-id/get-labuh/internal/modules/compose"
	testhelpers "github.com/akordium-id/get-labuh/internal/testing"
)

func TestDockerComposeStack_EndToEnd(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// 1. Setup Team, Project, and Environment
	teamID := "team-compose-e2e"
	projID := "proj-compose-e2e"
	envID := "env-compose-e2e"

	_, err := db.ExecContext(ctx, "INSERT INTO teams (id, name) VALUES (?, ?)", teamID, "E2E Team")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, "INSERT INTO projects (id, name, slug, team_id) VALUES (?, ?, ?, ?)", projID, "E2E Project", "e2e-project", teamID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, "INSERT INTO environments (id, project_id, name, slug) VALUES (?, ?, ?, ?)", envID, projID, "Production", "production")
	require.NoError(t, err)

	// 2. Setup Shared Environment Variables (Team & Project)
	sharedRepo := repo.NewSharedEnvVarRepo(db)
	err = sharedRepo.Set(ctx, &models.SharedEnvVar{
		Scope:   models.ScopeTeam,
		ScopeID: teamID,
		Key:     "ORG_DOMAIN",
		Value:   "akordium.com",
	})
	require.NoError(t, err)

	err = sharedRepo.Set(ctx, &models.SharedEnvVar{
		Scope:   models.ScopeProject,
		ScopeID: projID,
		Key:     "DATABASE_URL",
		Value:   "postgres://user:pass@db:5432/main",
	})
	require.NoError(t, err)

	// 3. Initialize Engine, Resolver, and Compose Module
	appEnvRepo := repo.NewEnvVarRepo(db)
	resolver := deploy.NewEnvResolver(sharedRepo, appEnvRepo, db)

	tempBase := filepath.Join(os.TempDir(), "labuh-compose-e2e-test")
	defer os.RemoveAll(tempBase)
	orch := composeengine.NewOrchestrator(tempBase, nil)

	mod := composemod.New(db, nil, nil)
	mod.SetOrchestrator(orch)
	mod.SetEnvResolver(resolver)

	// 4. Create Compose Application with Initial YAML
	initialYAML := `
version: '3.8'
services:
  web:
    image: nginx:alpine
    ports:
      - "80:80"
`
	app, err := mod.Repo.CreateWithContext(ctx, models.CreateComposeApplicationInput{
		EnvironmentID: envID,
		Name:          "Web Gateway Stack",
		RawYAML:       initialYAML,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, app.ID)

	// 5. Update YAML to Multi-Service (Web + Redis + Worker)
	multiYAML := `
version: '3.8'
services:
  web:
    image: nginx:alpine
    ports:
      - "8080:80"
    environment:
      - ORG_DOMAIN=${ORG_DOMAIN}
      - DATABASE_URL=${DATABASE_URL}
  redis:
    image: redis:alpine
    ports:
      - "6379:6379"
  worker:
    image: node:18-alpine
    depends_on:
      - redis
`
	formUp := url.Values{"raw_yaml": {multiYAML}}
	reqUp := httptest.NewRequest(http.MethodPost, "/compose-apps/"+app.ID+"/yaml", strings.NewReader(formUp.Encode()))
	reqUp.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctxUp := chi.NewRouteContext()
	rctxUp.URLParams.Add("id", app.ID)
	reqUp = reqUp.WithContext(context.WithValue(reqUp.Context(), chi.RouteCtxKey, rctxUp))
	wUp := httptest.NewRecorder()
	mod.Handler.UpdateYAML(wUp, reqUp)
	assert.Equal(t, http.StatusOK, wUp.Code)

	reloaded, err := mod.Repo.GetByIDWithContext(ctx, app.ID)
	require.NoError(t, err)
	assert.Len(t, reloaded.ParsedServices, 3)

	// 6. Verify WorkDir & Cascading .env Generation via Engine PrepareWorkDir
	envVars, err := resolver.ResolveForEnvironment(ctx, envID)
	require.NoError(t, err)
	require.NotEmpty(t, envVars)

	workDir, err := orch.PrepareWorkDir(reloaded, envVars)
	require.NoError(t, err)
	assert.DirExists(t, workDir)

	envContent, err := os.ReadFile(filepath.Join(workDir, ".env"))
	require.NoError(t, err)
	assert.Contains(t, string(envContent), "ORG_DOMAIN=\"akordium.com\"")
	assert.Contains(t, string(envContent), "DATABASE_URL=\"postgres://user:pass@db:5432/main\"")

	// 7. Verify Deploy Endpoint sets status to running and updates timestamp
	reqDeploy := httptest.NewRequest(http.MethodPost, "/compose-apps/"+app.ID+"/deploy", nil)
	rctxDep := chi.NewRouteContext()
	rctxDep.URLParams.Add("id", app.ID)
	reqDeploy = reqDeploy.WithContext(context.WithValue(reqDeploy.Context(), chi.RouteCtxKey, rctxDep))
	wDeploy := httptest.NewRecorder()
	mod.Handler.Deploy(wDeploy, reqDeploy)
	assert.Equal(t, http.StatusOK, wDeploy.Code)

	deployed, err := mod.Repo.GetByIDWithContext(ctx, app.ID)
	require.NoError(t, err)
	assert.NotNil(t, deployed.LastDeployedAt)

	// 8. Verify Stop Stack
	reqStop := httptest.NewRequest(http.MethodPost, "/compose-apps/"+app.ID+"/stop", nil)
	rctxStop := chi.NewRouteContext()
	rctxStop.URLParams.Add("id", app.ID)
	reqStop = reqStop.WithContext(context.WithValue(reqStop.Context(), chi.RouteCtxKey, rctxStop))
	wStop := httptest.NewRecorder()
	mod.Handler.Stop(wStop, reqStop)
	assert.Equal(t, http.StatusOK, wStop.Code)

	stopped, err := mod.Repo.GetByIDWithContext(ctx, app.ID)
	require.NoError(t, err)
	assert.Equal(t, models.ComposeStatusStopped, stopped.Status)
}
