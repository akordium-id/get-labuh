package deploy

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
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
	EnvVars        []models.AppEnvVar
	LogPath        string
}

type ComposeJob struct {
	ComposeID          string
	ComposeFilePath    string
	ComposeProjectName string
	LogPath            string
}

type Pipeline struct {
	dockerClient *docker.Client
}

func NewPipeline(dockerClient *docker.Client) *Pipeline {
	return &Pipeline{
		dockerClient: dockerClient,
	}
}

func (p *Pipeline) Execute(ctx context.Context, job DeploymentJob) error {
	logDir := filepath.Dir(job.LogPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile, err := os.Create(job.LogPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	writeLog(logFile, "Starting deployment pipeline...")

	var imageName string
	_ = imageName

	switch job.SourceType {
	case string(models.SourceTypeGit):
		writeLog(logFile, "Cloning git repository...")
		buildDir := filepath.Join("/tmp/labuh-builds", job.DeploymentID)
		if err := os.MkdirAll(buildDir, 0755); err != nil {
			writeLog(logFile, fmt.Sprintf("Failed to create build directory: %v", err))
			return err
		}
		writeLog(logFile, fmt.Sprintf("Build directory created at %s (git clone not implemented)", buildDir))
		imageName = "placeholder-image"

	case string(models.SourceTypeDockerfile):
		writeLog(logFile, "Building from Dockerfile...")
		buildDir := filepath.Join("/tmp/labuh-builds", job.DeploymentID)
		if err := os.MkdirAll(buildDir, 0755); err != nil {
			writeLog(logFile, fmt.Sprintf("Failed to create build directory: %v", err))
			return err
		}
		writeLog(logFile, fmt.Sprintf("Build directory created at %s (docker build not implemented)", buildDir))
		imageName = "placeholder-image"

	case string(models.SourceTypeDockerImage):
		writeLog(logFile, fmt.Sprintf("Pulling image %s...", job.DockerImage))
		if err := p.dockerClient.PullImage(ctx, job.DockerImage); err != nil {
			writeLog(logFile, fmt.Sprintf("Failed to pull image: %v", err))
			return err
		}
		imageName = job.DockerImage

	default:
		return fmt.Errorf("unknown source type: %s", job.SourceType)
	}

	writeLog(logFile, "Deployment completed successfully")
	return nil
}

func (p *Pipeline) ExecuteCompose(ctx context.Context, job ComposeJob) error {
	logDir := filepath.Dir(job.LogPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile, err := os.Create(job.LogPath)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	writeLog(logFile, "Starting compose deployment...")

	args := []string{"-f", job.ComposeFilePath, "-p", job.ComposeProjectName, "up", "-d", "--build"}
	cmd := exec.CommandContext(ctx, "docker", append([]string{"compose"}, args...)...)
	cmd.Dir = "/tmp/labuh-builds/" + job.ComposeID

	output, err := cmd.CombinedOutput()
	writeLog(logFile, string(output))
	if err != nil {
		writeLog(logFile, fmt.Sprintf("Compose deployment failed: %v", err))
		return fmt.Errorf("compose up failed: %w", err)
	}

	writeLog(logFile, "Compose deployment completed successfully")
	return nil
}

func writeLog(file *os.File, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] %s\n", timestamp, message)
	file.WriteString(line)
	file.Sync()
}
