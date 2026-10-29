package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
)

// SessionProvider mendefinisikan interface pencarian sesi
type SessionProvider interface {
	FindByToken(ctx context.Context, tokenHash string) (*models.Session, error)
}

// UserProvider mendefinisikan interface pencarian pengguna
type UserProvider interface {
	GetByID(ctx context.Context, id string) (*models.User, error)
}

// RequireAuth menghasilkan middleware HTTP untuk validasi cookie sesi
func RequireAuth(sessionRepo SessionProvider, userRepo UserProvider, secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil {
				http.Redirect(w, r, "/auth/login", http.StatusFound)
				return
			}

			tokenHash := HashToken(cookie.Value)
			session, err := sessionRepo.FindByToken(r.Context(), tokenHash)
			if err != nil || session.ExpiresAt.Before(time.Now()) {
				http.Redirect(w, r, "/auth/login", http.StatusFound)
				return
			}

			user, err := userRepo.GetByID(r.Context(), session.UserID)
			if err != nil {
				http.Redirect(w, r, "/auth/login", http.StatusFound)
				return
			}

			if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
				http.Redirect(w, r, "/auth/login?locked=1", http.StatusFound)
				return
			}

			ctx := ContextWithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
