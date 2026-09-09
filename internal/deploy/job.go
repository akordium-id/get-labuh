package deploy

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/akordium-id/get-labuh/internal/caddy"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/logging"
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
	Priority       int
}

type ComposeJob struct {
	ComposeID          string
	ComposeFilePath    string
	ComposeProjectName string
	LogPath            string
}

type Pipeline struct {
	dockerClient *docker.Client
	caddyClient  *caddy.Client
	settingRepo  *repo.SettingRepo
	appRepo      *repo.ApplicationRepo
	deployRepo   *repo.DeploymentRepo
}

func NewPipeline(dockerClient *docker.Client, caddyClient *caddy.Client, settingRepo *repo.SettingRepo, appRepo *repo.ApplicationRepo, deployRepo *repo.DeploymentRepo) *Pipeline {
	return &Pipeline{
		dockerClient: dockerClient,
		caddyClient:  caddyClient,
		settingRepo:  settingRepo,
		appRepo:      appRepo,
		deployRepo:   deployRepo,
	}
}

func (p *Pipeline) Execute(ctx context.Context, job DeploymentJob) error {
	logDir := filepath.Dir(job.LogPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return &DeployError{Step: "init", Cause: err}
	}

	logFile, err := logging.NewRotatingWriter(job.LogPath, 10*1024*1024, 5)
	if err != nil {
		return &DeployError{Step: "init", Cause: err}
	}
	defer logFile.Close()

	writeLog(logFile, "Starting deployment pipeline...")
	if p.deployRepo != nil {
		_ = p.deployRepo.MarkStarted(job.DeploymentID)
	}

	var imageName string

	switch job.SourceType {
	case string(models.SourceTypeGit):
		writeLog(logFile, "Cloning git repository...")
		if p.deployRepo != nil {
			if err := p.deployRepo.UpdateStep(job.DeploymentID, "clone"); err != nil {
				writeLog(logFile, fmt.Sprintf("Warning: failed to update step: %v", err))
			}
			if err := p.deployRepo.UpdateStatus(job.DeploymentID, models.DeployStatusCloning); err != nil {
				writeLog(logFile, fmt.Sprintf("Warning: failed to update status: %v", err))
			}
		}

		cloneCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		buildDir := filepath.Join("/tmp/labuh-builds", job.DeploymentID)
		if err := os.MkdirAll(buildDir, 0755); err != nil {
			errMsg := fmt.Sprintf("failed to create build directory: %v", err)
			writeLog(logFile, errMsg)
			if p.deployRepo != nil {
				p.deployRepo.UpdateStatusWithError(job.DeploymentID, models.DeployStatusFailed, errMsg)
			}
			return &DeployError{Step: "clone", Cause: err, Output: errMsg}
		}

		if err := p.dockerClient.CloneGitRepo(cloneCtx, job.RepositoryURL, job.Branch, buildDir); err != nil {
			errMsg := fmt.Sprintf("git clone failed: %v", err)
			writeLog(logFile, errMsg)
			if p.deployRepo != nil {
				p.deployRepo.UpdateStatusWithError(job.DeploymentID, models.DeployStatusFailed, errMsg)
			}
			return &DeployError{Step: "clone", Cause: err, Output: errMsg}
		}

		writeLog(logFile, "Git clone completed")

		writeLog(logFile, "Building docker image from repository...")
		if p.deployRepo != nil {
			_ = p.deployRepo.UpdateStep(job.DeploymentID, "build")
			_ = p.deployRepo.UpdateStatus(job.DeploymentID, models.DeployStatusBuilding)
		}

		buildCtx, cancelBuild := context.WithTimeout(ctx, 20*time.Minute)
		defer cancelBuild()

		cacheRef := docker.GenerateCacheRef(job.ApplicationID, job.Branch)
		builtImage, err := p.dockerClient.BuildWithCache(buildCtx, job.ApplicationID, job.Branch, buildDir, cacheRef)
		if err != nil {
			errMsg := fmt.Sprintf("docker build failed: %v", err)
			writeLog(logFile, errMsg)
			p.cleanupFailedContainer(ctx, job)
			if p.deployRepo != nil {
				p.deployRepo.UpdateStatusWithError(job.DeploymentID, models.DeployStatusFailed, errMsg)
			}
			return &DeployError{Step: "build", Cause: err, Output: errMsg}
		}

		writeLog(logFile, "Docker build completed: "+builtImage)
		imageName = builtImage

	case string(models.SourceTypeDockerfile):
		writeLog(logFile, "Building from Dockerfile...")
		if p.deployRepo != nil {
			if err := p.deployRepo.UpdateStep(job.DeploymentID, "build"); err != nil {
				writeLog(logFile, fmt.Sprintf("Warning: failed to update step: %v", err))
			}
			if err := p.deployRepo.UpdateStatus(job.DeploymentID, models.DeployStatusBuilding); err != nil {
				writeLog(logFile, fmt.Sprintf("Warning: failed to update status: %v", err))
			}
		}

		buildCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
		defer cancel()

		buildDir := filepath.Join("/tmp/labuh-builds", job.DeploymentID)
		if err := os.MkdirAll(buildDir, 0755); err != nil {
			errMsg := fmt.Sprintf("failed to create build directory: %v", err)
			writeLog(logFile, errMsg)
			if p.deployRepo != nil {
				p.deployRepo.UpdateStatusWithError(job.DeploymentID, models.DeployStatusFailed, errMsg)
			}
			return &DeployError{Step: "build", Cause: err, Output: errMsg}
		}

		cacheRef := docker.GenerateCacheRef(job.ApplicationID, job.Branch)
		builtImage, err := p.dockerClient.BuildWithCache(buildCtx, job.ApplicationID, job.Branch, buildDir, cacheRef)
		if err != nil {
			errMsg := fmt.Sprintf("docker build failed: %v", err)
			writeLog(logFile, errMsg)
			p.cleanupFailedContainer(ctx, job)
			if p.deployRepo != nil {
				p.deployRepo.UpdateStatusWithError(job.DeploymentID, models.DeployStatusFailed, errMsg)
			}
			return &DeployError{Step: "build", Cause: err, Output: errMsg}
		}

		writeLog(logFile, "Docker build completed: "+builtImage)
		imageName = builtImage

	case string(models.SourceTypeDockerImage):
		writeLog(logFile, fmt.Sprintf("Pulling image %s...", job.DockerImage))
		if p.deployRepo != nil {
			if err := p.deployRepo.UpdateStep(job.DeploymentID, "push"); err != nil {
				writeLog(logFile, fmt.Sprintf("Warning: failed to update step: %v", err))
			}
			if err := p.deployRepo.UpdateStatus(job.DeploymentID, models.DeployStatusDeploying); err != nil {
				writeLog(logFile, fmt.Sprintf("Warning: failed to update status: %v", err))
			}
		}

		pullCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
		defer cancel()

		if err := p.dockerClient.PullImage(pullCtx, job.DockerImage); err != nil {
			errMsg := fmt.Sprintf("failed to pull image: %v", err)
			writeLog(logFile, errMsg)
			if p.deployRepo != nil {
				p.deployRepo.UpdateStatusWithError(job.DeploymentID, models.DeployStatusFailed, errMsg)
			}
			return &DeployError{Step: "push", Cause: err, Output: errMsg}
		}
		imageName = job.DockerImage

	default:
		errMsg := fmt.Sprintf("unknown source type: %s", job.SourceType)
		writeLog(logFile, errMsg)
		if p.deployRepo != nil {
			p.deployRepo.UpdateStatusWithError(job.DeploymentID, models.DeployStatusFailed, errMsg)
		}
		return &DeployError{Step: "init", Output: errMsg}
	}

	writeLog(logFile, "Deploying container...")
	if p.deployRepo != nil {
		_ = p.deployRepo.UpdateStep(job.DeploymentID, "deploy")
		_ = p.deployRepo.UpdateStatus(job.DeploymentID, models.DeployStatusDeploying)
	}

	var appSlug string = "app"
	var customDomain *string
	var appPort int = job.AppPort

	if p.appRepo != nil {
		app, err := p.appRepo.GetByID(job.ApplicationID)
		if err == nil && app != nil {
			appSlug = app.Slug
			customDomain = app.CustomDomain
			if appPort <= 0 {
				appPort = app.AppPort
			}

			// Clean up old container if running
			if app.ContainerID != nil && *app.ContainerID != "" {
				writeLog(logFile, "Stopping existing container...")
				_ = p.dockerClient.StopContainer(ctx, *app.ContainerID, 10)
				_ = p.dockerClient.RemoveContainer(ctx, *app.ContainerID)
			}
		}
	}

	if appPort <= 0 {
		appPort = 8080
	}

	var dockerEnvVars []struct{ Key, Value string }
	for _, ev := range job.EnvVars {
		dockerEnvVars = append(dockerEnvVars, struct{ Key, Value string }{
			Key:   ev.Key,
			Value: ev.Value,
		})
	}

	containerID, err := p.dockerClient.RunContainer(ctx, job.ApplicationID, "prod", appSlug, imageName, appPort, dockerEnvVars)
	if err != nil {
		errMsg := fmt.Sprintf("failed to run container: %v", err)
		writeLog(logFile, errMsg)
		if p.deployRepo != nil {
			p.deployRepo.UpdateStatusWithError(job.DeploymentID, models.DeployStatusFailed, errMsg)
		}
		return &DeployError{Step: "deploy", Cause: err, Output: errMsg}
	}

	writeLog(logFile, fmt.Sprintf("Container started successfully (ID: %s)", containerID))
	if p.appRepo != nil {
		_ = p.appRepo.UpdateContainerAndStatus(job.ApplicationID, containerID, models.AppStatusRunning)
	}

	if p.caddyClient != nil && customDomain != nil && *customDomain != "" {
		containerName := p.dockerClient.GenerateContainerName("prod", appSlug, job.ApplicationID)
		writeLog(logFile, fmt.Sprintf("Configuring Caddy reverse proxy for %s -> %s:%d", *customDomain, containerName, appPort))
		if err := p.caddyClient.AddRoute(*customDomain, containerName, appPort); err != nil {
			writeLog(logFile, fmt.Sprintf("Warning: failed to add Caddy route: %v", err))
		} else {
			writeLog(logFile, "Caddy route configured successfully")
		}
	}

	writeLog(logFile, "Deployment completed successfully")
	if p.deployRepo != nil {
		_ = p.deployRepo.UpdateStep(job.DeploymentID, "done")
		_ = p.deployRepo.UpdateStatus(job.DeploymentID, models.DeployStatusSuccess)
		_ = p.deployRepo.MarkFinished(job.DeploymentID)
	}

	return nil
}

