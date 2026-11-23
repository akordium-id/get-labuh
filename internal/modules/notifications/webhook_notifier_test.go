package notifications

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	stdtesting "testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/models"
)

func TestRepo_WebhookChannel_CRUD(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	defer db.Close()

	r := NewRepo(db)

	webhookURL := "https://alert.example.com/webhook"
	webhookSecret := "whsec_supersecret123"

	ch, err := r.Create(models.CreateNotificationChannelInput{
		Type:          models.NotificationTypeWebhook,
		Name:          "Company Slack/OpsGenie Bridge",
		Enabled:       true,
		WebhookURL:    &webhookURL,
		WebhookSecret: &webhookSecret,
		OnServerAlert: true,
	})
	require.NoError(t, err)
	require.NotEmpty(t, ch.ID)
	assert.Equal(t, models.NotificationTypeWebhook, ch.Type)
	require.NotNil(t, ch.WebhookURL)
	assert.Equal(t, "https://alert.example.com/webhook", *ch.WebhookURL)
	require.NotNil(t, ch.WebhookSecret)
	assert.Equal(t, "whsec_supersecret123", *ch.WebhookSecret)
	assert.True(t, ch.OnServerAlert)

	// Test ListActiveForEvent with server_alert
	active, err := r.ListActiveForEvent("server_alert", nil)
	require.NoError(t, err)
	require.Len(t, active, 1)
	assert.Equal(t, ch.ID, active[0].ID)

	// Also supports EventServerAlertWarning
	activeWarn, err := r.ListActiveForEvent(models.EventServerAlertWarning, nil)
	require.NoError(t, err)
	require.Len(t, activeWarn, 1)

	// ListGlobal
	globals, err := r.ListGlobal()
	require.NoError(t, err)
	require.Len(t, globals, 1)
	assert.Equal(t, ch.ID, globals[0].ID)
}

func TestNotifier_SendWebhook(t *stdtesting.T) {
	var receivedBody map[string]any
	var receivedAuthHeader string
	var receivedSecretHeader string

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthHeader = r.Header.Get("Authorization")
		receivedSecretHeader = r.Header.Get("X-Webhook-Secret")
		_ = json.NewDecoder(r.Body).Decode(&receivedBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	n := NewNotifier(nil)
	webhookURL := mockServer.URL
	webhookSecret := "my-secret-key-456"

	ch := &models.NotificationChannel{
		Type:          models.NotificationTypeWebhook,
		Name:          "Test Webhook",
		Enabled:       true,
		WebhookURL:    &webhookURL,
		WebhookSecret: &webhookSecret,
		OnServerAlert: true,
	}

	event := models.NotificationEvent{
		Title:       "High Disk Usage Warning",
		Description: "Disk usage on / has reached 85.5%",
		Status:      "warning",
		EventType:   models.EventServerAlertWarning,
		Fields: map[string]string{
			"Disk Used": "85.5%",
			"Host":      "srv-prod-1",
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := n.SendToChannel(ctx, ch, event)
	require.NoError(t, err)

	assert.Equal(t, "Bearer my-secret-key-456", receivedAuthHeader)
	assert.Equal(t, "my-secret-key-456", receivedSecretHeader)
	assert.Equal(t, "High Disk Usage Warning", receivedBody["title"])
	assert.Equal(t, "warning", receivedBody["status"])
	assert.Equal(t, models.EventServerAlertWarning, receivedBody["event"])
}
