package api

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/akordium-id/get-labuh/internal/database/repo"
)

type APIKeyAuthMiddleware struct {
	apiKeyRepo *repo.ApiKeyRepo
	userRepo   *repo.UserRepo
}

func NewAPIKeyAuthMiddleware(apiKeyRepo *repo.ApiKeyRepo, userRepo *repo.UserRepo) *APIKeyAuthMiddleware {
	return &APIKeyAuthMiddleware{
		apiKeyRepo: apiKeyRepo,
		userRepo:   userRepo,
	}
}

func (m *APIKeyAuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeAPIError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			writeAPIError(w, http.StatusUnauthorized, "invalid authorization header format")
			return
		}

		apiKey := parts[1]
		keyHash := hashAPIKey(apiKey)

		apiKeyRecord, err := m.apiKeyRepo.GetByKeyHash(keyHash)
		if err != nil || apiKeyRecord == nil {
			writeAPIError(w, http.StatusUnauthorized, "invalid api key")
			return
		}

		_ = m.apiKeyRepo.UpdateLastUsed(apiKeyRecord.ID)

		user, err := m.userRepo.GetByID(apiKeyRecord.UserID)
		if err != nil || user == nil {
			writeAPIError(w, http.StatusUnauthorized, "user not found")
			return
		}

		ctx := r.Context()
		_ = ctx
		next.ServeHTTP(w, r)
	})
}

func hashAPIKey(apiKey string) string {
	hash := sha256.Sum256([]byte(apiKey))
	return hex.EncodeToString(hash[:])
}

func writeAPIError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = message
}

type APIResponse struct {
	Data  interface{} `json:"data"`
	Error string      `json:"error,omitempty"`
}

func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = data
}
