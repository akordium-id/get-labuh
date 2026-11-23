package mcp

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/akordium-id/get-labuh/internal/observability"
)

func (s *MCPServer) registerServerTools() {
	// 15. get_server_metrics
	s.internalServer.AddTool(
		mcp.NewTool("get_server_metrics",
			mcp.WithDescription("Mendapatkan metrik resource realtime host server: CPU load, RAM usage, dan kapasitas Disk"),
		),
		s.handleGetServerMetrics,
	)

	// 16. list_system_alerts
	s.internalServer.AddTool(
		mcp.NewTool("list_system_alerts",
			mcp.WithDescription("Melihat status kesehatan dan peringatan ambang batas resource server"),
		),
		s.handleListSystemAlerts,
	)
}

func (s *MCPServer) handleGetServerMetrics(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	stats := observability.GetHostStats("/")
	b, _ := json.MarshalIndent(stats, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}

func (s *MCPServer) handleListSystemAlerts(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	stats := observability.GetHostStats("/")

	status := "OK"
	var alerts []string

	if stats.DiskUsedPercent >= 90.0 {
		status = "CRITICAL"
		alerts = append(alerts, "Disk usage melebihi 90%")
	} else if stats.DiskUsedPercent >= 80.0 {
		status = "WARNING"
		alerts = append(alerts, "Disk usage melebihi 80%")
	}

	if stats.MemUsedPercent >= 95.0 {
		status = "CRITICAL"
		alerts = append(alerts, "Memory RAM usage melebihi 95%")
	} else if stats.MemUsedPercent >= 85.0 {
		if status != "CRITICAL" {
			status = "WARNING"
		}
		alerts = append(alerts, "Memory RAM usage melebihi 85%")
	}

	res := map[string]interface{}{
		"status":  status,
		"stats":   stats,
		"alerts":  alerts,
		"message": "Sistem berjalan normal",
	}
	if len(alerts) > 0 {
		res["message"] = "Terdapat peringatan pada resource host"
	}

	b, _ := json.MarshalIndent(res, "", "  ")
	return mcp.NewToolResultText(string(b)), nil
}
