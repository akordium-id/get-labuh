package caddy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultTimeout = 5 * time.Second

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}
}

func (c *Client) AddRoute(domain, upstreamAddr string, port int) error {
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}

	route := Route{
		Match: []Match{
			{Host: []string{domain}},
		},
		Handle: []Handle{
			{
				Handler: "reverse_proxy",
				Upstreams: []Upstream{
					{Dial: fmt.Sprintf("%s:%d", upstreamAddr, port)},
				},
			},
		},
	}

	body, err := json.Marshal(route)
	if err != nil {
		return fmt.Errorf("failed to marshal route: %w", err)
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", c.baseURL+"/config/apps/http/servers/srv0/routes", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("X-Caddy-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to add route: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("caddy API returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) RemoveRoute(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain cannot be empty")
	}

	req, err := http.NewRequestWithContext(context.Background(), "DELETE", c.baseURL+"/config/apps/http/servers/srv0/routes/host/"+domain, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("X-Caddy-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to remove route: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("caddy API returned status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) RouteExists(domain string) (bool, error) {
	if domain == "" {
		return false, fmt.Errorf("domain cannot be empty")
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", c.baseURL+"/config/apps/http/servers/srv0/routes", nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("X-Caddy-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to list routes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("caddy API returned status %d", resp.StatusCode)
	}

	var routes []Route
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		return false, fmt.Errorf("failed to decode routes: %w", err)
	}

	for _, route := range routes {
		for _, match := range route.Match {
			for _, host := range match.Host {
				if host == domain {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

func (c *Client) ListRoutes() ([]Route, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", c.baseURL+"/config/apps/http/servers/srv0/routes", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if c.apiKey != "" {
		req.Header.Set("X-Caddy-API-Key", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list routes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("caddy API returned status %d", resp.StatusCode)
	}

	var routes []Route
	if err := json.NewDecoder(resp.Body).Decode(&routes); err != nil {
		return nil, fmt.Errorf("failed to decode routes: %w", err)
	}

	return routes, nil
}
