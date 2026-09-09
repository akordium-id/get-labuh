package models

import "time"

type AuditLog struct {
	ID         string
	UserID     *string
	Action     string
	Resource   string
	ResourceID string
	IPAddress  string
	UserAgent  string
	CreatedAt  time.Time
}

type CreateAuditLogInput struct {
	UserID     *string
	Action     string
	Resource   string
	ResourceID string
	IPAddress  string
	UserAgent  string
}
