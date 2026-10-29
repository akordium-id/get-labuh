package settings_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	stdtesting "testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/modules/settings"
)

func TestSettingsModule_CRUD(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	t.Cleanup(func() { _ = db.Close() })

	mod := settings.New(db, t.TempDir())

	r := chi.NewRouter()
	mod.RegisterRoutes(r)

	// 1. Test Caddy Settings
	formCaddy := url.Values{}
	formCaddy.Set("caddy_api_url", "http://127.0.0.1:2019")
	formCaddy.Set("caddy_network", "custom-net")

	reqCaddy := httptest.NewRequest(http.MethodPost, "/settings/caddy", strings.NewReader(formCaddy.Encode()))
	reqCaddy.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wCaddy := httptest.NewRecorder()
	r.ServeHTTP(wCaddy, reqCaddy)
	assert.Equal(t, http.StatusOK, wCaddy.Code)

	val, err := mod.SettingRepo.Get(context.Background(), "caddy_api_url")
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:2019", val)

	// 2. Test Mode Switcher
	formMode := url.Values{}
	formMode.Set("mode", "lite")

	reqMode := httptest.NewRequest(http.MethodPost, "/settings/mode", strings.NewReader(formMode.Encode()))
	reqMode.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wMode := httptest.NewRecorder()
	r.ServeHTTP(wMode, reqMode)
	assert.Equal(t, http.StatusOK, wMode.Code)

	cookies := wMode.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Equal(t, "labuh_mode", cookies[0].Name)
	assert.Equal(t, "lite", cookies[0].Value)

	// 3. Test Storage Destination CRUD
	formStorage := url.Values{}
	formStorage.Set("name", "Cloudflare R2 Primary")
	formStorage.Set("endpoint", "https://r2.cloudflarestorage.com")
	formStorage.Set("bucket", "r2-backups")
	formStorage.Set("region", "auto")
	formStorage.Set("access_key_id", "r2-key")
	formStorage.Set("secret_access_key", "r2-secret")
	formStorage.Set("prefix", "databases")
	formStorage.Set("is_default", "true")

	reqStorage := httptest.NewRequest(http.MethodPost, "/settings/storage-destinations", strings.NewReader(formStorage.Encode()))
	reqStorage.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wStorage := httptest.NewRecorder()
	r.ServeHTTP(wStorage, reqStorage)
	assert.Equal(t, http.StatusFound, wStorage.Code)

	list, err := mod.StorageRepo.List(context.Background())
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Cloudflare R2 Primary", list[0].Name)
	assert.True(t, list[0].IsDefault)
}
