package runner

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/akordium-id/get-labuh/internal/caddy"
	"github.com/akordium-id/get-labuh/internal/deploy"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
)

type Executor struct {
	dockerClient *docker.Client
	caddyClient  *caddy.Client
	pipeline     *deploy.Pipeline
}

func NewExecutor(dockerClient *docker.Client, caddyClient *caddy.Client) *Executor {
	pipeline := deploy.NewPipeline(dockerClient, caddyClient, nil, nil, nil)
	return &Executor{
		dockerClient: dockerClient,
		caddyClient:  caddyClient,
		pipeline:     pipeline,
	}
}

func (e *Executor) ExecuteDeploy(
	ctx context.Context,
	payload DeployJobPayload,
	logWriter io.Writer,
	statusReporter func(step string, status string, err error),
) error {
	logPath := payload.LogPath
	if logPath == "" {
		logPath = filepath.Join("/tmp/labuh-logs", fmt.Sprintf("%s.log", payload.JobID))
	}

	var envVars []models.AppEnvVar
	for k, v := range payload.EnvVars {
		envVars = append(envVars, models.AppEnvVar{
			Key:   k,
			Value: v,
		})
	}

	var customDomain *string
	if len(payload.Domains) > 0 {
		d := payload.Domains[0]
		customDomain = &d
	}

	job := deploy.DeploymentJob{
		DeploymentID:   payload.JobID,
		ApplicationID:  payload.ApplicationID,
		SourceType:     payload.SourceType,
		RepositoryURL:  payload.RepositoryURL,
		Branch:         payload.Branch,
		DockerfilePath: payload.DockerfilePath,
		BuildPath:      payload.BuildPath,
		DockerImage:    payload.DockerImage,
		AppPort:        payload.AppPort,
		EnvVars:        envVars,
		LogPath:        logPath,
		Priority:       payload.Priority,
		AppSlug:        payload.ApplicationID,
		CustomDomain:   customDomain,
		ExtraLogWriter: logWriter,
		StatusCallback: statusReporter,
	}

	return e.pipeline.Execute(ctx, job)
}

func (e *Executor) ExecuteContainerAction(ctx context.Context, payload ContainerActionPayload) error {
	targetID := payload.ContainerID
	if targetID == "" {
		targetID = payload.ApplicationID
	}

	switch payload.Action {
	case "start":
		return e.dockerClient.StartContainer(ctx, targetID)
	case "stop":
		return e.dockerClient.StopContainer(ctx, targetID, 10)
	case "restart":
		return e.dockerClient.RestartContainer(ctx, targetID, 10)
	default:
		return fmt.Errorf("unsupported container action: %s", payload.Action)
	}
}

func (e *Executor) ExecuteProxySync(ctx context.Context, payload ProxySyncPayload) error {
	if e.caddyClient == nil {
		return nil
	}
	containerName := e.dockerClient.GenerateContainerName("prod", payload.ApplicationID, payload.ApplicationID)
	for _, domain := range payload.Domains {
		if err := e.caddyClient.AddRoute(domain, containerName, payload.TargetPort); err != nil {
			return err
		}
	}
	return nil
}
