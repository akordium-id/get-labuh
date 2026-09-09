package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
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

const MaxFailedLogins = 5
const LockoutDuration = 15 * time.Minute
const MaxConcurrentSessions = 3

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

			if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
				http.Redirect(w, r, "/auth/login?locked=1", http.StatusFound)
				return
			}

		activeSessions, _ := userRepo.CountActiveSessions(user.ID)
		if activeSessions > MaxConcurrentSessions {
			http.Redirect(w, r, "/auth/login?too_many_sessions=1", http.StatusFound)
			return
		}

			ctx := ContextWithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(teamRepo *repo.TeamRepo, roles ...models.TeamRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok || user == nil {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			teamID := chi.URLParam(r, "team_id")
			if teamID == "" {
				next.ServeHTTP(w, r)
				return
			}

			member, err := teamRepo.GetMember(teamID, user.ID)
			if err != nil || member == nil {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}

			for _, role := range roles {
				if member.Role == role || repo.HasRole(member.Role, role) {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "Forbidden", http.StatusForbidden)
		})
	}
}
