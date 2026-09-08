package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/faiq/labuh/internal/database/repo"
	"github.com/faiq/labuh/internal/models"
)

type contextKey string

const userContextKey contextKey = "user"

func ContextWithUser(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) (*models.User, bool) {
	user, ok := ctx.Value(userContextKey).(*models.User)
	return user, ok
}

func RequireAuth(sessionRepo *repo.SessionRepo, userRepo *repo.UserRepo, secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session")
			if err != nil {
				http.Redirect(w, r, "/auth/login", http.StatusFound)
				return
			}

			tokenHash := HashToken(cookie.Value)
			session, err := sessionRepo.FindByToken(tokenHash)
			if err != nil || session.ExpiresAt.Before(time.Now()) {
				http.Redirect(w, r, "/auth/login", http.StatusFound)
				return
			}

			user, err := userRepo.GetByID(session.UserID)
			if err != nil {
				http.Redirect(w, r, "/auth/login", http.StatusFound)
				return
			}

			ctx := ContextWithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