func (p *Pipeline) ExecuteCompose(ctx context.Context, job ComposeJob) error {
	logDir := filepath.Dir(job.LogPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return &DeployError{Step: "init", Cause: err}
	}

	logFile, err := os.Create(job.LogPath)
	if err != nil {
		return &DeployError{Step: "init", Cause: err}
	}
	defer logFile.Close()

	writeLog(logFile, "Starting compose deployment...")

	args := []string{"-f", job.ComposeFilePath, "-p", job.ComposeProjectName, "up", "-d", "--build"}
	cmd := exec.CommandContext(ctx, "docker", append([]string{"compose"}, args...)...)
	cmd.Dir = "/tmp/labuh-builds/" + job.ComposeID

	output, err := cmd.CombinedOutput()
	writeLog(logFile, string(output))
	if err != nil {
		errMsg := fmt.Sprintf("compose up failed: %v", err)
		writeLog(logFile, errMsg)
		return &DeployError{Step: "deploy", Cause: err, Output: errMsg}
	}

	writeLog(logFile, "Compose deployment completed successfully")
	return nil
}

func (p *Pipeline) cleanupFailedContainer(ctx context.Context, job DeploymentJob) {
	if p.appRepo == nil {
		return
	}

	app, err := p.appRepo.GetByID(job.ApplicationID)
	if err != nil || app == nil || app.ContainerID == nil {
		return
	}

	containerID := *app.ContainerID
	if containerID == "" {
		return
	}

	slog.Info("cleaning up failed container", "container_id", containerID, "app_id", job.ApplicationID)
	if err := p.dockerClient.RemoveContainer(ctx, containerID); err != nil {
		slog.Error("failed to remove failed container", "container_id", containerID, "error", err)
	}
}

func writeLog(file logging.LogWriter, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] %s\n", timestamp, message)
	if _, err := file.Write([]byte(line)); err != nil {
		// best effort
	}
	if err := file.Sync(); err != nil {
		// best effort
	}
}
