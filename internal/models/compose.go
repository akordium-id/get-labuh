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

type ComposeServiceDef struct {
	Name        string            `json:"name"`
	Image       string            `json:"image,omitempty"`
	BuildPath   string            `json:"build_path,omitempty"`
	Ports       []string          `json:"ports,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Volumes     []string          `json:"volumes,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty"`
}

type ComposeServiceRuntime struct {
	Name        string            `json:"name"`
	Definition  ComposeServiceDef `json:"definition"`
	ContainerID string            `json:"container_id,omitempty"`
	Status      string            `json:"status"` // "running", "exited", "restarting", "not_created"
	Health      string            `json:"health,omitempty"` // "healthy", "unhealthy", "starting"
	State       string            `json:"state,omitempty"`
	CreatedAt   string            `json:"created_at,omitempty"`
}

type ComposeApplication struct {
	ID                 string
	EnvironmentID      string
	Name               string
	Slug               string
	ComposeFilePath    string
	ComposeProjectName *string
	CustomDomain       *string
	RawYAML            string
	ParsedServices     []ComposeServiceDef
	LastDeployedAt     *time.Time
	ErrorMessage       *string
	Status             ComposeStatus
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type CreateComposeApplicationInput struct {
	EnvironmentID      string
	Name               string
	Slug               string
	ComposeFilePath    string
	ComposeProjectName *string
	CustomDomain       *string
	RawYAML            string
}

type ComposeDeployJob struct {
	ComposeID          string
	EnvironmentID      string
	ComposeFilePath    string
	ComposeProjectName string
	LogPath            string
}
