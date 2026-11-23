package mcp

import (
	"github.com/mark3labs/mcp-go/server"
)

// ServeStdio menjalankan MCP server melalui transport standar input/output (JSON-RPC over stdin/stdout).
func (s *MCPServer) ServeStdio() error {
	return server.ServeStdio(s.internalServer)
}
