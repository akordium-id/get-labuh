package models

import "time"

type SourceType string

const (
	SourceTypeGit         SourceType = "git"
	SourceTypeDockerImage SourceType = "docker_image"
	SourceTypeDockerfile  SourceType = "dockerfile"
)

type AppStatus string

const (
	AppStatusIdle     AppStatus = "idle"
	AppStatusBuilding AppStatus = "building"
	AppStatusRunning  AppStatus = "running"
	AppStatusStopped  AppStatus = "stopped"
	AppStatusFailed   AppStatus = "failed"
)

type Application struct {
	ID              string
	EnvironmentID   string
	Name            string
	Slug            string
	SourceType      SourceType
	RepositoryURL   *string
	Branch          *string
	BuildPath       *string
	DockerfilePath  *string
	DockerImage     *string
	CustomDomain    *string
	AppPort         int
	ContainerID     *string
	ContainerName   *string
	Status          AppStatus
	WebhookSecret   *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateApplicationInput struct {
	EnvironmentID  string
	Name           string
	Slug           string
	SourceType     SourceType
	RepositoryURL  *string
	Branch         *string
	BuildPath      *string
	DockerfilePath *string
	DockerImage    *string
	CustomDomain   *string
	AppPort        int
	WebhookSecret  *string
}

type AppEnvVar struct {
	ID           string
	ApplicationID string
	Key          string
	Value        string
	IsSecret     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreateAppEnvVarInput struct {
	ApplicationID string
	Key           string
	Value         string
	IsSecret      bool
}

type DeployStatus string

const (
	DeployStatusQueued    DeployStatus = "queued"
	DeployStatusCloning   DeployStatus = "cloning"
	DeployStatusBuilding  DeployStatus = "building"
	DeployStatusDeploying DeployStatus = "deploying"
	DeployStatusSuccess   DeployStatus = "success"
	DeployStatusFailed    DeployStatus = "failed"
	DeployStatusCancelled DeployStatus = "cancelled"
)

type Deployment struct {
	ID             string
	ApplicationID  string
	CommitHash     *string
	CommitMessage  *string
	Status         DeployStatus
	ErrorMessage   *string
	LogPath        *string
	StartedAt      *time.Time
	FinishedAt     *time.Time
	CreatedAt      time.Time
}

type CreateDeploymentInput struct {
	ApplicationID  string
	CommitHash     *string
	CommitMessage  *string
	LogPath        *string
}


