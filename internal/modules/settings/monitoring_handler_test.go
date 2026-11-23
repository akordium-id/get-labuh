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
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/modules/settings"
)

type mockNotifier struct {
	events []models.NotificationEvent
}

func (m *mockNotifier) Dispatch(event models.NotificationEvent) {
	m.events = append(m.events, event)
}

func TestMonitoringHandler(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	t.Cleanup(func() { _ = db.Close() })

	mod := settings.New(db, t.TempDir())
	notifier := &mockNotifier{}
	mod.SetNotifier(notifier)

	r := chi.NewRouter()
	mod.RegisterRoutes(r)

	// 1. GET /settings/monitoring
	reqGet := httptest.NewRequest(http.MethodGet, "/settings/monitoring", nil)
	wGet := httptest.NewRecorder()
	r.ServeHTTP(wGet, reqGet)
	assert.Equal(t, http.StatusOK, wGet.Code)

	// 2. POST /settings/monitoring (Update settings)
	form := url.Values{
		"host_monitoring_enabled":     {"true"},
		"host_disk_warning_pct":       {"75"},
		"host_disk_critical_pct":      {"88"},
		"host_ram_warning_pct":        {"85"},
		"host_alert_cooldown_minutes": {"45"},
	}
	reqPost := httptest.NewRequest(http.MethodPost, "/settings/monitoring", strings.NewReader(form.Encode()))
	reqPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wPost := httptest.NewRecorder()
	r.ServeHTTP(wPost, reqPost)
	assert.Equal(t, http.StatusSeeOther, wPost.Code)

	// Verify in DB
	diskWarn, err := mod.SettingRepo.Get(context.Background(), "host_disk_warning_pct")
	require.NoError(t, err)
	assert.Equal(t, "75", diskWarn)

	diskCrit, err := mod.SettingRepo.Get(context.Background(), "host_disk_critical_pct")
	require.NoError(t, err)
	assert.Equal(t, "88", diskCrit)

	ramWarn, err := mod.SettingRepo.Get(context.Background(), "host_ram_warning_pct")
	require.NoError(t, err)
	assert.Equal(t, "85", ramWarn)

	cooldown, err := mod.SettingRepo.Get(context.Background(), "host_alert_cooldown_minutes")
	require.NoError(t, err)
	assert.Equal(t, "45", cooldown)

	// 3. POST /settings/monitoring/test (Trigger manual test alert)
	reqTest := httptest.NewRequest(http.MethodPost, "/settings/monitoring/test", nil)
	wTest := httptest.NewRecorder()
	r.ServeHTTP(wTest, reqTest)
	assert.Equal(t, http.StatusOK, wTest.Code)

	require.Len(t, notifier.events, 1)
	assert.Equal(t, models.EventServerAlertWarning, notifier.events[0].EventType)
	assert.Contains(t, notifier.events[0].Title, "Test Server Resource Alert")
}
