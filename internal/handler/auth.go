package handler

import (
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
	"github.com/a-h/templ"

	"github.com/akordium-id/get-labuh/internal/auth"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/pages"
)

type AuthHandler struct {
	userRepo    *repo.UserRepo
	sessionRepo *repo.SessionRepo
	secure      bool
}

func NewAuthHandler(userRepo *repo.UserRepo, sessionRepo *repo.SessionRepo, secure bool) *AuthHandler {
	return &AuthHandler{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		secure:      secure,
	}
}

func (h *AuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.LoginPage(w, r)
	case http.MethodPost:
		h.Login(w, r)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func (h *AuthHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	templ.Handler(pages.LoginPage()).ServeHTTP(w, r)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	user, err := h.userRepo.GetByEmail(email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		if user != nil && user.ID != "" {
			h.userRepo.IncrementFailedLogin(user.ID)
			if user.FailedLoginCount+1 >= auth.MaxFailedLogins {
				lockedUntil := time.Now().Add(auth.LockoutDuration)
				h.userRepo.LockUser(user.ID, lockedUntil)
			}
		}

		w.Header().Set("HX-Retarget", "#login-error")
		w.Header().Set("HX-Reswap", "innerHTML")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("<div class=\"text-red-600 text-sm\">Invalid email or password</div>"))
		return
	}

	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		w.Header().Set("HX-Retarget", "#login-error")
		w.Header().Set("HX-Reswap", "innerHTML")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("<div class=\"text-red-600 text-sm\">Account is locked. Please try again later.</div>"))
		return
	}

	_ = h.userRepo.ResetFailedLogin(user.ID)

	token, err := auth.GenerateSessionToken()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	tokenHash := auth.HashToken(token)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	_ = h.sessionRepo.EvictOldestSessions(user.ID, auth.MaxConcurrentSessions-1)

	_, err = h.sessionRepo.Create(models.CreateSessionInput{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	cookie := auth.GenerateSessionCookie(token, h.secure)
	http.SetCookie(w, cookie)

	w.Header().Set("HX-Redirect", "/dashboard")
	w.WriteHeader(http.StatusOK)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		tokenHash := auth.HashToken(cookie.Value)
		session, err := h.sessionRepo.FindByToken(tokenHash)
		if err == nil {
			h.sessionRepo.Delete(session.ID)
		}
	}

	clearCookie := auth.ClearSessionCookie(h.secure)
	http.SetCookie(w, clearCookie)

	http.Redirect(w, r, "/auth/login", http.StatusFound)
}
