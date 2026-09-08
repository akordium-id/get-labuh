package models

import "time"

type ApiKey struct {
	ID          string
	UserID      string
	Name        string
	KeyHash     string
	LastUsedAt  *time.Time
	CreatedAt   time.Time
}

type CreateApiKeyInput struct {
	UserID  string
	Name    string
	KeyHash string
}
