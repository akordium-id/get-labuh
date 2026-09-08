package handler

import (
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
	"github.com/a-h/templ"

	"github.com/faiq/labuh/internal/auth"
	"github.com/faiq/labuh/internal/database/repo"
	"github.com/faiq/labuh/internal/models"
	"github.com/faiq/labuh/internal/web/pages"
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
		w.Header().Set("HX-Retarget", "#login-error")
		w.Header().Set("HX-Reswap", "innerHTML")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("<div class=\"text-red-600 text-sm\">Invalid email or password</div>"))
		return
	}

	token, err := auth.GenerateSessionToken()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	tokenHash := auth.HashToken(token)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

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
