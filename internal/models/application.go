package models

import "time"

type SourceType string

const (
	SourceTypeGit           SourceType = "git"
	SourceTypeDockerImage   SourceType = "docker_image"
	SourceTypeDockerfile    SourceType = "dockerfile"
	SourceTypeNixpacks      SourceType = "nixpacks"
	SourceTypeDockerCompose SourceType = "docker_compose"
	SourceTypeDropzone      SourceType = "dropzone"
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
	ID                    string
	EnvironmentID         string
	Name                  string
	Slug                  string
	SourceType            SourceType
	RepositoryURL         *string
	Branch                *string
	BuildPath             *string
	DockerfilePath        *string
	DockerImage           *string
	CustomDomain          *string
	AppPort               int
	ContainerID           *string
	ContainerName         *string
	ServerID              *string
	Status                AppStatus
	WebhookSecret         *string
	CPULimit              *float64
	MemoryLimit           *int64
	MemoryReservation     *int64
	HealthcheckPath       *string
	HealthcheckTimeout    *int
	BasicAuthUser         *string
	BasicAuthPassword     *string
	ParentAppID           *string
	PRNumber              *int
	IsPreview             bool
	PreviewTemplateDomain *string
	PRPreviewsEnabled     bool
	GitToken              *string
	PreDeploymentCommand  *string
	PostDeploymentCommand *string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type CreateApplicationInput struct {
	EnvironmentID         string
	ServerID              *string
	Name                  string
	Slug                  string
	SourceType            SourceType
	RepositoryURL         *string
	Branch                *string
	BuildPath             *string
	DockerfilePath        *string
	DockerImage           *string
	CustomDomain          *string
	AppPort               int
	WebhookSecret         *string
	CPULimit              *float64
	MemoryLimit           *int64
	MemoryReservation     *int64
	HealthcheckPath       *string
	HealthcheckTimeout    *int
	BasicAuthUser         *string
	BasicAuthPassword     *string
	ParentAppID           *string
	PRNumber              *int
	IsPreview             bool
	PreviewTemplateDomain *string
	PRPreviewsEnabled     bool
	GitToken              *string
	PreDeploymentCommand  *string
	PostDeploymentCommand *string
}

type AppEnvVar struct {
	ID            string
	ApplicationID string
	Key           string
	Value         string
	IsSecret      bool
	IsEncrypted   bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CreateAppEnvVarInput struct {
	ApplicationID string
	Key           string
	Value         string
	IsSecret      bool
	IsEncrypted   bool
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
	ID            string
	ApplicationID string
	CommitHash    *string
	CommitMessage *string
	ImageName     *string
	Status        DeployStatus
	Step          *string
	Output        *string
	ErrorMessage  *string
	LogPath       *string
	StartedAt     *time.Time
	FinishedAt    *time.Time
	CreatedAt     time.Time
	Priority      int
}

type CreateDeploymentInput struct {
	ApplicationID string
	CommitHash    *string
	CommitMessage *string
	ImageName     *string
	LogPath       *string
	Step          *string
	Output        *string
}

type VolumeType string

const (
	VolumeTypeNamed VolumeType = "named"
	VolumeTypeBind  VolumeType = "bind"
)

type AppVolume struct {
	ID            string     `json:"id"`
	ApplicationID string     `json:"application_id"`
	Name          string     `json:"name"`
	VolumeType    VolumeType `json:"volume_type"`
	HostPath      *string    `json:"host_path"`
	MountPath     string     `json:"mount_path"`
	IsReadOnly    bool       `json:"is_read_only"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type CreateAppVolumeInput struct {
	ApplicationID string     `json:"application_id"`
	Name          string     `json:"name"`
	VolumeType    VolumeType `json:"volume_type"`
	HostPath      *string    `json:"host_path"`
	MountPath     string     `json:"mount_path"`
	IsReadOnly    bool       `json:"is_read_only"`
}

type AppFileMount struct {
	ID            string    `json:"id"`
	ApplicationID string    `json:"application_id"`
	MountPath     string    `json:"mount_path"`
	Content       string    `json:"content"`
	FileMode      string    `json:"file_mode"`
	IsReadOnly    bool      `json:"is_read_only"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type CreateAppFileMountInput struct {
	ApplicationID string `json:"application_id"`
	MountPath     string `json:"mount_path"`
	Content       string `json:"content"`
	FileMode      string `json:"file_mode"`
	IsReadOnly    bool   `json:"is_read_only"`
}

