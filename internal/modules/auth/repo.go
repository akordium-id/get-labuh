package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

type contextKey string

const userContextKey contextKey = "user"

const (
	MaxFailedLogins       = 5
	LockoutDuration       = 15 * time.Minute
	MaxConcurrentSessions = 3
)

func ContextWithUser(ctx context.Context, user *models.User) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}

func UserFromContext(ctx context.Context) (*models.User, bool) {
	user, ok := ctx.Value(userContextKey).(*models.User)
	return user, ok
}

func ExtractIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

func ExtractUserAgent(r *http.Request) string {
	return r.Header.Get("User-Agent")
}

func GenerateSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func GenerateSessionCookie(token string, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
	}
}

func ClearSessionCookie(secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		MaxAge:   -1,
	}
}

// UserRepo mengelola persistensi data pengguna
type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(ctx context.Context, input models.CreateUserInput) (*models.User, error) {
	user := &models.User{
		ID:           uuid.New().String(),
		Email:        input.Email,
		PasswordHash: input.PasswordHash,
		Name:         input.Name,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err := r.db.ExecContext(ctx,
		"INSERT INTO users (id, email, password_hash, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		user.ID, user.Email, user.PasswordHash, user.Name, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRowContext(ctx,
		"SELECT id, email, password_hash, name, failed_login_count, locked_until, last_login_at, created_at, updated_at FROM users WHERE email = ?",
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.FailedLoginCount, &user.LockedUntil, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRowContext(ctx,
		"SELECT id, email, password_hash, name, failed_login_count, locked_until, last_login_at, created_at, updated_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.FailedLoginCount, &user.LockedUntil, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepo) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func (r *UserRepo) IncrementFailedLogin(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET failed_login_count = failed_login_count + 1, updated_at = ? WHERE id = ?", time.Now(), id)
	return err
}

func (r *UserRepo) ResetFailedLogin(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET failed_login_count = 0, locked_until = NULL, updated_at = ? WHERE id = ?", time.Now(), id)
	return err
}

func (r *UserRepo) LockUser(ctx context.Context, id string, lockedUntil time.Time) error {
	_, err := r.db.ExecContext(ctx, "UPDATE users SET locked_until = ?, updated_at = ? WHERE id = ?", lockedUntil, time.Now(), id)
	return err
}

func (r *UserRepo) UpdateLastLogin(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, "UPDATE users SET last_login_at = ?, updated_at = ? WHERE id = ?", now, now, id)
	return err
}

// SessionRepo mengelola persistensi session
type SessionRepo struct {
	db *sql.DB
}

func NewSessionRepo(db *sql.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

func (r *SessionRepo) Create(ctx context.Context, input models.CreateSessionInput) (*models.Session, error) {
	session := &models.Session{
		ID:        uuid.New().String(),
		UserID:    input.UserID,
		TokenHash: input.TokenHash,
		ExpiresAt: input.ExpiresAt,
		CreatedAt: time.Now(),
	}

	_, err := r.db.ExecContext(ctx,
		"INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?)",
		session.ID, session.UserID, session.TokenHash, session.ExpiresAt, session.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (r *SessionRepo) FindByToken(ctx context.Context, tokenHash string) (*models.Session, error) {
	session := &models.Session{}
	err := r.db.QueryRowContext(ctx,
		"SELECT id, user_id, token_hash, expires_at, created_at FROM sessions WHERE token_hash = ?",
		tokenHash,
	).Scan(&session.ID, &session.UserID, &session.TokenHash, &session.ExpiresAt, &session.CreatedAt)
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (r *SessionRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", id)
	return err
}

func (r *SessionRepo) EvictOldestSessions(ctx context.Context, userID string, keepCount int) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM sessions
		WHERE user_id = ? AND id NOT IN (
			SELECT id FROM sessions
			WHERE user_id = ?
			ORDER BY created_at DESC
			LIMIT ?
		)
	`, userID, userID, keepCount)
	return err
}

// APIKeyRepo mengelola persistensi API Keys
type APIKeyRepo struct {
	db *sql.DB
}

func NewAPIKeyRepo(db *sql.DB) *APIKeyRepo {
	return &APIKeyRepo{db: db}
}

func (r *APIKeyRepo) Create(ctx context.Context, input models.CreateApiKeyInput) (*models.ApiKey, error) {
	apiKey := &models.ApiKey{
		ID:        uuid.New().String(),
		UserID:    input.UserID,
		Name:      input.Name,
		KeyHash:   input.KeyHash,
		CreatedAt: time.Now(),
	}

	_, err := r.db.ExecContext(ctx,
		"INSERT INTO api_keys (id, user_id, name, key_hash, created_at) VALUES (?, ?, ?, ?, ?)",
		apiKey.ID, apiKey.UserID, apiKey.Name, apiKey.KeyHash, apiKey.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return apiKey, nil
}

func (r *APIKeyRepo) GetByID(ctx context.Context, id string) (*models.ApiKey, error) {
	apiKey := &models.ApiKey{}
	err := r.db.QueryRowContext(ctx,
		"SELECT id, user_id, name, key_hash, last_used_at, created_at FROM api_keys WHERE id = ?",
		id,
	).Scan(&apiKey.ID, &apiKey.UserID, &apiKey.Name, &apiKey.KeyHash, &apiKey.LastUsedAt, &apiKey.CreatedAt)
	if err != nil {
		return nil, err
	}

	return apiKey, nil
}

func (r *APIKeyRepo) GetByKeyHash(ctx context.Context, keyHash string) (*models.ApiKey, error) {
	apiKey := &models.ApiKey{}
	err := r.db.QueryRowContext(ctx,
		"SELECT id, user_id, name, key_hash, last_used_at, created_at FROM api_keys WHERE key_hash = ?",
		keyHash,
	).Scan(&apiKey.ID, &apiKey.UserID, &apiKey.Name, &apiKey.KeyHash, &apiKey.LastUsedAt, &apiKey.CreatedAt)
	if err != nil {
		return nil, err
	}

	return apiKey, nil
}

func (r *APIKeyRepo) GetByUserID(ctx context.Context, userID string) ([]*models.ApiKey, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, user_id, name, key_hash, last_used_at, created_at FROM api_keys WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var apiKeys []*models.ApiKey
	for rows.Next() {
		apiKey := &models.ApiKey{}
		err := rows.Scan(&apiKey.ID, &apiKey.UserID, &apiKey.Name, &apiKey.KeyHash, &apiKey.LastUsedAt, &apiKey.CreatedAt)
		if err != nil {
			return nil, err
		}
		apiKeys = append(apiKeys, apiKey)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return apiKeys, nil
}

func (r *APIKeyRepo) UpdateLastUsed(ctx context.Context, id string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, "UPDATE api_keys SET last_used_at = ? WHERE id = ?", now, id)
	return err
}

func (r *APIKeyRepo) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM api_keys WHERE id = ?", id)
	return err
}
