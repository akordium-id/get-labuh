package worker

import (
	"context"
	"sync"

	"github.com/akordium-id/get-labuh/internal/caddy"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/deploy"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
)

type DeployWorker struct {
	jobQueue chan models.DeploymentJob
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewDeployWorker(bufferSize int) *DeployWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &DeployWorker{
		jobQueue: make(chan models.DeploymentJob, bufferSize),
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (w *DeployWorker) Start(dockerClient *docker.Client, caddyClient *caddy.Client, settingRepo *repo.SettingRepo, appRepo *repo.ApplicationRepo, deployRepo *repo.DeploymentRepo) {
	pipeline := deploy.NewPipeline(dockerClient, caddyClient, settingRepo, appRepo, deployRepo)

	w.wg.Go(func() {
		for {
			select {
			case job, ok := <-w.jobQueue:
				if !ok {
					return
				}
				_ = job
				_ = pipeline
			case <-w.ctx.Done():
				return
			}
		}
	})
}

func (w *DeployWorker) Enqueue(job models.DeploymentJob) {
	select {
	case w.jobQueue <- job:
	default:
	}
}

func (w *DeployWorker) Stop() {
	w.cancel()
	close(w.jobQueue)
	w.wg.Wait()
}
