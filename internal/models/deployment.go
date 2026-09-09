package models

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
