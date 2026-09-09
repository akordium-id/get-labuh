package worker

import (
	"context"
	"math"
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
	jobQueue           chan *models.DeploymentJob
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 sync.WaitGroup
	maxConcurrent      int
	semaphore          chan struct{}
	workerWg           sync.WaitGroup
}

func NewDeployWorker(bufferSize, maxConcurrent int) *DeployWorker {
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	if bufferSize <= 0 {
		bufferSize = 10
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &DeployWorker{
		jobQueue:      make(chan *models.DeploymentJob, bufferSize),
		ctx:           ctx,
		cancel:        cancel,
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
	}
}

func (w *DeployWorker) Start(dockerClient *docker.Client, caddyClient *caddy.Client, settingRepo *repo.SettingRepo, appRepo *repo.ApplicationRepo, deployRepo *repo.DeploymentRepo) {
	pipeline := deploy.NewPipeline(dockerClient, caddyClient, settingRepo, appRepo, deployRepo)

	w.wg.Go(func() {
		w.runWorkers(pipeline, 1)
	})

	w.wg.Go(func() {
		w.drainOnShutdown()
	})
}

func (w *DeployWorker) runWorkers(pipeline *deploy.Pipeline, workerCount int) {
	for i := 0; i < workerCount; i++ {
		w.workerWg.Add(1)
		go func() {
			defer w.workerWg.Done()
			for {
				select {
				case job, ok := <-w.jobQueue:
					if !ok {
						return
					}
					w.processJob(w.ctx, pipeline, job)
				case <-w.ctx.Done():
					return
				}
			}
		}()
	}
}

func (w *DeployWorker) processJob(ctx context.Context, pipeline *deploy.Pipeline, job *models.DeploymentJob) {
	w.semaphore <- struct{}{}
	defer func() { <-w.semaphore }()

	timeout := 30 * time.Minute
	stepCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

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

	_ = stepCtx
	_ = pipeline
	_ = deployJob
}

func (w *DeployWorker) drainOnShutdown() {
	<-w.ctx.Done()
	w.workerWg.Wait()
}

func (w *DeployWorker) Enqueue(job *models.DeploymentJob) {
	if job == nil {
		return
	}
	select {
	case w.jobQueue <- job:
	default:
	}
}

func (w *DeployWorker) Stop() {
	w.cancel()
	close(w.jobQueue)
	w.wg.Wait()
	w.workerWg.Wait()
}

func (w *DeployWorker) Queued() int {
	return len(w.jobQueue)
}

func (w *DeployWorker) Running() int {
	return len(w.semaphore)
}

type jobItem struct {
	job    *models.DeploymentJob
	index  int
}

type priorityQueue []*jobItem

func (pq priorityQueue) Len() int { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool {
	if pq[i].job.Priority != pq[j].job.Priority {
		return pq[i].job.Priority > pq[j].job.Priority
	}
	return pq[i].index < pq[j].index
}
func (pq priorityQueue) Swap(i, j int) { pq[i], pq[j] = pq[j], pq[i] }

func (pq *priorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*jobItem))
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[:n-1]
	return item
}

type jobScheduler struct {
	jobs []*models.DeploymentJob
}

func newJobScheduler() *jobScheduler {
	return &jobScheduler{jobs: make([]*models.DeploymentJob, 0)}
}

func (s *jobScheduler) Add(job *models.DeploymentJob) {
	if job == nil {
		return
	}
	s.jobs = append(s.jobs, job)
}

func (s *jobScheduler) Sort() {
	sort.SliceStable(s.jobs, func(i, j int) bool {
		if s.jobs[i].Priority != s.jobs[j].Priority {
			return s.jobs[i].Priority > s.jobs[j].Priority
		}
		return false
	})
}

func (s *jobScheduler) Drain() []*models.DeploymentJob {
	jobs := s.jobs
	s.jobs = make([]*models.DeploymentJob, 0)
	return jobs
}

func calculateTimeoutMultiplier(base time.Duration, complexity int) time.Duration {
	if complexity <= 0 {
		return base
	}
	factor := math.Pow(1.2, float64(complexity-1))
	return time.Duration(float64(base) * factor)
}
