package repo

import (
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

type SessionRepo struct {
	db *sql.DB
}

func NewSessionRepo(db *sql.DB) *SessionRepo {
	return &SessionRepo{db: db}
}

func (r *SessionRepo) Create(input models.CreateSessionInput) (*models.Session, error) {
	session := &models.Session{
		ID:        uuid.New().String(),
		UserID:    input.UserID,
		TokenHash: input.TokenHash,
		ExpiresAt: input.ExpiresAt,
		CreatedAt: time.Now(),
	}

	_, err := r.db.Exec(
		"INSERT INTO sessions (id, user_id, token_hash, expires_at, created_at) VALUES (?, ?, ?, ?, ?)",
		session.ID, session.UserID, session.TokenHash, session.ExpiresAt, session.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (r *SessionRepo) FindByToken(tokenHash string) (*models.Session, error) {
	session := &models.Session{}
	err := r.db.QueryRow(
		"SELECT id, user_id, token_hash, expires_at, created_at FROM sessions WHERE token_hash = ?",
		tokenHash,
	).Scan(&session.ID, &session.UserID, &session.TokenHash, &session.ExpiresAt, &session.CreatedAt)
	if err != nil {
		return nil, err
	}

	return session, nil
}

func (r *SessionRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM sessions WHERE id = ?", id)
	return err
}

func (r *SessionRepo) EvictOldestSessions(userID string, keepCount int) error {
	_, err := r.db.Exec(`
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
