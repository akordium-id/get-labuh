package models

import "time"

type ComposeStatus string

const (
	ComposeStatusIdle     ComposeStatus = "idle"
	ComposeStatusBuilding ComposeStatus = "building"
	ComposeStatusRunning  ComposeStatus = "running"
	ComposeStatusStopped  ComposeStatus = "stopped"
	ComposeStatusFailed   ComposeStatus = "failed"
)

type ComposeApplication struct {
	ID                  string
	EnvironmentID       string
	Name                string
	Slug                string
	ComposeFilePath     string
	ComposeProjectName  *string
	CustomDomain        *string
	Status              ComposeStatus
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type CreateComposeApplicationInput struct {
	EnvironmentID       string
	Name                string
	Slug                string
	ComposeFilePath     string
	ComposeProjectName  *string
	CustomDomain        *string
}

type ComposeDeployJob struct {
	ComposeID           string
	EnvironmentID       string
	ComposeFilePath     string
	ComposeProjectName  string
	LogPath             string
}
