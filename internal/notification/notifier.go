package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
)

type Notifier struct {
	repo       *repo.NotificationRepo
	httpClient *http.Client
}

func NewNotifier(repo *repo.NotificationRepo) *Notifier {
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
		return fmt.Errorf("unsupported channel type: %s", ch.Type)
	}
}

// Discord Webhook Payload
type discordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type discordEmbed struct {
	Title       string              `json:"title"`
	Description string              `json:"description"`
	Color       int                 `json:"color"`
	Fields      []discordEmbedField `json:"fields,omitempty"`
	Timestamp   string              `json:"timestamp"`
	Footer      struct {
		Text string `json:"text"`
	} `json:"footer"`
}

type discordPayload struct {
	Username  string         `json:"username"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Embeds    []discordEmbed `json:"embeds"`
}

func (n *Notifier) sendDiscord(ctx context.Context, webhookURL string, event models.NotificationEvent) error {
	color := 0x22c55e // green
	if event.Status == "failure" {
		color = 0xef4444 // red
	}

	var fields []discordEmbedField
	for k, v := range event.Fields {
		fields = append(fields, discordEmbedField{
			Name:   k,
			Value:  v,
			Inline: true,
		})
	}

	embed := discordEmbed{
		Title:       event.Title,
		Description: event.Description,
		Color:       color,
		Fields:      fields,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}
	embed.Footer.Text = "Labuh PaaS Notification"

	payload := discordPayload{
		Username: "Labuh",
		Embeds:   []discordEmbed{embed},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
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
		return fmt.Errorf("discord returned status %d", resp.StatusCode)
	}
	return nil
}

// Telegram Bot API Payload
type telegramPayload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

func (n *Notifier) sendTelegram(ctx context.Context, botToken, chatID string, event models.NotificationEvent) error {
	statusEmoji := "✅"
	if event.Status == "failure" {
		statusEmoji = "❌"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s <b>%s</b>\n\n", statusEmoji, event.Title)
	if event.Description != "" {
		fmt.Fprintf(&sb, "%s\n\n", event.Description)
	}

	if len(event.Fields) > 0 {
		for k, v := range event.Fields {
			fmt.Fprintf(&sb, "• <b>%s:</b> <code>%s</code>\n", k, v)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("<i>Labuh Platform Notification</i>")

	payload := telegramPayload{
		ChatID:    chatID,
		Text:      sb.String(),
		ParseMode: "HTML",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
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
		return fmt.Errorf("telegram returned status %d", resp.StatusCode)
	}
	return nil
}
