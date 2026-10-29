package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	stdtesting "testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/akordium-id/get-labuh/internal/database"
	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/akordium-id/get-labuh/internal/modules/auth"
)

func setupAuthModule(t *stdtesting.T) *auth.Module {
	t.Helper()
	db, err := database.Connect("file::memory:?cache=shared")
	require.NoError(t, err)
	require.NoError(t, database.RunAllMigrations(db))
	t.Cleanup(func() { _ = db.Close() })

	return auth.New(db, false)
}

func TestAuthModule_LoginPage(t *stdtesting.T) {
	mod := setupAuthModule(t)

	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	w := httptest.NewRecorder()

	mod.Handler.LoginPage(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestAuthModule_Login_InvalidCredentials(t *stdtesting.T) {
	mod := setupAuthModule(t)
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	_, err := mod.UserRepo.Create(ctx, models.CreateUserInput{
		Email:        "test@example.com",
		PasswordHash: string(hash),
		Name:         "Test",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.PostForm = map[string][]string{
		"email":    {"test@example.com"},
		"password": {"wrong"},
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	mod.Handler.Login(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAuthModule_Login_Success(t *stdtesting.T) {
	mod := setupAuthModule(t)
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	_, err := mod.UserRepo.Create(ctx, models.CreateUserInput{
		Email:        "test@example.com",
		PasswordHash: string(hash),
		Name:         "Test",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/auth/login", nil)
	req.PostForm = map[string][]string{
		"email":    {"test@example.com"},
		"password": {"password"},
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	mod.Handler.Login(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	cookies := resp.Cookies()
	assert.Len(t, cookies, 1)
	assert.Equal(t, "session", cookies[0].Name)
}

func TestAuthModule_Logout(t *stdtesting.T) {
	mod := setupAuthModule(t)
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.DefaultCost)
	user, err := mod.UserRepo.Create(ctx, models.CreateUserInput{
		Email:        "test@example.com",
		PasswordHash: string(hash),
		Name:         "Test",
	})
	require.NoError(t, err)

	_, err = mod.SessionRepo.Create(ctx, models.CreateSessionInput{
		UserID:    user.ID,
		TokenHash: auth.HashToken("token"),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "token"})
	w := httptest.NewRecorder()

	mod.Handler.Logout(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusFound, resp.StatusCode)
	assert.Equal(t, "/auth/login", resp.Header.Get("Location"))
}

func TestAuthModule_RegisterRoutes(t *stdtesting.T) {
	mod := setupAuthModule(t)
	r := chi.NewRouter()

	mod.RegisterPublicRoutes(r)
	r.Group(func(protected chi.Router) {
		protected.Use(mod.RequireAuth)
		mod.RegisterProtectedRoutes(protected)
	})

	// Test login endpoint route
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
