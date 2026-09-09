package repo

import (
	"database/sql"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
	"github.com/google/uuid"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(input models.CreateUserInput) (*models.User, error) {
	user := &models.User{
		ID:           uuid.New().String(),
		Email:        input.Email,
		PasswordHash: input.PasswordHash,
		Name:         input.Name,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err := r.db.Exec(
		"INSERT INTO users (id, email, password_hash, name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		user.ID, user.Email, user.PasswordHash, user.Name, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepo) GetByEmail(email string) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(
		"SELECT id, email, password_hash, name, failed_login_count, locked_until, last_login_at, created_at, updated_at FROM users WHERE email = ?",
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.FailedLoginCount, &user.LockedUntil, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepo) GetByID(id string) (*models.User, error) {
	user := &models.User{}
	err := r.db.QueryRow(
		"SELECT id, email, password_hash, name, failed_login_count, locked_until, last_login_at, created_at, updated_at FROM users WHERE id = ?",
		id,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.FailedLoginCount, &user.LockedUntil, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepo) GetAll() ([]*models.User, error) {
	rows, err := r.db.Query(
		"SELECT id, email, password_hash, name, failed_login_count, locked_until, last_login_at, created_at, updated_at FROM users ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		err := rows.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Name, &user.FailedLoginCount, &user.LockedUntil, &user.LastLoginAt, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

func (r *UserRepo) IncrementFailedLogin(id string) error {
	_, err := r.db.Exec(
		"UPDATE users SET failed_login_count = failed_login_count + 1, updated_at = ? WHERE id = ?",
		time.Now(), id,
	)
	return err
}

func (r *UserRepo) ResetFailedLogin(id string) error {
	now := time.Now()
	_, err := r.db.Exec(
		"UPDATE users SET failed_login_count = 0, locked_until = NULL, last_login_at = ?, updated_at = ? WHERE id = ?",
		now, now, id,
	)
	return err
}

func (r *UserRepo) LockUser(id string, until time.Time) error {
	_, err := r.db.Exec(
		"UPDATE users SET locked_until = ?, updated_at = ? WHERE id = ?",
		until, time.Now(), id,
	)
	return err
}

func (r *UserRepo) CountActiveSessions(userID string) (int, error) {
	var count int
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM sessions WHERE user_id = ? AND expires_at > ?",
		userID, time.Now(),
	).Scan(&count)
	return count, err
}
