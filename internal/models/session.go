package models

import (
	"time"
)

type Session struct {
	ID        string
	UserID    string
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
}

type CreateSessionInput struct {
	UserID    string
	TokenHash string
	ExpiresAt time.Time
}
