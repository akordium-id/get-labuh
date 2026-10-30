package deploy

import (
	"context"
	"errors"
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
	ServerID       *string
	AppSlug        string
	CustomDomain   *string
	ExtraLogWriter io.Writer
	StatusCallback func(step string, status string, err error)
}

type ComposeJob struct {
	ComposeID          string
	ComposeFilePath    string
	ComposeProjectName string
	LogPath            string
}

type Pipeline struct {
	dockerClient *docker.Client
	dockerPool   *docker.ClientPool
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

func (p *Pipeline) SetDockerPool(pool *docker.ClientPool) {
	p.dockerPool = pool
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
	defer func() { _ = logFile.Close() }()

	activeLogWriter := logging.NewTeeLogWriter(logFile, job.ExtraLogWriter)

	updateStep := func(step string, status models.DeployStatus, errMsg string) {
		if p.deployRepo != nil {
			_ = p.deployRepo.UpdateStep(job.DeploymentID, step)
			if errMsg != "" {
				_ = p.deployRepo.UpdateStatusWithError(job.DeploymentID, status, errMsg)
			} else {
				_ = p.deployRepo.UpdateStatus(job.DeploymentID, status)
			}
		}
		if job.StatusCallback != nil {
			var err error
			if errMsg != "" {
				err = errors.New(errMsg)
			}
			job.StatusCallback(step, string(status), err)
		}
	}

	writeLog(activeLogWriter, "Starting deployment pipeline...")
	if p.deployRepo != nil {
		_ = p.deployRepo.MarkStarted(job.DeploymentID)
	}

	targetDockerClient := p.dockerClient
	if p.dockerPool != nil && job.ServerID != nil && *job.ServerID != "" && *job.ServerID != "local" {
		writeLog(activeLogWriter, fmt.Sprintf("Routing deployment to target remote server (ID: %s)...", *job.ServerID))
		cli, err := p.dockerPool.GetClient(ctx, job.ServerID)
		if err != nil {
			errMsg := fmt.Sprintf("failed to connect to remote server docker daemon: %v", err)
			writeLog(activeLogWriter, errMsg)
			updateStep("init", models.DeployStatusFailed, errMsg)
			return &DeployError{Step: "init", Cause: err, Output: errMsg}
		}
		targetDockerClient = cli
		writeLog(activeLogWriter, "Successfully connected to remote Docker daemon via SSH tunnel.")
	}

	var imageName string

	switch job.SourceType {
	case string(models.SourceTypeGit):
		writeLog(activeLogWriter, "Cloning git repository...")
		updateStep("clone", models.DeployStatusCloning, "")

		cloneCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		buildDir := filepath.Join("/tmp/labuh-builds", job.DeploymentID)
		if err := os.MkdirAll(buildDir, 0755); err != nil {
			errMsg := fmt.Sprintf("failed to create build directory: %v", err)
			writeLog(activeLogWriter, errMsg)
			updateStep("clone", models.DeployStatusFailed, errMsg)
			return &DeployError{Step: "clone", Cause: err, Output: errMsg}
		}

		if err := p.dockerClient.CloneGitRepo(cloneCtx, job.RepositoryURL, job.Branch, buildDir); err != nil {
			errMsg := fmt.Sprintf("git clone failed: %v", err)
			writeLog(activeLogWriter, errMsg)
			updateStep("clone", models.DeployStatusFailed, errMsg)
			return &DeployError{Step: "clone", Cause: err, Output: errMsg}
		}

		writeLog(activeLogWriter, "Git clone completed")

		dockerfilePath := "Dockerfile"
		if job.DockerfilePath != "" {
			dockerfilePath = job.DockerfilePath
		}

		fullDockerfilePath := filepath.Join(buildDir, dockerfilePath)
		hasDockerfile := false
		if fi, err := os.Stat(fullDockerfilePath); err == nil && !fi.IsDir() {
			hasDockerfile = true
		}

		buildCtx, cancelBuild := context.WithTimeout(ctx, 20*time.Minute)
		defer cancelBuild()

		if hasDockerfile {
			writeLog(activeLogWriter, fmt.Sprintf("Found %s, building docker image...", dockerfilePath))
			updateStep("build", models.DeployStatusBuilding, "")

			cacheRef := docker.GenerateCacheRef(job.ApplicationID, job.Branch)
			builtImage, err := p.dockerClient.BuildFromDockerfileWithCache(buildCtx, job.ApplicationID, buildDir, dockerfilePath, cacheRef)
			if err != nil {
				errMsg := fmt.Sprintf("docker build failed: %v", err)
				writeLog(activeLogWriter, errMsg)
				p.cleanupFailedContainer(ctx, job)
				updateStep("build", models.DeployStatusFailed, errMsg)
				return &DeployError{Step: "build", Cause: err, Output: errMsg}
			}
			writeLog(activeLogWriter, "Docker build completed: "+builtImage)
			imageName = builtImage
		} else {
			writeLog(activeLogWriter, "No Dockerfile found. Using Nixpacks to auto-detect runtime...")
			updateStep("build", models.DeployStatusBuilding, "")

			envs := make(map[string]string)
			for _, ev := range job.EnvVars {
				envs[ev.Key] = ev.Value
			}

			builtImage, err := p.dockerClient.BuildWithNixpacks(buildCtx, job.ApplicationID, buildDir, envs, activeLogWriter)
			if err != nil {
				errMsg := fmt.Sprintf("nixpacks build failed: %v", err)
				writeLog(activeLogWriter, errMsg)
				p.cleanupFailedContainer(ctx, job)
				updateStep("build", models.DeployStatusFailed, errMsg)
				return &DeployError{Step: "build", Cause: err, Output: errMsg}
			}
			writeLog(activeLogWriter, "Nixpacks build completed: "+builtImage)
			imageName = builtImage
		}

	case string(models.SourceTypeNixpacks):
		writeLog(activeLogWriter, "Cloning git repository for Nixpacks build...")
		updateStep("clone", models.DeployStatusCloning, "")

		cloneCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		buildDir := filepath.Join("/tmp/labuh-builds", job.DeploymentID)
		if err := os.MkdirAll(buildDir, 0755); err != nil {
			errMsg := fmt.Sprintf("failed to create build directory: %v", err)
			writeLog(activeLogWriter, errMsg)
			updateStep("clone", models.DeployStatusFailed, errMsg)
			return &DeployError{Step: "clone", Cause: err, Output: errMsg}
		}

		if err := p.dockerClient.CloneGitRepo(cloneCtx, job.RepositoryURL, job.Branch, buildDir); err != nil {
			errMsg := fmt.Sprintf("git clone failed: %v", err)
			writeLog(activeLogWriter, errMsg)
			updateStep("clone", models.DeployStatusFailed, errMsg)
			return &DeployError{Step: "clone", Cause: err, Output: errMsg}
		}

		writeLog(activeLogWriter, "Git clone completed. Running Nixpacks build...")
		updateStep("build", models.DeployStatusBuilding, "")

		buildCtx, cancelBuild := context.WithTimeout(ctx, 20*time.Minute)
		defer cancelBuild()

		envs := make(map[string]string)
		for _, ev := range job.EnvVars {
			envs[ev.Key] = ev.Value
		}

		builtImage, err := p.dockerClient.BuildWithNixpacks(buildCtx, job.ApplicationID, buildDir, envs, activeLogWriter)
		if err != nil {
			errMsg := fmt.Sprintf("nixpacks build failed: %v", err)
			writeLog(activeLogWriter, errMsg)
			p.cleanupFailedContainer(ctx, job)
			updateStep("build", models.DeployStatusFailed, errMsg)
			return &DeployError{Step: "build", Cause: err, Output: errMsg}
		}
		writeLog(activeLogWriter, "Nixpacks build completed: "+builtImage)
		imageName = builtImage

	case string(models.SourceTypeDockerfile):
		writeLog(activeLogWriter, "Building from Dockerfile...")
		updateStep("build", models.DeployStatusBuilding, "")

		buildCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
		defer cancel()

		buildDir := filepath.Join("/tmp/labuh-builds", job.DeploymentID)
		if err := os.MkdirAll(buildDir, 0755); err != nil {
			errMsg := fmt.Sprintf("failed to create build directory: %v", err)
			writeLog(activeLogWriter, errMsg)
			updateStep("build", models.DeployStatusFailed, errMsg)
			return &DeployError{Step: "build", Cause: err, Output: errMsg}
		}

		cacheRef := docker.GenerateCacheRef(job.ApplicationID, job.Branch)
		builtImage, err := p.dockerClient.BuildWithCache(buildCtx, job.ApplicationID, job.Branch, buildDir, cacheRef)
		if err != nil {
			errMsg := fmt.Sprintf("docker build failed: %v", err)
			writeLog(activeLogWriter, errMsg)
			p.cleanupFailedContainer(ctx, job)
			updateStep("build", models.DeployStatusFailed, errMsg)
			return &DeployError{Step: "build", Cause: err, Output: errMsg}
		}

		writeLog(activeLogWriter, "Docker build completed: "+builtImage)
		imageName = builtImage

	case string(models.SourceTypeDropzone):
		writeLog(activeLogWriter, "Processing dropped source code...")
		updateStep("build", models.DeployStatusBuilding, "")

		buildDir := filepath.Join("/tmp/labuh-sources", job.ApplicationID)
		if fi, err := os.Stat(buildDir); err != nil || !fi.IsDir() {
			errMsg := fmt.Sprintf("source directory not found: %s", buildDir)
			writeLog(activeLogWriter, errMsg)
			updateStep("build", models.DeployStatusFailed, errMsg)
			return &DeployError{Step: "build", Output: errMsg}
		}

		dockerfilePath := "Dockerfile"
		if job.DockerfilePath != "" {
			dockerfilePath = job.DockerfilePath
		}
		fullDockerfilePath := filepath.Join(buildDir, dockerfilePath)
		hasDockerfile := false
		if fi, err := os.Stat(fullDockerfilePath); err == nil && !fi.IsDir() {
			hasDockerfile = true
		}

		buildCtx, cancelBuild := context.WithTimeout(ctx, 20*time.Minute)
		defer cancelBuild()

		if hasDockerfile {
			writeLog(activeLogWriter, fmt.Sprintf("Found %s in uploaded source. Building docker image...", dockerfilePath))
			cacheRef := docker.GenerateCacheRef(job.ApplicationID, "dropzone")
			builtImage, err := p.dockerClient.BuildFromDockerfileWithCache(buildCtx, job.ApplicationID, buildDir, dockerfilePath, cacheRef)
			if err != nil {
				errMsg := fmt.Sprintf("docker build failed: %v", err)
				writeLog(activeLogWriter, errMsg)
				p.cleanupFailedContainer(ctx, job)
				updateStep("build", models.DeployStatusFailed, errMsg)
				return &DeployError{Step: "build", Cause: err, Output: errMsg}
			}
			writeLog(activeLogWriter, "Docker build completed: "+builtImage)
			imageName = builtImage
		} else {
			writeLog(activeLogWriter, "No Dockerfile found. Using Nixpacks to auto-detect and build runtime...")
			envs := make(map[string]string)
			for _, ev := range job.EnvVars {
				envs[ev.Key] = ev.Value
			}
			builtImage, err := p.dockerClient.BuildWithNixpacks(buildCtx, job.ApplicationID, buildDir, envs, activeLogWriter)
			if err != nil {
				errMsg := fmt.Sprintf("nixpacks build failed: %v", err)
				writeLog(activeLogWriter, errMsg)
				p.cleanupFailedContainer(ctx, job)
				updateStep("build", models.DeployStatusFailed, errMsg)
				return &DeployError{Step: "build", Cause: err, Output: errMsg}
			}
			writeLog(activeLogWriter, "Nixpacks build completed: "+builtImage)
			imageName = builtImage
		}

	case string(models.SourceTypeDockerImage):
		writeLog(activeLogWriter, fmt.Sprintf("Pulling image %s...", job.DockerImage))
		updateStep("push", models.DeployStatusDeploying, "")

		pullCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
		defer cancel()

		if err := p.dockerClient.PullImage(pullCtx, job.DockerImage); err != nil {
			errMsg := fmt.Sprintf("failed to pull image: %v", err)
			writeLog(activeLogWriter, errMsg)
			updateStep("push", models.DeployStatusFailed, errMsg)
			return &DeployError{Step: "push", Cause: err, Output: errMsg}
		}
		imageName = job.DockerImage

	default:
		errMsg := fmt.Sprintf("unknown source type: %s", job.SourceType)
		writeLog(activeLogWriter, errMsg)
		updateStep("init", models.DeployStatusFailed, errMsg)
		return &DeployError{Step: "init", Output: errMsg}
	}

	writeLog(activeLogWriter, "Deploying container...")
	updateStep("deploy", models.DeployStatusDeploying, "")

	var appSlug = "app"
	if job.AppSlug != "" {
		appSlug = job.AppSlug
	}
	var customDomain = job.CustomDomain
	var appPort = job.AppPort
	var oldContainerID string

	var resLimits docker.ContainerResourceLimits
	var healthcheckPath = "/"
	var healthcheckTimeout = 30 * time.Second
	var routeOpts caddy.RouteOptions

	if p.appRepo != nil {
		app, err := p.appRepo.GetByID(job.ApplicationID)
		if err == nil && app != nil {
			appSlug = app.Slug
			if app.CustomDomain != nil && *app.CustomDomain != "" {
				customDomain = app.CustomDomain
			}
			if appPort <= 0 {
				appPort = app.AppPort
			}
			if app.ContainerID != nil {
				oldContainerID = *app.ContainerID
			}
			if app.HealthcheckPath != nil && *app.HealthcheckPath != "" {
				healthcheckPath = *app.HealthcheckPath
			}
			if app.HealthcheckTimeout != nil && *app.HealthcheckTimeout > 0 {
				healthcheckTimeout = time.Duration(*app.HealthcheckTimeout) * time.Second
			}
			if app.BasicAuthUser != nil && *app.BasicAuthUser != "" && app.BasicAuthPassword != nil && *app.BasicAuthPassword != "" {
				routeOpts.BasicAuthUser = *app.BasicAuthUser
				routeOpts.BasicAuthPassword = *app.BasicAuthPassword
			}
			resLimits = docker.ContainerResourceLimits{
				CPULimit:          app.CPULimit,
				MemoryLimit:       app.MemoryLimit,
				MemoryReservation: app.MemoryReservation,
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

	// Blue-Green container naming: generate candidate container name with timestamp
	candidateName := fmt.Sprintf("%s-%d", targetDockerClient.GenerateContainerName("prod", appSlug, job.ApplicationID), time.Now().Unix())
	writeLog(activeLogWriter, fmt.Sprintf("Launching new candidate container (%s)...", candidateName))

	candidateContainerID, err := targetDockerClient.RunContainerWithCustomName(ctx, job.ApplicationID, candidateName, imageName, appPort, dockerEnvVars, resLimits)
	if err != nil {
		errMsg := fmt.Sprintf("failed to run candidate container: %v", err)
		writeLog(activeLogWriter, errMsg)
		updateStep("deploy", models.DeployStatusFailed, errMsg)
		return &DeployError{Step: "deploy", Cause: err, Output: errMsg}
	}

	writeLog(activeLogWriter, fmt.Sprintf("Candidate container started (ID: %s). Running healthcheck gate (%s, timeout %v)...", candidateContainerID, healthcheckPath, healthcheckTimeout))
	updateStep("healthcheck", models.DeployStatusDeploying, "")

	// Healthcheck gate: wait for container to answer on healthcheckPath or stay running
	if err := targetDockerClient.WaitForHealthCustom(ctx, candidateContainerID, appPort, healthcheckPath, healthcheckTimeout); err != nil {
		errMsg := fmt.Sprintf("Healthcheck failed for candidate container: %v. Rolling back...", err)
		writeLog(activeLogWriter, errMsg)
		_ = targetDockerClient.StopContainer(ctx, candidateContainerID, 5)
		_ = targetDockerClient.RemoveContainer(ctx, candidateContainerID)
		updateStep("healthcheck", models.DeployStatusFailed, errMsg)
		return &DeployError{Step: "healthcheck", Cause: err, Output: errMsg}
	}

	writeLog(activeLogWriter, "Candidate container is healthy! Proceeding with traffic switch...")

	// Atomic Caddy Route Update to the new healthy candidate container
	if p.caddyClient != nil && customDomain != nil && *customDomain != "" {
		writeLog(activeLogWriter, fmt.Sprintf("Switching Caddy route for %s -> %s:%d", *customDomain, candidateName, appPort))
		if err := p.caddyClient.AddRouteWithOptions(*customDomain, candidateName, appPort, routeOpts); err != nil {
			writeLog(activeLogWriter, fmt.Sprintf("Warning: failed to update Caddy route: %v", err))
		} else {
			writeLog(activeLogWriter, "Caddy route switched successfully without downtime")
		}
	}

	// Update DB record to point to the new running container
	if p.appRepo != nil {
		_ = p.appRepo.UpdateContainerAndStatus(job.ApplicationID, candidateContainerID, models.AppStatusRunning)
	}

	// Zero-Downtime Cleanup: Stop and remove old container only after candidate is healthy and live
	if oldContainerID != "" && oldContainerID != candidateContainerID {
		writeLog(activeLogWriter, fmt.Sprintf("Stopping previous container (%s)...", oldContainerID))
		_ = targetDockerClient.StopContainer(ctx, oldContainerID, 10)
		_ = targetDockerClient.RemoveContainer(ctx, oldContainerID)
		writeLog(activeLogWriter, "Previous container stopped and cleaned up")
	}

	writeLog(activeLogWriter, "Zero-downtime deployment completed successfully!")
	updateStep("done", models.DeployStatusSuccess, "")
	if p.deployRepo != nil {
		if imageName != "" {
			_ = p.deployRepo.UpdateImageName(job.DeploymentID, imageName)
		}
		_ = p.deployRepo.MarkFinished(job.DeploymentID)
	}

	// Retention policy: Cache 5 image sukses terakhir per aplikasi (seperti Coolify)
	p.pruneOldCachedImages(ctx, targetDockerClient, job.ApplicationID, imageName, activeLogWriter)

	return nil
}

// pruneOldCachedImages membersihkan image docker lama milik aplikasi dengan mempertahankan 5 image sukses terbaru.
func (p *Pipeline) pruneOldCachedImages(ctx context.Context, dockerClient *docker.Client, appID, currentImage string, logWriter logging.LogWriter) {
	if p.deployRepo == nil || dockerClient == nil {
		return
	}

	// Whitelist 5 image sukses terakhir dari DB
	keepImages, err := p.deployRepo.GetRecentSuccessfulImages(appID, 5)
	if err != nil {
		writeLog(logWriter, fmt.Sprintf("Warning: failed to fetch recent successful images for retention: %v", err))
		return
	}

	keepSet := make(map[string]bool)
	for _, img := range keepImages {
		keepSet[img] = true
	}
	if currentImage != "" {
		keepSet[currentImage] = true
	}

	// Cari semua image yang memiliki prefix labuh-<appID>:
	prefix := fmt.Sprintf("labuh-%s:", appID)
	localImages, err := dockerClient.ListImagesByPrefix(ctx, prefix)
	if err != nil {
		writeLog(logWriter, fmt.Sprintf("Warning: failed to list images for retention prune: %v", err))
		return
	}

	for _, localImg := range localImages {
		if !keepSet[localImg] {
			writeLog(logWriter, fmt.Sprintf("Pruning older cached image to free space: %s", localImg))
			if err := dockerClient.RemoveImage(ctx, localImg); err != nil {
				writeLog(logWriter, fmt.Sprintf("Notice: could not remove image %s (might be in use): %v", localImg, err))
			}
		}
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

func writeLog(file logging.LogWriter, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] %s\n", timestamp, message)
	_, _ = file.Write([]byte(line))
	_ = file.Sync()
}
