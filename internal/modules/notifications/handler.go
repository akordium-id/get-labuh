package notifications

import (
	"context"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/settings"
)

type ProjectProvider interface {
	GetAll() ([]*models.Project, error)
}

type Handler struct {
	repo            *Repo
	projectProvider ProjectProvider
	notifier        *Notifier
}

func NewHandler(
	repo *Repo,
	projectProvider ProjectProvider,
	notifier *Notifier,
) *Handler {
	return &Handler{
		repo:            repo,
		projectProvider: projectProvider,
		notifier:        notifier,
	}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	channels, err := h.repo.ListGlobal()
	if err != nil {
		http.Error(w, "Failed to load notification channels", http.StatusInternalServerError)
		return
	}

	projects, _ := h.projectProvider.GetAll()
	for _, p := range projects {
		if pChannels, err := h.repo.ListByProjectID(p.ID); err == nil {
			channels = append(channels, pChannels...)
		}
	}

	templ.Handler(layouts.AppLayout(settings.NotificationsPage(channels, projects))).ServeHTTP(w, r)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	chType := models.NotificationType(r.FormValue("type"))
	projectIDVal := r.FormValue("project_id")
	discordURLVal := r.FormValue("discord_webhook_url")
	botTokenVal := r.FormValue("telegram_bot_token")
	chatIDVal := r.FormValue("telegram_chat_id")

	var projectID *string
	if projectIDVal != "" {
		projectID = &projectIDVal
	}
	var discordURL *string
	if discordURLVal != "" {
		discordURL = &discordURLVal
	}
	var botToken *string
	if botTokenVal != "" {
		botToken = &botTokenVal
	}
	var chatID *string
	if chatIDVal != "" {
		chatID = &chatIDVal
	}

	input := models.CreateNotificationChannelInput{
		Name:              name,
		Type:              chType,
		ProjectID:         projectID,
		Enabled:           true,
		DiscordWebhookURL: discordURL,
		TelegramBotToken:  botToken,
		TelegramChatID:    chatID,
		OnDeploySuccess:   r.FormValue("on_deploy_success") == "true",
		OnDeployFailure:   r.FormValue("on_deploy_failure") == "true",
		OnBackupSuccess:   r.FormValue("on_backup_success") == "true",
		OnBackupFailure:   r.FormValue("on_backup_failure") == "true",
	}

	if _, err := h.repo.Create(input); err != nil {
		http.Error(w, "Failed to create channel: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings/notifications", http.StatusFound)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := h.repo.Delete(id); err != nil {
		http.Error(w, "Failed to delete channel", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings/notifications", http.StatusFound)
}

func (h *Handler) Test(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	ch, err := h.repo.GetByID(id)
	if err != nil {
		http.Error(w, "Channel not found", http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	testEvent := models.NotificationEvent{
		Title:       "Uji Coba Notifikasi Labuh",
		Description: "Koneksi webhook notification channel berhasil terhubung ke Labuh PaaS!",
		Status:      "success",
		Fields: map[string]string{
			"Channel Name": ch.Name,
			"Channel Type": string(ch.Type),
			"Status":       "Verified & Active",
		},
	}

	if err := h.notifier.SendToChannel(ctx, ch, testEvent); err != nil {
		http.Error(w, "Test notification failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	http.Redirect(w, r, "/settings/notifications", http.StatusFound)
}
