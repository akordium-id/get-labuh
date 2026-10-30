package auth

import (
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/web/layouts"
	"github.com/akordium-id/get-labuh/internal/web/pages"
	"github.com/akordium-id/get-labuh/internal/web/pages/settings"
)

type Handler struct {
	userRepo    *UserRepo
	sessionRepo *SessionRepo
	apiKeyRepo  *APIKeyRepo
	secure      bool
}

func NewHandler(userRepo *UserRepo, sessionRepo *SessionRepo, apiKeyRepo *APIKeyRepo, secure bool) *Handler {
	return &Handler{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		apiKeyRepo:  apiKeyRepo,
		secure:      secure,
	}
}

func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	// Jika belum ada user terdaftar sama sekali, redirect otomatis ke halaman onboarding register admin
	count, err := h.userRepo.Count(r.Context())
	if err == nil && count == 0 {
		http.Redirect(w, r, "/auth/register", http.StatusFound)
		return
	}
	templ.Handler(pages.LoginPage()).ServeHTTP(w, r)
}

func (h *Handler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	// Halaman register hanya boleh diakses saat belum ada user (first user setup)
	count, err := h.userRepo.Count(r.Context())
	if err == nil && count > 0 {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return
	}
	templ.Handler(pages.RegisterPage()).ServeHTTP(w, r)
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	count, err := h.userRepo.Count(ctx)
	if err == nil && count > 0 {
		w.Header().Set("HX-Retarget", "#register-error")
		w.Header().Set("HX-Reswap", "innerHTML")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<div class=\"bg-red-500/10 border border-red-500/30 text-red-400 text-xs p-3 rounded-lg\">Registrasi ditutup. Administrator sudah terdaftar.</div>"))
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	email := r.FormValue("email")
	password := r.FormValue("password")
	passwordConfirm := r.FormValue("password_confirm")

	if name == "" || email == "" || password == "" {
		w.Header().Set("HX-Retarget", "#register-error")
		w.Header().Set("HX-Reswap", "innerHTML")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("<div class=\"bg-red-500/10 border border-red-500/30 text-red-400 text-xs p-3 rounded-lg\">Semua kolom wajib diisi.</div>"))
		return
	}

	if len(password) < 8 {
		w.Header().Set("HX-Retarget", "#register-error")
		w.Header().Set("HX-Reswap", "innerHTML")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("<div class=\"bg-red-500/10 border border-red-500/30 text-red-400 text-xs p-3 rounded-lg\">Password minimal 8 karakter.</div>"))
		return
	}

	if password != passwordConfirm {
		w.Header().Set("HX-Retarget", "#register-error")
		w.Header().Set("HX-Reswap", "innerHTML")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("<div class=\"bg-red-500/10 border border-red-500/30 text-red-400 text-xs p-3 rounded-lg\">Konfirmasi password tidak cocok.</div>"))
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	user, err := h.userRepo.Create(ctx, models.CreateUserInput{
		Email:        email,
		PasswordHash: string(hashedPassword),
		Name:         name,
	})
	if err != nil {
		w.Header().Set("HX-Retarget", "#register-error")
		w.Header().Set("HX-Reswap", "innerHTML")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("<div class=\"bg-red-500/10 border border-red-500/30 text-red-400 text-xs p-3 rounded-lg\">Gagal mendaftarkan user: " + err.Error() + "</div>"))
		return
	}

	// Login otomatis setelah registrasi berhasil
	token, err := GenerateSessionToken()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	tokenHash := HashToken(token)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	_, err = h.sessionRepo.Create(ctx, models.CreateSessionInput{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	cookie := GenerateSessionCookie(token, h.secure)
	http.SetCookie(w, cookie)

	w.Header().Set("HX-Redirect", "/dashboard")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	ctx := r.Context()
	user, err := h.userRepo.GetByEmail(ctx, email)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		if user != nil && user.ID != "" {
			_ = h.userRepo.IncrementFailedLogin(ctx, user.ID)
			if user.FailedLoginCount+1 >= MaxFailedLogins {
				lockedUntil := time.Now().Add(LockoutDuration)
				_ = h.userRepo.LockUser(ctx, user.ID, lockedUntil)
			}
		}

		w.Header().Set("HX-Retarget", "#login-error")
		w.Header().Set("HX-Reswap", "innerHTML")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("<div class=\"text-red-600 text-sm\">Invalid email or password</div>"))
		return
	}

	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		w.Header().Set("HX-Retarget", "#login-error")
		w.Header().Set("HX-Reswap", "innerHTML")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("<div class=\"text-red-600 text-sm\">Account is locked. Please try again later.</div>"))
		return
	}

	_ = h.userRepo.ResetFailedLogin(ctx, user.ID)

	token, err := GenerateSessionToken()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	tokenHash := HashToken(token)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	_ = h.sessionRepo.EvictOldestSessions(ctx, user.ID, MaxConcurrentSessions-1)

	_, err = h.sessionRepo.Create(ctx, models.CreateSessionInput{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	cookie := GenerateSessionCookie(token, h.secure)
	http.SetCookie(w, cookie)

	w.Header().Set("HX-Redirect", "/dashboard")
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		tokenHash := HashToken(cookie.Value)
		session, err := h.sessionRepo.FindByToken(r.Context(), tokenHash)
		if err == nil {
			_ = h.sessionRepo.Delete(r.Context(), session.ID)
		}
	}

	clearCookie := ClearSessionCookie(h.secure)
	http.SetCookie(w, clearCookie)

	http.Redirect(w, r, "/auth/login", http.StatusFound)
}

// API Keys handlers
func (h *Handler) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return
	}

	apiKeys, err := h.apiKeyRepo.GetByUserID(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	templ.Handler(layouts.AppLayout(settings.APIKeysPage(apiKeys))).ServeHTTP(w, r)
}

func (h *Handler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	user, _ := UserFromContext(r.Context())
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

	rawKey, err := GenerateSessionToken()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	keyHash := HashToken(rawKey)

	apiKey, err := h.apiKeyRepo.Create(r.Context(), models.CreateApiKeyInput{
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

func (h *Handler) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	if err := h.apiKeyRepo.Delete(r.Context(), id); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/settings/api-keys")
	w.WriteHeader(http.StatusOK)
}
