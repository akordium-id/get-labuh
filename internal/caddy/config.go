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
		Match: []RouteMatch{
			{Host: []string{domain}},
		},
		Handle: []RouteHandle{
			{
				Handler: "reverse_proxy",
				Upstreams: []Upstream{
					{Dial: fmt.Sprintf("%s:%d", upstreamAddr, port)},
				},
			},
		},
	}

	return json.MarshalIndent(route, "", "  ")
}

func GenerateServerConfig(domain, upstreamAddr string, port int) ([]byte, error) {
	config := map[string]interface{}{
		"apps": map[string]interface{}{
			"http": map[string]interface{}{
				"servers": map[string]interface{}{
					"srv0": map[string]interface{}{
						"routes": []Route{
							{
								Match: []RouteMatch{
									{Host: []string{domain}},
								},
								Handle: []RouteHandle{
									{
										Handler: "reverse_proxy",
										Upstreams: []Upstream{
											{Dial: fmt.Sprintf("%s:%d", upstreamAddr, port)},
										},
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
