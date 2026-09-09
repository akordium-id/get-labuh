package models

import "time"

type DeployStep string

const (
	DeployStepClone   DeployStep = "clone"
	DeployStepBuild   DeployStep = "build"
	DeployStepPush    DeployStep = "push"
	DeployStepDeploy  DeployStep = "deploy"
	DeployStepCleanup DeployStep = "cleanup"
)

type DeploymentJob struct {
	DeploymentID   string
	ApplicationID  string
	SourceType     string
	RepositoryURL  string
	Branch         string
	DockerfilePath string
	BuildPath      string
	DockerImage    string
	AppPort        int
	EnvVars        []AppEnvVar
	LogPath        string
	Priority       int
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
	Step           *string
	Output         *string
	ErrorMessage   *string
	LogPath        *string
	StartedAt      *time.Time
	FinishedAt     *time.Time
	Priority       int
	CreatedAt      time.Time
}

type CreateDeploymentInput struct {
	ApplicationID  string
	CommitHash     *string
	CommitMessage  *string
	LogPath        *string
	Step           *string
	Output         *string
}
