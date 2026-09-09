package handler

import (
	"net/http"
	"net/http/httptest"
	stdtesting "testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/stretchr/testify/assert"
)

func setupAuthHandler(t *stdtesting.T) (*AuthHandler, *repo.UserRepo, *repo.SessionRepo) {
	t.Helper()
	db, err := database.Connect("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("failed to connect to test database: %v", err)
	}
	if err := database.RunAllMigrations(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	userRepo := repo.NewUserRepo(db)
	sessionRepo := repo.NewSessionRepo(db)

	return NewAuthHandler(userRepo, sessionRepo, false), userRepo, sessionRepo
}

func TestAuthHandler_LoginPage(t *stdtesting.T) {
	h, _, _ := setupAuthHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	w := httptest.NewRecorder()

	h.LoginPage(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAuthHandler_Login_InvalidCredentials(t *stdtesting.T) {
	h, userRepo, _ := setupAuthHandler(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	userRepo.Create(models.CreateUserInput{
		Email:        "test@example.com",
		PasswordHash: string(hash),
		Name:         "Test",
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.PostForm = map[string][]string{
		"email":    {"test@example.com"},
		"password": {"wrong"},
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.Login(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuthHandler_Login_Success(t *stdtesting.T) {
	h, userRepo, _ := setupAuthHandler(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	userRepo.Create(models.CreateUserInput{
		Email:        "test@example.com",
		PasswordHash: string(hash),
		Name:         "Test",
	})

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.PostForm = map[string][]string{
		"email":    {"test@example.com"},
		"password": {"password"},
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	h.Login(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	cookies := resp.Cookies()
	assert.Len(t, cookies, 1)
	assert.Equal(t, "session", cookies[0].Name)
}

func TestAuthHandler_Logout(t *stdtesting.T) {
	h, userRepo, sessionRepo := setupAuthHandler(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	user, _ := userRepo.Create(models.CreateUserInput{
		Email:        "test@example.com",
		PasswordHash: string(hash),
		Name:         "Test",
	})
	sessionRepo.Create(models.CreateSessionInput{
		UserID:    user.ID,
		TokenHash: "tokenhash",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "token"})
	w := httptest.NewRecorder()

	h.Logout(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "/auth/login", resp.Header.Get("Location"))
}
