package handler

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/akordium-id/get-labuh/internal/auth"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages/settings"
)

type APIKeysHandler struct {
	apiKeyRepo *repo.ApiKeyRepo
	userRepo   *repo.UserRepo
}

func NewAPIKeysHandler(apiKeyRepo *repo.ApiKeyRepo, userRepo *repo.UserRepo) *APIKeysHandler {
	return &APIKeysHandler{
		apiKeyRepo: apiKeyRepo,
		userRepo:   userRepo,
	}
}

func (h *APIKeysHandler) List(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return
	}

	apiKeys, err := h.apiKeyRepo.GetByUserID(user.ID)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	templ.Handler(layouts.AppLayout(settings.APIKeysPage(apiKeys))).ServeHTTP(w, r)
}

func (h *APIKeysHandler) Create(w http.ResponseWriter, r *http.Request) {
	user, _ := auth.UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		name = "API Key"
	}

	rawKey, err := auth.GenerateSessionToken()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	keyHash := auth.HashToken(rawKey)

	apiKey, err := h.apiKeyRepo.Create(models.CreateApiKeyInput{
		UserID:  user.ID,
		Name:    name,
		KeyHash: keyHash,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	templ.Handler(layouts.AppLayout(settings.APIKeyCreatePage(apiKey, rawKey))).ServeHTTP(w, r)
}

func (h *APIKeysHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := h.apiKeyRepo.Delete(id); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/settings/api-keys")
	w.WriteHeader(http.StatusOK)
}
