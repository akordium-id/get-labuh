package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
)

type Notifier struct {
	repo       *Repo
	httpClient *http.Client
}

func NewNotifier(repo *Repo) *Notifier {
	return &Notifier{
		repo:       repo,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Dispatch mengirim notifikasi ke semua channel yang relevan secara asinkron
func (n *Notifier) Dispatch(event models.NotificationEvent) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		channels, err := n.repo.ListActiveForEvent(event.EventType, event.ProjectID)
		if err != nil {
			slog.Error("failed to list channels for event", "event", event.EventType, "error", err)
			return
		}

		for _, ch := range channels {
			if err := n.SendToChannel(ctx, ch, event); err != nil {
				slog.Error("failed to send notification", "channel", ch.Name, "type", ch.Type, "error", err)
			}
		}
	}()
}

func (n *Notifier) SendToChannel(ctx context.Context, ch *models.NotificationChannel, event models.NotificationEvent) error {
	switch ch.Type {
	case models.NotificationTypeDiscord:
		if ch.DiscordWebhookURL == nil || *ch.DiscordWebhookURL == "" {
			return fmt.Errorf("missing discord webhook url")
		}
		return n.sendDiscord(ctx, *ch.DiscordWebhookURL, event)

	case models.NotificationTypeTelegram:
		if ch.TelegramBotToken == nil || *ch.TelegramBotToken == "" || ch.TelegramChatID == nil || *ch.TelegramChatID == "" {
			return fmt.Errorf("missing telegram bot token or chat id")
		}
		return n.sendTelegram(ctx, *ch.TelegramBotToken, *ch.TelegramChatID, event)

	default:
		return fmt.Errorf("unsupported notification channel type: %s", ch.Type)
	}
}

func (n *Notifier) sendDiscord(ctx context.Context, webhookURL string, event models.NotificationEvent) error {
	var color int
	switch event.Status {
	case "success":
		color = 0x22c55e // Green
	case "failed", "error":
		color = 0xef4444 // Red
	default:
		color = 0x3b82f6 // Blue
	}

	type discordField struct {
		Name   string `json:"name"`
		Value  string `json:"value"`
		Inline bool   `json:"inline"`
	}

	fields := make([]discordField, 0, len(event.Fields))
	for k, v := range event.Fields {
		fields = append(fields, discordField{
			Name:   k,
			Value:  v,
			Inline: true,
		})
	}

	payload := map[string]any{
		"username": "Labuh PaaS",
		"embeds": []map[string]any{
			{
				"title":       event.Title,
				"description": event.Description,
				"color":       color,
				"fields":      fields,
				"timestamp":   time.Now().UTC().Format(time.RFC3339),
				"footer": map[string]string{
					"text": "Labuh Deployment Engine",
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord returned non-2xx status: %d", resp.StatusCode)
	}

	return nil
}

func (n *Notifier) sendTelegram(ctx context.Context, botToken, chatID string, event models.NotificationEvent) error {
	var statusIcon string
	switch event.Status {
	case "success":
		statusIcon = "✅"
	case "failed", "error":
		statusIcon = "❌"
	default:
		statusIcon = "ℹ️"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s <b>%s</b>\n\n", statusIcon, event.Title)
	fmt.Fprintf(&sb, "%s\n\n", event.Description)

	for k, v := range event.Fields {
		fmt.Fprintf(&sb, "<b>%s:</b> <code>%s</code>\n", k, v)
	}
	fmt.Fprintf(&sb, "\n<i>Labuh Deployment Engine • %s</i>", time.Now().Format("2006-01-02 15:04:05"))

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := map[string]string{
		"chat_id":    chatID,
		"text":       sb.String(),
		"parse_mode": "HTML",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram returned non-2xx status: %d", resp.StatusCode)
	}

	return nil
}
