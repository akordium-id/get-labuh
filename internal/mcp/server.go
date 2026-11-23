package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/server"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/deploy"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/worker"
)

// ServerDeps membungkus seluruh dependensi repositori, worker, dan client yang dibutuhkan oleh tools MCP.
type ServerDeps struct {
	AppRepo       *repo.ApplicationRepo
	DeployRepo    *repo.DeploymentRepo
	SharedEnvRepo *repo.SharedEnvVarRepo
	AppEnvRepo    *repo.EnvVarRepo
	DatabaseRepo  *repo.DatabaseRepo
	DeployWorker  *worker.DeployWorker
	DockerPool    *docker.ClientPool
	DockerClient  *docker.Client
	EnvResolver   *deploy.EnvResolver
}

// MCPServer adalah wrapper Model Context Protocol server untuk get-labuh.
type MCPServer struct {
	internalServer *server.MCPServer
	deps           ServerDeps
}

// NewServer menginisialisasi MCPServer get-labuh dan mendaftarkan seluruh 15 core operational tools.
func NewServer(deps ServerDeps) *MCPServer {
	s := server.NewMCPServer("get-labuh", "1.0.0", server.WithToolCapabilities(true), server.WithLogging())

	mcpSrv := &MCPServer{
		internalServer: s,
		deps:           deps,
	}

	mcpSrv.registerAppTools()
	mcpSrv.registerEnvTools()
	mcpSrv.registerDatabaseTools()
	mcpSrv.registerServerTools()

	return mcpSrv
}

// GetInternalServer mengembalikan instance dasar *server.MCPServer.
func (s *MCPServer) GetInternalServer() *server.MCPServer {
	return s.internalServer
}

func (s *MCPServer) getDockerClient(ctx context.Context, serverID *string) *docker.Client {
	if s.deps.DockerPool != nil {
		c, err := s.deps.DockerPool.GetClient(ctx, serverID)
		if err == nil && c != nil {
			return c
		}
	}
	return s.deps.DockerClient
}
