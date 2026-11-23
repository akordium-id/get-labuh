package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	labuhmcp "github.com/akordium-id/get-labuh/internal/mcp"
	"github.com/akordium-id/get-labuh/internal/models"
	testhelpers "github.com/akordium-id/get-labuh/internal/testing"
)

func TestMCPServer_ListApplicationsAndMetrics(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	defer db.Close()

	ctx := context.Background()

	// Setup Project and Environment
	projID := "p-mcp"
	envID := "e-mcp"
	_, err := db.ExecContext(ctx, "INSERT INTO projects (id, name, slug) VALUES (?, ?, ?)", projID, "MCP Proj", "mcp-proj")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, "INSERT INTO environments (id, project_id, name, slug) VALUES (?, ?, ?, ?)", envID, projID, "Prod", "prod")
	require.NoError(t, err)

	appRepo := repo.NewApplicationRepo(db)
	app, err := appRepo.CreateWithContext(ctx, models.CreateApplicationInput{
		EnvironmentID: envID,
		Name:          "MCP App",
		Slug:          "mcp-app",
		SourceType:    models.SourceTypeGit,
		AppPort:       8080,
	})
	require.NoError(t, err)

	server := labuhmcp.NewServer(labuhmcp.ServerDeps{
		AppRepo: appRepo,
	})

	// 1. Call list_applications via JSON-RPC
	reqJSON := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_applications","arguments":{}}}`
	res := server.GetInternalServer().HandleMessage(ctx, []byte(reqJSON))
	require.NotNil(t, res)

	resBytes, err := json.Marshal(res)
	require.NoError(t, err)
	assert.Contains(t, string(resBytes), app.ID)
	assert.Contains(t, string(resBytes), "MCP App")

	// 2. Call get_application
	getAppJSON := `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"get_application","arguments":{"id":"` + app.ID + `"}}}`
	resGet := server.GetInternalServer().HandleMessage(ctx, []byte(getAppJSON))
	require.NotNil(t, resGet)

	resGetBytes, err := json.Marshal(resGet)
	require.NoError(t, err)
	assert.Contains(t, string(resGetBytes), "8080")
	assert.Contains(t, string(resGetBytes), "mcp-app")

	// 3. Call get_server_metrics
	metricsJSON := `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_server_metrics","arguments":{}}}`
	resMetrics := server.GetInternalServer().HandleMessage(ctx, []byte(metricsJSON))
	require.NotNil(t, resMetrics)

	resMetricsBytes, err := json.Marshal(resMetrics)
	require.NoError(t, err)
	assert.Contains(t, string(resMetricsBytes), "mem_total")
	assert.Contains(t, string(resMetricsBytes), "disk_total")
}
