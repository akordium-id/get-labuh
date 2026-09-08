package models

import "time"

type TeamRole string

const (
	TeamRoleOwner   TeamRole = "owner"
	TeamRoleAdmin   TeamRole = "admin"
	TeamRoleDeveloper TeamRole = "developer"
	TeamRoleViewer  TeamRole = "viewer"
)

type Team struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type CreateTeamInput struct {
	Name string
}

type TeamMember struct {
	ID        string
	TeamID    string
	UserID    string
	Role      TeamRole
	CreatedAt time.Time
}

type CreateTeamMemberInput struct {
	TeamID string
	UserID string
	Role   TeamRole
}
