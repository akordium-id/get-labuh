package mcp_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"

	labuhmcp "github.com/akordium-id/get-labuh/internal/mcp"
	"github.com/akordium-id/get-labuh/internal/middleware"
)

func dummyAuthMiddleware(validKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("Authorization")
			if token != "Bearer "+validKey {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func TestMCPTransportHTTP_AuthRejectionAndAllow(t *testing.T) {
	mcpServer := labuhmcp.NewServer(labuhmcp.ServerDeps{})

	r := chi.NewRouter()
	mcpServer.MountHTTP(r, dummyAuthMiddleware("secret-api-key"))

	// 1. Unauthorized GET request to /mcp/sse
	reqUnauth := httptest.NewRequest(http.MethodGet, "/mcp/sse", nil)
	wUnauth := httptest.NewRecorder()
	r.ServeHTTP(wUnauth, reqUnauth)
	assert.Equal(t, http.StatusUnauthorized, wUnauth.Code)

	// 2. Unauthorized POST request to /mcp/message
	reqPostUnauth := httptest.NewRequest(http.MethodPost, "/mcp/message", nil)
	wPostUnauth := httptest.NewRecorder()
	r.ServeHTTP(wPostUnauth, reqPostUnauth)
	assert.Equal(t, http.StatusUnauthorized, wPostUnauth.Code)
}

func TestMCPTransportHTTP_CSRFBypass(t *testing.T) {
	mcpServer := labuhmcp.NewServer(labuhmcp.ServerDeps{})

	r := chi.NewRouter()
	r.Use(middleware.CSRFProtectionMiddleware)
	mcpServer.MountHTTP(r, dummyAuthMiddleware("secret-api-key"))

	// POST to /mcp/message with auth header and no CSRF cookie/header
	// It must NOT be rejected by CSRFProtectionMiddleware (which would return 403)
	req := httptest.NewRequest(http.MethodPost, "/mcp/message", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer secret-api-key")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.NotEqual(t, http.StatusForbidden, w.Code)
}
