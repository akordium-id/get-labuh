package testing_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/deploy"
	labuhmcp "github.com/akordium-id/get-labuh/internal/mcp"
	"github.com/akordium-id/get-labuh/internal/models"
	testhelpers "github.com/akordium-id/get-labuh/internal/testing"
	"github.com/akordium-id/get-labuh/internal/worker"
)

func TestSharedEnvAndMCP_EndToEnd(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// 1. Setup Team, Project, and Environment in Database
	teamID := "team-fintech"
	projID := "proj-fintech"
	envID := "env-fintech"

	_, err := db.ExecContext(ctx, "INSERT INTO teams (id, name) VALUES (?, ?)", teamID, "Fintech Core")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, "INSERT INTO projects (id, name, slug, team_id) VALUES (?, ?, ?, ?)", projID, "Fintech Platform", "fintech-platform", teamID)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, "INSERT INTO environments (id, project_id, name, slug) VALUES (?, ?, ?, ?)", envID, projID, "Production", "production")
	require.NoError(t, err)

	appRepo := repo.NewApplicationRepo(db)
	appEnvRepo := repo.NewEnvVarRepo(db)
	sharedRepo := repo.NewSharedEnvVarRepo(db)
	deployRepo := repo.NewDeploymentRepo(db)
	deployWorker := worker.NewDeployWorker(10)
	resolver := deploy.NewEnvResolver(sharedRepo, appEnvRepo, db)
	deployWorker.SetEnvResolver(resolver)

	server := labuhmcp.NewServer(labuhmcp.ServerDeps{
		AppRepo:       appRepo,
		DeployRepo:    deployRepo,
		SharedEnvRepo: sharedRepo,
		AppEnvRepo:    appEnvRepo,
		DeployWorker:  deployWorker,
		EnvResolver:   resolver,
	})

	mcpSrv := server.GetInternalServer()

	// 2. Set Shared Team Env via MCP tool: ORG_DOMAIN=fintech.internal
	setTeamEnvJSON := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"set_shared_env","arguments":{"scope":"team","scope_id":"` + teamID + `","key":"ORG_DOMAIN","value":"fintech.internal"}}}`
	resTeam := mcpSrv.HandleMessage(ctx, []byte(setTeamEnvJSON))
	require.NotNil(t, resTeam)

	// 3. Set Shared Project Env via MCP tool: DB_CLUSTER=postgres-cluster-01
	setProjEnvJSON := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"set_shared_env","arguments":{"scope":"project","scope_id":"` + projID + `","key":"DB_CLUSTER","value":"postgres-cluster-01"}}}`
	resProj := mcpSrv.HandleMessage(ctx, []byte(setProjEnvJSON))
	require.NotNil(t, resProj)

	// 4. Create Application under Environment
	app, err := appRepo.CreateWithContext(ctx, models.CreateApplicationInput{
		EnvironmentID: envID,
		Name:          "Fintech API",
		Slug:          "fintech-api",
		SourceType:    models.SourceTypeGit,
		AppPort:       8080,
	})
	require.NoError(t, err)

	// 5. Set Application-specific variable via MCP tool: PORT=8080
	setAppEnvJSON := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"set_application_env","arguments":{"id":"` + app.ID + `","key":"PORT","value":"8080"}}}`
	resAppEnv := mcpSrv.HandleMessage(ctx, []byte(setAppEnvJSON))
	require.NotNil(t, resAppEnv)

	// 6. Query get_application_env with inheritance via MCP tool
	getEnvJSON := `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get_application_env","arguments":{"id":"` + app.ID + `","include_inherited":true}}}`
	resGetEnv := mcpSrv.HandleMessage(ctx, []byte(getEnvJSON))
	require.NotNil(t, resGetEnv)

	bEnv, err := json.Marshal(resGetEnv)
	require.NoError(t, err)
	envStr := string(bEnv)

	// Verify all 3 tiers of cascading inheritance are present
	assert.Contains(t, envStr, "ORG_DOMAIN")
	assert.Contains(t, envStr, "fintech.internal")
	assert.Contains(t, envStr, "team")

	assert.Contains(t, envStr, "DB_CLUSTER")
	assert.Contains(t, envStr, "postgres-cluster-01")
	assert.Contains(t, envStr, "project")

	assert.Contains(t, envStr, "PORT")
	assert.Contains(t, envStr, "8080")
	assert.Contains(t, envStr, "app")

	// 7. Trigger deploy_application via MCP tool
	deployJSON := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"deploy_application","arguments":{"id":"` + app.ID + `"}}}`
	resDeploy := mcpSrv.HandleMessage(ctx, []byte(deployJSON))
	require.NotNil(t, resDeploy)

	bDeploy, err := json.Marshal(resDeploy)
	require.NoError(t, err)
	assert.Contains(t, string(bDeploy), "queued")
	assert.Contains(t, string(bDeploy), "deployment_id")
}
