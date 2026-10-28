package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	stdtesting "testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/notification"
)

func TestNotificationsHandler_CRUD_And_Filtering(t *stdtesting.T) {
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	t.Cleanup(func() { _ = db.Close() })

	projectRepo := repo.NewProjectRepo(db)
	notifRepo := repo.NewNotificationRepo(db)
	notifier := notification.NewNotifier(notifRepo)
	h := NewNotificationsHandler(notifRepo, projectRepo, notifier)

	proj, err := projectRepo.Create(models.CreateProjectInput{Name: "Client A Project", Slug: "client-a"})
	require.NoError(t, err)

	r := chi.NewRouter()
	r.Get("/settings/notifications", h.List)
	r.Post("/settings/notifications", h.Create)
	r.Post("/settings/notifications/{id}/delete", h.Delete)

	// 1. Create Global Discord Channel
	form := url.Values{}
	form.Set("name", "Global DevOps Discord")
	form.Set("type", "discord")
	form.Set("discord_webhook_url", "https://discord.com/api/webhooks/mock/global")
	form.Set("on_deploy_success", "true")
	form.Set("on_deploy_failure", "true")
	form.Set("on_backup_success", "false")
	form.Set("on_backup_failure", "true")

	req := httptest.NewRequest(http.MethodPost, "/settings/notifications", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusFound, w.Code)

	// 2. Create Project-Level Telegram Channel (Project Override)
	form2 := url.Values{}
	form2.Set("name", "Client A Telegram Alert")
	form2.Set("type", "telegram")
	form2.Set("project_id", proj.ID)
	form2.Set("telegram_bot_token", "123456:mocktoken")
	form2.Set("telegram_chat_id", "-100987654321")
	form2.Set("on_deploy_success", "true")
	form2.Set("on_deploy_failure", "true")

	req2 := httptest.NewRequest(http.MethodPost, "/settings/notifications", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusFound, w2.Code)

	// 3. Test Filtering (Opsi B: Global + Project Override)
	// Event Deploy Success untuk Project A: Harus dapat 2 channel (Global Discord + Project A Telegram)
	channelsForProjectA, err := notifRepo.ListActiveForEvent("deploy_success", &proj.ID)
	require.NoError(t, err)
	assert.Len(t, channelsForProjectA, 2)

	// Event Deploy Success untuk Project Lain (atau nil): Hanya dapat 1 channel (Global Discord)
	otherProjID := "some-other-project-id"
	channelsForOther, err := notifRepo.ListActiveForEvent("deploy_success", &otherProjID)
	require.NoError(t, err)
	assert.Len(t, channelsForOther, 1)
	assert.Equal(t, "Global DevOps Discord", channelsForOther[0].Name)

	// Event Backup Success: Global Discord disetel false, maka harus kosong
	backupSuccessChannels, err := notifRepo.ListActiveForEvent("backup_success", &proj.ID)
	require.NoError(t, err)
	assert.Empty(t, backupSuccessChannels)

	// 4. Test Delete Channel
	reqDelete := httptest.NewRequest(http.MethodPost, "/settings/notifications/"+channelsForProjectA[0].ID+"/delete", nil)
	wDelete := httptest.NewRecorder()
	r.ServeHTTP(wDelete, reqDelete)
	assert.Equal(t, http.StatusFound, wDelete.Code)
}
