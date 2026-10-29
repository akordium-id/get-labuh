package middleware

import (
	"context"
	"net/http"
)

type contextKey string

const UserModeContextKey contextKey = "user_mode"

const (
	CookieModeName = "labuh_mode"
	ModeLite       = "lite"
	ModePro        = "pro"
)

// ModeMiddleware memeriksa cookie "labuh_mode" dan menyuntikkan nilainya ke context
// Default mode adalah "lite" untuk keramahan pengguna baru / vibecoders
func ModeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mode := ModeLite
		if cookie, err := r.Cookie(CookieModeName); err == nil {
			if cookie.Value == ModePro {
				mode = ModePro
			}
		}

		ctx := context.WithValue(r.Context(), UserModeContextKey, mode)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserMode mengembalikan mode aktif ("lite" atau "pro") dari context request
func GetUserMode(ctx context.Context) string {
	if mode, ok := ctx.Value(UserModeContextKey).(string); ok && mode != "" {
		return mode
	}
	return ModeLite
}

// IsLiteMode mengembalikan true jika mode aktif adalah lite
func IsLiteMode(ctx context.Context) bool {
	return GetUserMode(ctx) == ModeLite
}
