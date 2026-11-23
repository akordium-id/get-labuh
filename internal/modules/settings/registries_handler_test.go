package settings_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	stdtesting "testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/encryption"
	"github.com/akordium-id/get-labuh/internal/modules/settings"
)

func TestRegistriesHandler(t *stdtesting.T) {
	os.Setenv("LABUH_MASTER_KEY", "test-master-key-for-registries-32b")
	err := encryption.Init("LABUH_MASTER_KEY")
	require.NoError(t, err)

	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	t.Cleanup(func() { _ = db.Close() })

	mod := settings.New(db, t.TempDir())

	r := chi.NewRouter()
	mod.RegisterRoutes(r)

	// 1. Test POST /settings/registries (Create)
	form := url.Values{}
	form.Set("name", "GitHub Packages")
	form.Set("server_address", "ghcr.io")
	form.Set("username", "test-user")
	form.Set("password", "ghp_mocktoken123456")
	form.Set("email", "test@example.com")

	reqCreate := httptest.NewRequest(http.MethodPost, "/settings/registries", strings.NewReader(form.Encode()))
	reqCreate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wCreate := httptest.NewRecorder()
	r.ServeHTTP(wCreate, reqCreate)
	assert.Equal(t, http.StatusSeeOther, wCreate.Code)

	// Verify in DB
	list, err := mod.RegistryRepo.List(context.Background())
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "GitHub Packages", list[0].Name)
	assert.Equal(t, "ghcr.io", list[0].ServerAddress)

	// 2. Test POST /settings/registries/test (Verify credentials)
	reqTest := httptest.NewRequest(http.MethodPost, "/settings/registries/test", strings.NewReader(form.Encode()))
	reqTest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wTest := httptest.NewRecorder()
	r.ServeHTTP(wTest, reqTest)
	// Even without live docker daemon, should handle gracefully
	assert.True(t, wTest.Code == http.StatusOK || wTest.Code == http.StatusBadRequest)

	// 3. Test POST /settings/registries/{id}/delete
	reqDel := httptest.NewRequest(http.MethodPost, "/settings/registries/"+list[0].ID+"/delete", nil)
	wDel := httptest.NewRecorder()
	r.ServeHTTP(wDel, reqDel)
	assert.Equal(t, http.StatusSeeOther, wDel.Code)

	// Verify deleted from DB
	listAfter, err := mod.RegistryRepo.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, listAfter)
}
