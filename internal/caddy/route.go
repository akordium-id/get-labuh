package caddy

type Route struct {
	ID     string   `json:"id,omitempty"`
	Match  []Match  `json:"match"`
	Handle []Handle `json:"handle"`
}

type Match struct {
	Host []string `json:"host"`
}

type Handle struct {
	Handler   string     `json:"handler"`
	Upstreams []Upstream `json:"upstreams"`
}

type Upstream struct {
	Dial string `json:"dial"`
}
