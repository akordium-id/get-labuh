package mcp

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/mark3labs/mcp-go/server"
)

// MountHTTP mendaftarkan SSE transport dan message endpoint pada router Chi dengan proteksi autentikasi.
func (s *MCPServer) MountHTTP(r chi.Router, authMiddleware func(http.Handler) http.Handler) {
	sseServer := server.NewSSEServer(s.internalServer, server.WithBasePath("/mcp"))

	r.Group(func(r chi.Router) {
		if authMiddleware != nil {
			r.Use(authMiddleware)
		}
		r.Handle("/mcp/*", sseServer)
		r.Handle("/mcp", sseServer)
	})
}
