package mcp_test

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

func TestMCPServer_FullToolsSuite(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// 1. Setup Data
	projID := "p-full"
	envID := "e-full"
	_, err := db.ExecContext(ctx, "INSERT INTO projects (id, name, slug) VALUES (?, ?, ?)", projID, "Full Proj", "full-proj")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, "INSERT INTO environments (id, project_id, name, slug) VALUES (?, ?, ?, ?)", envID, projID, "Prod", "prod")
	require.NoError(t, err)

	appRepo := repo.NewApplicationRepo(db)
	app, err := appRepo.CreateWithContext(ctx, models.CreateApplicationInput{
		EnvironmentID: envID,
		Name:          "API App",
		Slug:          "api-app",
		SourceType:    models.SourceTypeGit,
		AppPort:       5000,
	})
	require.NoError(t, err)

	sharedRepo := repo.NewSharedEnvVarRepo(db)
	appEnvRepo := repo.NewEnvVarRepo(db)
	deployRepo := repo.NewDeploymentRepo(db)
	deployWorker := worker.NewDeployWorker(10)
	resolver := deploy.NewEnvResolver(sharedRepo, appEnvRepo, db)

	server := labuhmcp.NewServer(labuhmcp.ServerDeps{
		AppRepo:       appRepo,
		DeployRepo:    deployRepo,
		SharedEnvRepo: sharedRepo,
		AppEnvRepo:    appEnvRepo,
		DeployWorker:  deployWorker,
		EnvResolver:   resolver,
	})

	mcpSrv := server.GetInternalServer()

	// 2. Test set_shared_env
	setSharedJSON := `{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"set_shared_env","arguments":{"scope":"project","scope_id":"` + projID + `","key":"COMMON_DB","value":"postgres.internal","is_secret":false}}}`
	resSetShared := mcpSrv.HandleMessage(ctx, []byte(setSharedJSON))
	require.NotNil(t, resSetShared)

	// 3. Test get_shared_env
	getSharedJSON := `{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"get_shared_env","arguments":{"scope":"project","scope_id":"` + projID + `"}}}`
	resGetShared := mcpSrv.HandleMessage(ctx, []byte(getSharedJSON))
	require.NotNil(t, resGetShared)
	b, _ := json.Marshal(resGetShared)
	assert.Contains(t, string(b), "COMMON_DB")

	// 4. Test set_application_env
	setAppEnvJSON := `{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"set_application_env","arguments":{"id":"` + app.ID + `","key":"PORT","value":"5000"}}}`
	resSetAppEnv := mcpSrv.HandleMessage(ctx, []byte(setAppEnvJSON))
	require.NotNil(t, resSetAppEnv)

	// 5. Test get_application_env (with inheritance)
	getAppEnvJSON := `{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"get_application_env","arguments":{"id":"` + app.ID + `","include_inherited":true}}}`
	resGetAppEnv := mcpSrv.HandleMessage(ctx, []byte(getAppEnvJSON))
	require.NotNil(t, resGetAppEnv)
	bEnv, _ := json.Marshal(resGetAppEnv)
	assert.Contains(t, string(bEnv), "COMMON_DB")
	assert.Contains(t, string(bEnv), "PORT")

	// 6. Test deploy_application
	deployJSON := `{"jsonrpc":"2.0","id":14,"method":"tools/call","params":{"name":"deploy_application","arguments":{"id":"` + app.ID + `"}}}`
	resDeploy := mcpSrv.HandleMessage(ctx, []byte(deployJSON))
	require.NotNil(t, resDeploy)
	bDep, _ := json.Marshal(resDeploy)
	assert.Contains(t, string(bDep), "queued")
	assert.Contains(t, string(bDep), "deployment_id")

	// 7. Test list_databases & backup_database
	listDBJSON := `{"jsonrpc":"2.0","id":15,"method":"tools/call","params":{"name":"list_databases","arguments":{}}}`
	resListDB := mcpSrv.HandleMessage(ctx, []byte(listDBJSON))
	require.NotNil(t, resListDB)

	backupJSON := `{"jsonrpc":"2.0","id":16,"method":"tools/call","params":{"name":"backup_database","arguments":{"id":"db-mock-1"}}}`
	resBackup := mcpSrv.HandleMessage(ctx, []byte(backupJSON))
	require.NotNil(t, resBackup)
	bBackup, _ := json.Marshal(resBackup)
	assert.Contains(t, string(bBackup), "initiated")

	// 8. Test list_system_alerts
	alertsJSON := `{"jsonrpc":"2.0","id":17,"method":"tools/call","params":{"name":"list_system_alerts","arguments":{}}}`
	resAlerts := mcpSrv.HandleMessage(ctx, []byte(alertsJSON))
	require.NotNil(t, resAlerts)
	bAlerts, _ := json.Marshal(resAlerts)
	assert.Contains(t, string(bAlerts), "status")

	// 9. Test rollback_application
	rollbackJSON := `{"jsonrpc":"2.0","id":18,"method":"tools/call","params":{"name":"rollback_application","arguments":{"id":"` + app.ID + `"}}}`
	resRollback := mcpSrv.HandleMessage(ctx, []byte(rollbackJSON))
	require.NotNil(t, resRollback)
	bRollback, _ := json.Marshal(resRollback)
	assert.Contains(t, string(bRollback), "rollback_initiated")
}
