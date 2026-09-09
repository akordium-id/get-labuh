package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/akordium-id/get-labuh/internal/caddy"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/deploy"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
)

type DeployWorker struct {
	jobQueue     chan models.DeploymentJob
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	semaphore    chan struct{}
	maxConcurrent int
	workerCount  int
}

func NewDeployWorker(bufferSize int) *DeployWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &DeployWorker{
		jobQueue:      make(chan models.DeploymentJob, bufferSize),
		ctx:           ctx,
		cancel:        cancel,
		maxConcurrent: 3,
		workerCount:   1,
	}
}

func (w *DeployWorker) SetMaxConcurrent(max int) {
	if max > 0 {
		w.maxConcurrent = max
	}
	w.semaphore = make(chan struct{}, w.maxConcurrent)
}

func (w *DeployWorker) SetWorkerCount(count int) {
	if count > 0 {
		w.workerCount = count
	}
}

func (w *DeployWorker) Start(dockerClient *docker.Client, caddyClient *caddy.Client, settingRepo *repo.SettingRepo, appRepo *repo.ApplicationRepo, deployRepo *repo.DeploymentRepo) {
	pipeline := deploy.NewPipeline(dockerClient, caddyClient, settingRepo, appRepo, deployRepo)

	if w.semaphore == nil {
		w.semaphore = make(chan struct{}, w.maxConcurrent)
	}

	for i := 0; i < w.workerCount; i++ {
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			for {
				select {
				case job, ok := <-w.jobQueue:
					if !ok {
						return
					}
					w.semaphore <- struct{}{}
				jobCtx, cancel := context.WithTimeout(w.ctx, 30*time.Minute)
				deployJob := deploy.DeploymentJob{
					DeploymentID:   job.DeploymentID,
					ApplicationID:  job.ApplicationID,
					SourceType:     job.SourceType,
					RepositoryURL:  job.RepositoryURL,
					Branch:         job.Branch,
					DockerfilePath: job.DockerfilePath,
					BuildPath:      job.BuildPath,
					DockerImage:    job.DockerImage,
					AppPort:        job.AppPort,
					EnvVars:        job.EnvVars,
					LogPath:        job.LogPath,
					Priority:       job.Priority,
				}
				err := pipeline.Execute(jobCtx, deployJob)
				cancel()
				<-w.semaphore
				if err != nil {
					slog.Error("deployment job failed", "error", err, "deployment_id", job.DeploymentID)
				}
				case <-w.ctx.Done():
					return
				}
			}
		}()
	}

	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = docker.CleanOldCache(24 * time.Hour)
			case <-w.ctx.Done():
				return
			}
		}
	}()
}

func (w *DeployWorker) Enqueue(job models.DeploymentJob) error {
	select {
	case w.jobQueue <- job:
		return nil
	case <-w.ctx.Done():
		return w.ctx.Err()
	case <-time.After(5 * time.Second):
		return fmt.Errorf("deploy queue is full, could not enqueue job %s", job.DeploymentID)
	}
}

func (w *DeployWorker) EnqueuePriority(job models.DeploymentJob) error {
	return w.Enqueue(job)
}

func BuildDeploymentJob(app *models.Application, deploymentID string, envVars []*models.AppEnvVar) models.DeploymentJob {
	var envList []models.AppEnvVar
	for _, ev := range envVars {
		if ev != nil {
			envList = append(envList, *ev)
		}
	}
	var repoURL, branch, dockerfilePath, buildPath, dockerImage string
	if app.RepositoryURL != nil {
		repoURL = *app.RepositoryURL
	}
	if app.Branch != nil {
		branch = *app.Branch
	}
	if app.DockerfilePath != nil {
		dockerfilePath = *app.DockerfilePath
	}
	if app.BuildPath != nil {
		buildPath = *app.BuildPath
	}
	if app.DockerImage != nil {
		dockerImage = *app.DockerImage
	}

	return models.DeploymentJob{
		DeploymentID:   deploymentID,
		ApplicationID:  app.ID,
		SourceType:     string(app.SourceType),
		RepositoryURL:  repoURL,
		Branch:         branch,
		DockerfilePath: dockerfilePath,
		BuildPath:      buildPath,
		DockerImage:    dockerImage,
		AppPort:        app.AppPort,
		EnvVars:        envList,
		LogPath:        "/tmp/labuh-logs/" + deploymentID + ".log",
		Priority:       1,
	}
}

func (w *DeployWorker) Stop() {
	w.cancel()
	close(w.jobQueue)
	w.wg.Wait()
}

type jobPriority struct {
	job models.DeploymentJob
	idx int
}

func sortJobsByPriority(jobs []models.DeploymentJob) {
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].Priority > jobs[j].Priority
	})
}
