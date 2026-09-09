package caddy

import (
	"encoding/json"
	"fmt"
)

func GenerateRouteConfig(domain, upstreamAddr string, port int) ([]byte, error) {
	if domain == "" {
		return nil, fmt.Errorf("domain cannot be empty")
	}

	route := Route{
		Match: []Match{
			{Host: []string{domain}},
		},
		Handle: []Handle{
			{
				Handler:   "reverse_proxy",
				Upstreams: []Upstream{{Dial: fmt.Sprintf("%s:%d", upstreamAddr, port)}},
			},
		},
	}

	return json.MarshalIndent(route, "", "  ")
}

func GenerateServerConfig(domain, upstreamAddr string, port int) ([]byte, error) {
	config := map[string]any{
		"apps": map[string]any{
			"http": map[string]any{
				"servers": map[string]any{
					"srv0": map[string]any{
						"routes": []Route{
							{
								Match: []Match{
									{Host: []string{domain}},
								},
								Handle: []Handle{
									{
										Handler:   "reverse_proxy",
										Upstreams: []Upstream{{Dial: fmt.Sprintf("%s:%d", upstreamAddr, port)}},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	return json.MarshalIndent(config, "", "  ")
}
