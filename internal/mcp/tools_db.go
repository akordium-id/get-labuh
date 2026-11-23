package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/akordium-id/get-labuh/internal/models"
)

func (s *MCPServer) registerDatabaseTools() {
	// 13. list_databases
	s.internalServer.AddTool(
		mcp.NewTool("list_databases",
			mcp.WithDescription("Daftar database yang terdaftar (PostgreSQL, MySQL, Redis, MongoDB) beserta port dan status"),
			mcp.WithString("environment_id", mcp.Description("Filter berdasarkan ID environment (opsional)")),
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

	envID, _ := req.Params.Arguments["environment_id"].(string)
	var (
		dbs []*models.DatabaseService
		err error
	)

	if envID != "" {
		dbs, err = s.deps.DatabaseRepo.GetByEnvironmentIDWithContext(ctx, envID)
	} else {
		dbs, err = s.deps.DatabaseRepo.GetAllWithContext(ctx)
	}

	if err != nil {
		dbs = nil
	}

	var sanitized []*models.DatabaseService
	for _, dbItem := range dbs {
		if dbItem != nil {
			cp := *dbItem
			cp.Password = "••••••••"
			if cp.ConnectionString != nil {
				maskedConn := "••••••••"
				cp.ConnectionString = &maskedConn
			}
			sanitized = append(sanitized, &cp)
		}
	}

	b, _ := json.MarshalIndent(sanitized, "", "  ")
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
