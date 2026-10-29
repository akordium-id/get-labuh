package caddy

type Route struct {
	ID     string   `json:"@id,omitempty"`
	Match  []Match  `json:"match"`
	Handle []Handle `json:"handle"`
}

type Match struct {
	Host []string `json:"host"`
}

type Handle struct {
	Handler   string     `json:"handler"`
	Upstreams []Upstream `json:"upstreams,omitempty"`
	Providers *Providers `json:"providers,omitempty"`
}

type Providers struct {
	HTTPBasic *HTTPBasicProvider `json:"http_basic,omitempty"`
}

type HTTPBasicProvider struct {
	Accounts []BasicAuthAccount `json:"accounts"`
}

type BasicAuthAccount struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Upstream struct {
	Dial string `json:"dial"`
}

// RouteOptions menyimpan opsi konfigurasi tambahan untuk rute Caddy.
type RouteOptions struct {
	BasicAuthUser     string
	BasicAuthPassword string
}
