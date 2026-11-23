package models_test

import (
	stdtesting "testing"

	"github.com/stretchr/testify/assert"

	"github.com/akordium-id/get-labuh/internal/models"
)

func TestNotificationChannel_WebhookType(t *stdtesting.T) {
	webhookURL := "https://webhook.site/test-uuid"
	webhookSecret := "secret-token-123"

	ch := models.NotificationChannel{
		ID:            "ch-1",
		Type:          models.NotificationTypeWebhook,
		Name:          "Slack Alert Hook",
		Enabled:       true,
		WebhookURL:    &webhookURL,
		WebhookSecret: &webhookSecret,
		OnServerAlert: true,
	}

	assert.Equal(t, models.NotificationTypeWebhook, ch.Type)
	assert.Equal(t, "https://webhook.site/test-uuid", *ch.WebhookURL)
	assert.Equal(t, "secret-token-123", *ch.WebhookSecret)
	assert.True(t, ch.OnServerAlert)
	assert.Equal(t, "server_resource_warning", models.EventServerAlertWarning)
	assert.Equal(t, "server_resource_critical", models.EventServerAlertCritical)
	assert.Equal(t, "server_resource_resolved", models.EventServerAlertResolved)
}
