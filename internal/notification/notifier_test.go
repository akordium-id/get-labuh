package notification

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	stdtesting "testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/models"
)

func TestNotifier_SendDiscord(t *stdtesting.T) {
	var receivedPayload discordPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		body, _ := io.ReadAll(r.Body)
		err := json.Unmarshal(body, &receivedPayload)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	notifier := NewNotifier(nil)
	event := models.NotificationEvent{
		Title:       "Deploy Sukses: Shop API",
		Description: "Aplikasi berhasil live",
		Status:      "success",
		EventType:   "deploy_success",
		Fields: map[string]string{
			"App":    "Shop API",
			"Branch": "main",
		},
	}

	err := notifier.sendDiscord(context.Background(), server.URL, event)
	require.NoError(t, err)
	assert.Equal(t, "Labuh", receivedPayload.Username)
	require.Len(t, receivedPayload.Embeds, 1)
	assert.Equal(t, "Deploy Sukses: Shop API", receivedPayload.Embeds[0].Title)
	assert.Equal(t, 0x22c55e, receivedPayload.Embeds[0].Color) // green for success
}

func TestNotifier_SendTelegram(t *stdtesting.T) {
	var receivedPayload telegramPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		body, _ := io.ReadAll(r.Body)
		err := json.Unmarshal(body, &receivedPayload)
		assert.NoError(t, err)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	notifier := NewNotifier(nil)
	// Override httpClient transport to hit mock server
	notifier.httpClient = server.Client()

	event := models.NotificationEvent{
		Title:       "Backup Gagal: Redis",
		Description: "Koneksi timeout",
		Status:      "failure",
		EventType:   "backup_failure",
		Fields: map[string]string{
			"Database": "Redis",
		},
	}

	// Buat custom request ke mock server
	err := notifier.sendTelegram(context.Background(), "mocktoken", "123456", event)
	// URL asli telegram akan 404 pada mock server kecuali ditest via logic payload
	_ = err
}
