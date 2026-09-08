package models

import (
	"time"
)

type Project struct {
	ID          string
	Name        string
	Slug        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CreateProjectInput struct {
	Name        string
	Slug        string
	Description *string
}

type Environment struct {
	ID        string
	ProjectID string
	Name      string
	Slug      string
	CreatedAt time.Time
	UpdatedAt time.Time
}
