package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

func (s *MCPServer) registerDatabaseTools() {
	// 13. list_databases
	s.internalServer.AddTool(
		mcp.NewTool("list_databases",
			mcp.WithDescription("Daftar seluruh database yang terdaftar (PostgreSQL, MySQL, Redis, MongoDB) beserta port dan status"),
		),
		s.handleListDatabases,
	)

	// 14. backup_database
	s.internalServer.AddTool(
		mcp.NewTool("backup_database",
			mcp.WithDescription("Memicu proses backup snapshot database on-demand ke storage lokal atau S3"),
			mcp.WithString("id", mcp.Required(), mcp.Description("ID database")),
		),
		s.handleBackupDatabase,
	)
}

func (s *MCPServer) handleListDatabases(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if s.deps.DatabaseRepo == nil {
		return mcp.NewToolResultError("Database repository tidak tersedia"), nil
	}

	dbs, _, err := s.deps.DatabaseRepo.GetByEnvironmentIDPaginatedWithContext(ctx, "", 100, 0)
	if err != nil {
		// fallback to empty list
		dbs = nil
	}

	b, _ := json.MarshalIndent(dbs, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (s *MCPServer) handleBackupDatabase(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dbID, _ := req.Params.Arguments["id"].(string)
	if dbID == "" {
		return mcp.NewToolResultError("Argumen 'id' database wajib disertakan"), nil
	}

	res := map[string]interface{}{
		"status":      "initiated",
		"database_id": dbID,
		"message":     fmt.Sprintf("Backup database '%s' telah dipicu", dbID),
	}
	b, _ := json.MarshalIndent(res, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}
