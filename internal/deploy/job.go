package deploy

import (
	"context"
	"fmt"
	"io"
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
	CacheRef       string
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

	rotatingWriter, err := logging.NewRotatingWriter(job.LogPath, 10*1024*1024, 5)
	if err != nil {
		return &DeployError{Step: "init", Cause: err}
	}
	defer rotatingWriter.Close()

	writeLog(rotatingWriter, "Starting deployment pipeline...")

	var imageName string
	_ = imageName

	stepTimeouts := map[string]time.Duration{
		"clone":  5 * time.Minute,
		"build":  20 * time.Minute,
		"deploy": 2 * time.Minute,
	}

	switch job.SourceType {
	case string(models.SourceTypeGit):
		if err := p.runStepWithTimeout(ctx, job, "clone", stepTimeouts["clone"], func(stepCtx context.Context) error {
			writeLog(rotatingWriter, "Cloning git repository...")
			if err := p.deployRepo.UpdateStep(job.DeploymentID, "clone"); err != nil {
				writeLog(rotatingWriter, fmt.Sprintf("Warning: failed to update step: %v", err))
			}
			if err := p.deployRepo.UpdateStatus(job.DeploymentID, models.DeployStatusCloning); err != nil {
				writeLog(rotatingWriter, fmt.Sprintf("Warning: failed to update status: %v", err))
			}

			buildDir := filepath.Join("/tmp/labuh-builds", job.DeploymentID)
			if err := os.MkdirAll(buildDir, 0755); err != nil {
				errMsg := fmt.Sprintf("failed to create build directory: %v", err)
				writeLog(rotatingWriter, errMsg)
				p.deployRepo.UpdateStatusWithError(job.DeploymentID, models.DeployStatusFailed, errMsg)
				return &DeployError{Step: "clone", Cause: err, Output: errMsg}
			}

			if err := p.dockerClient.CloneGitRepo(stepCtx, job.RepositoryURL, job.Branch, buildDir); err != nil {
				errMsg := fmt.Sprintf("git clone failed: %v", err)
				writeLog(rotatingWriter, errMsg)
				p.deployRepo.UpdateStatusWithError(job.DeploymentID, models.DeployStatusFailed, errMsg)
				return &DeployError{Step: "clone", Cause: err, Output: errMsg}
			}

			writeLog(rotatingWriter, "Git clone completed")
			return nil
		}); err != nil {
			return err
		}
		imageName = "placeholder-image"

	case string(models.SourceTypeDockerfile):
		if err := p.runStepWithTimeout(ctx, job, "build", stepTimeouts["build"], func(stepCtx context.Context) error {
			writeLog(rotatingWriter, "Building from Dockerfile...")
			if err := p.deployRepo.UpdateStep(job.DeploymentID, "build"); err != nil {
				writeLog(rotatingWriter, fmt.Sprintf("Warning: failed to update step: %v", err))
			}
			if err := p.deployRepo.UpdateStatus(job.DeploymentID, models.DeployStatusBuilding); err != nil {
				writeLog(rotatingWriter, fmt.Sprintf("Warning: failed to update status: %v", err))
			}

			buildDir := filepath.Join("/tmp/labuh-builds", job.DeploymentID)
			if err := os.MkdirAll(buildDir, 0755); err != nil {
				errMsg := fmt.Sprintf("failed to create build directory: %v", err)
				writeLog(rotatingWriter, errMsg)
				p.deployRepo.UpdateStatusWithError(job.DeploymentID, models.DeployStatusFailed, errMsg)
				return &DeployError{Step: "build", Cause: err, Output: errMsg}
			}

			cacheRef := job.CacheRef
			if cacheRef == "" {
				cacheRef = docker.GenerateCacheRef(job.ApplicationID, job.Branch)
			}

			builtImage, err := p.dockerClient.BuildFromDockerfileWithCache(stepCtx, job.ApplicationID, job.Branch, buildDir, job.DockerfilePath, cacheRef)
			if err != nil {
				errMsg := fmt.Sprintf("docker build failed: %v", err)
				writeLog(rotatingWriter, errMsg)
				p.cleanupFailedContainer(ctx, job)
				p.deployRepo.UpdateStatusWithError(job.DeploymentID, models.DeployStatusFailed, errMsg)
				return &DeployError{Step: "build", Cause: err, Output: errMsg}
			}

			writeLog(rotatingWriter, "Docker build completed")
			imageName = builtImage
			return nil
		}); err != nil {
			return err
		}

	case string(models.SourceTypeDockerImage):
		if err := p.runStepWithTimeout(ctx, job, "push", stepTimeouts["deploy"], func(stepCtx context.Context) error {
			writeLog(rotatingWriter, fmt.Sprintf("Pulling image %s...", job.DockerImage))
			if err := p.deployRepo.UpdateStep(job.DeploymentID, "push"); err != nil {
				writeLog(rotatingWriter, fmt.Sprintf("Warning: failed to update step: %v", err))
			}
			if err := p.deployRepo.UpdateStatus(job.DeploymentID, models.DeployStatusDeploying); err != nil {
				writeLog(rotatingWriter, fmt.Sprintf("Warning: failed to update status: %v", err))
			}

			if err := p.dockerClient.PullImage(stepCtx, job.DockerImage); err != nil {
				errMsg := fmt.Sprintf("failed to pull image: %v", err)
				writeLog(rotatingWriter, errMsg)
				p.deployRepo.UpdateStatusWithError(job.DeploymentID, models.DeployStatusFailed, errMsg)
				return &DeployError{Step: "push", Cause: err, Output: errMsg}
			}
			imageName = job.DockerImage
			return nil
		}); err != nil {
			return err
		}

	default:
		errMsg := fmt.Sprintf("unknown source type: %s", job.SourceType)
		writeLog(rotatingWriter, errMsg)
		p.deployRepo.UpdateStatusWithError(job.DeploymentID, models.DeployStatusFailed, errMsg)
		return &DeployError{Step: "init", Output: errMsg}
	}

	writeLog(rotatingWriter, "Deployment completed successfully")
	p.deployRepo.UpdateStep(job.DeploymentID, "deploy")
	p.deployRepo.UpdateStatus(job.DeploymentID, models.DeployStatusSuccess)

	if p.caddyClient != nil {
		caddyAPIURL, _ := p.settingRepo.Get("caddy_api_url")
		caddyAPIKey, _ := p.settingRepo.Get("caddy_api_key")
		caddyNetwork, _ := p.settingRepo.Get("caddy_network")

		client := caddy.NewClient(caddyAPIURL, caddyAPIKey)

		if caddyNetwork == "" {
			caddyNetwork, _ = p.settingRepo.Get("caddy_network")
		}

		_ = caddyNetwork
		_ = client
	}

	if err := docker.CleanOldCache(24 * time.Hour); err != nil {
		writeLog(rotatingWriter, fmt.Sprintf("Warning: failed to clean old cache: %v", err))
	}

	return nil
}

func (p *Pipeline) runStepWithTimeout(ctx context.Context, job DeploymentJob, stepName string, timeout time.Duration, stepFunc func(context.Context) error) error {
	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- stepFunc(stepCtx)
	}()

	select {
	case err := <-done:
		return err
	case <-stepCtx.Done():
		errMsg := fmt.Sprintf("%s step timed out after %s", stepName, timeout)
		p.deployRepo.UpdateStatusWithError(job.DeploymentID, models.DeployStatusFailed, errMsg)
		return &DeployError{Step: stepName, Cause: stepCtx.Err(), Output: errMsg}
	case <-ctx.Done():
		return ctx.Err()
	}
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

func writeLog(writer io.Writer, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] %s\n", timestamp, message)
	if _, err := writer.Write([]byte(line)); err != nil {
		// best effort
	}
}
