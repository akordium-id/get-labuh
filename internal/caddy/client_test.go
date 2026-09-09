package caddy

import (
	stdtesting "testing"
	"net/http"
	"net/http/httptest"
	"encoding/json"

	"github.com/stretchr/testify/assert"
)

func TestClient_AddRoute(t *stdtesting.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	err := client.AddRoute("example.com", "127.0.0.1", 8080)
	assert.NoError(t, err)
}

func TestClient_AddRoute_EmptyDomain(t *stdtesting.T) {
	client := NewClient("http://localhost:2019", "")
	err := client.AddRoute("", "127.0.0.1", 8080)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "domain cannot be empty")
}

func TestClient_AddRoute_ServerError(t *stdtesting.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	err := client.AddRoute("example.com", "127.0.0.1", 8080)
	assert.Error(t, err)
}

func TestClient_RemoveRoute(t *stdtesting.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	err := client.RemoveRoute("example.com")
	assert.NoError(t, err)
}

func TestClient_RemoveRoute_EmptyDomain(t *stdtesting.T) {
	client := NewClient("http://localhost:2019", "")
	err := client.RemoveRoute("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "domain cannot be empty")
}

func TestClient_RouteExists(t *stdtesting.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routes := []Route{
			{Match: []Match{{Host: []string{"example.com"}}},
			 Handle: []Handle{{Handler: "reverse_proxy"}}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(routes)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	exists, err := client.RouteExists("example.com")
	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestClient_RouteExists_NotFound(t *stdtesting.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routes := []Route{}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(routes)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	exists, err := client.RouteExists("nonexistent.com")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestClient_RouteExists_EmptyDomain(t *stdtesting.T) {
	client := NewClient("http://localhost:2019", "")
	exists, err := client.RouteExists("")
	assert.Error(t, err)
	assert.False(t, exists)
}

func TestClient_ListRoutes(t *stdtesting.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		routes := []Route{
			{Match: []Match{{Host: []string{"example.com"}}},
			 Handle: []Handle{{Handler: "reverse_proxy"}}},
			{Match: []Match{{Host: []string{"test.com"}}},
			 Handle: []Handle{{Handler: "reverse_proxy"}}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(routes)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	routes, err := client.ListRoutes()
	assert.NoError(t, err)
	assert.Len(t, routes, 2)
}

func TestClient_APIKeyAuth(t *stdtesting.T) {
	var receivedKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedKey = r.Header.Get("X-Caddy-API-Key")
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL, "my-api-key")
	err := client.AddRoute("example.com", "127.0.0.1", 8080)
	assert.NoError(t, err)
	assert.Equal(t, "my-api-key", receivedKey)
}

func TestRouteStructure(t *stdtesting.T) {
	route := Route{
		Match:  []Match{{Host: []string{"example.com"}}},
		Handle: []Handle{{Handler: "reverse_proxy", Upstreams: []Upstream{{Dial: "127.0.0.1:8080"}}}},
	}

	data, err := json.Marshal(route)
	assert.NoError(t, err)

	var parsed Route
	err = json.Unmarshal(data, &parsed)
	assert.NoError(t, err)
	assert.Len(t, parsed.Match, 1)
	assert.Len(t, parsed.Match[0].Host, 1)
	assert.Equal(t, "example.com", parsed.Match[0].Host[0])
}
