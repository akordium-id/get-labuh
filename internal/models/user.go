package models

import (
	"time"
)

type User struct {
	ID                string
	Email             string
	PasswordHash      string
	Name              string
	FailedLoginCount  int
	LockedUntil       *time.Time
	LastLoginAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type CreateUserInput struct {
	Email        string
	PasswordHash string
	Name         string
}
