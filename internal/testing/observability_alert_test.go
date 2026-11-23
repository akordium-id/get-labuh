package testing

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	stdtesting "testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/modules/notifications"
	"github.com/akordium-id/get-labuh/internal/modules/settings"
	"github.com/akordium-id/get-labuh/internal/observability"
)

func TestIntegration_Observability_ProactiveAlertAndWebhook(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	defer db.Close()

	// 1. Mock External Webhook Server (Slack / PagerDuty / OpsGenie)
	var mu sync.Mutex
	receivedEvents := make([]map[string]any, 0)
	var lastAuthHeader string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		lastAuthHeader = r.Header.Get("Authorization")
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		receivedEvents = append(receivedEvents, body)

		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	// 2. Setup Notification Channel
	notifRepo := notifications.NewRepo(db)
	webhookURL := mockServer.URL
	webhookSecret := "sec-ops-token-789"

	channel, err := notifRepo.Create(models.CreateNotificationChannelInput{
		Type:          models.NotificationTypeWebhook,
		Name:          "PagerDuty Webhook",
		Enabled:       true,
		WebhookURL:    &webhookURL,
		WebhookSecret: &webhookSecret,
		OnServerAlert: true,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, channel.ID)

	// 3. Setup Settings and Monitor
	settingRepo := settings.NewSettingRepo(db)
	ctx := context.Background()
	_ = settingRepo.Set(ctx, "host_monitoring_enabled", "true")
	_ = settingRepo.Set(ctx, "host_disk_warning_pct", "80")
	_ = settingRepo.Set(ctx, "host_disk_critical_pct", "90")
	_ = settingRepo.Set(ctx, "host_ram_warning_pct", "85")
	_ = settingRepo.Set(ctx, "host_alert_cooldown_minutes", "30")

	notifier := notifications.NewNotifier(notifRepo)
	monitor := observability.NewHostMonitor(settingRepo, notifier, "/")

	// 4. Trigger Warning Alert (Disk 82%)
	statsWarning := observability.HostStats{
		DiskTotal:       100 * 1024 * 1024 * 1024,
		DiskUsed:        82 * 1024 * 1024 * 1024,
		DiskFree:        18 * 1024 * 1024 * 1024,
		DiskUsedPercent: 82.0,
		MemTotal:        16 * 1024 * 1024 * 1024,
		MemUsedPercent:  45.0,
	}
	monitor.Evaluate(ctx, statsWarning)

	// Wait briefly for async Dispatch goroutine
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(receivedEvents) == 1
	}, 2*time.Second, 50*time.Millisecond)

	mu.Lock()
	assert.Equal(t, "Bearer sec-ops-token-789", lastAuthHeader)
	assert.Equal(t, models.EventServerAlertWarning, receivedEvents[0]["event"])
	assert.Equal(t, "warning", receivedEvents[0]["status"])
	mu.Unlock()

	// 5. Duplicate condition within cooldown (Disk stays 82%) -> should be suppressed
	monitor.Evaluate(ctx, statsWarning)
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	assert.Len(t, receivedEvents, 1, "Duplicate alert within cooldown window must be suppressed")
	mu.Unlock()

	// 6. Escalation to Critical (Disk 93%) -> should trigger immediately
	statsCritical := statsWarning
	statsCritical.DiskUsedPercent = 93.0
	monitor.Evaluate(ctx, statsCritical)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(receivedEvents) == 2
	}, 2*time.Second, 50*time.Millisecond)

	mu.Lock()
	assert.Equal(t, models.EventServerAlertCritical, receivedEvents[1]["event"])
	assert.Equal(t, "failed", receivedEvents[1]["status"])
	mu.Unlock()

	// 7. Recovery / Resolved (Disk drops to 55%)
	statsResolved := statsWarning
	statsResolved.DiskUsedPercent = 55.0
	monitor.Evaluate(ctx, statsResolved)

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(receivedEvents) == 3
	}, 2*time.Second, 50*time.Millisecond)

	mu.Lock()
	assert.Equal(t, models.EventServerAlertResolved, receivedEvents[2]["event"])
	assert.Equal(t, "success", receivedEvents[2]["status"])
	mu.Unlock()
}
