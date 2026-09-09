package handler

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/settings"
)

type SettingsHandler struct {
	settingRepo *repo.SettingRepo
}

func NewSettingsHandler(settingRepo *repo.SettingRepo) *SettingsHandler {
	return &SettingsHandler{
		settingRepo: settingRepo,
	}
}

func (h *SettingsHandler) Caddy(w http.ResponseWriter, r *http.Request) {
	caddyAPIURL, _ := h.settingRepo.Get("caddy_api_url")
	caddyAPIKey, _ := h.settingRepo.Get("caddy_api_key")
	caddyNetwork, _ := h.settingRepo.Get("caddy_network")

	templ.Handler(layouts.AppLayout(settings.CaddySettingsPage(caddyAPIURL, caddyAPIKey, caddyNetwork))).ServeHTTP(w, r)
}

func (h *SettingsHandler) UpdateCaddy(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	caddyAPIURL := r.FormValue("caddy_api_url")
	caddyAPIKey := r.FormValue("caddy_api_key")
	caddyNetwork := r.FormValue("caddy_network")

	if caddyAPIURL == "" {
		caddyAPIURL = "http://localhost:2019"
	}

	_ = h.settingRepo.Set("caddy_api_url", caddyAPIURL)
	_ = h.settingRepo.Set("caddy_api_key", caddyAPIKey)
	_ = h.settingRepo.Set("caddy_network", caddyNetwork)

	w.Header().Set("HX-Redirect", "/settings/caddy")
	w.WriteHeader(http.StatusOK)
}

func (h *SettingsHandler) TestCaddy(w http.ResponseWriter, r *http.Request) {
	caddyAPIURL, _ := h.settingRepo.Get("caddy_api_url")
	caddyAPIKey, _ := h.settingRepo.Get("caddy_api_key")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok","caddy_url":"` + caddyAPIURL + `"}`))
	_ = caddyAPIKey
}
