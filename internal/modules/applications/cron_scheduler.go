package applications

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/models"
)

type CronScheduler struct {
	cronRepo     *CronJobRepo
	appRepo      *Repo
	envVarRepo   *EnvVarRepo
	dockerClient *docker.Client
	cronParser   cron.Parser
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	runningJobs  sync.Map
}

func NewCronScheduler(cronRepo *CronJobRepo, appRepo *Repo, envVarRepo *EnvVarRepo, dockerClient *docker.Client) *CronScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	return &CronScheduler{
		cronRepo:     cronRepo,
		appRepo:      appRepo,
		envVarRepo:   envVarRepo,
		dockerClient: dockerClient,
		cronParser:   parser,
		ctx:          ctx,
		cancel:       cancel,
	}
}

func (s *CronScheduler) Start() {
	s.wg.Go(func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				return
			case now := <-ticker.C:
				s.checkAndTriggerJobs(now)
			}
		}
	})
	slog.Info("application cron scheduler started")
}

func (s *CronScheduler) Stop() {
	s.cancel()
	s.wg.Wait()
	slog.Info("application cron scheduler stopped")
}

func (s *CronScheduler) checkAndTriggerJobs(now time.Time) {
	if s.cronRepo == nil {
		return
	}

	jobs, err := s.cronRepo.ListActiveJobsWithContext(s.ctx)
	if err != nil {
		slog.Error("cron scheduler: failed to list active cron jobs", "error", err)
		return
	}

	for _, job := range jobs {
		schedule, err := s.cronParser.Parse(job.CronExpression)
		if err != nil {
			slog.Warn("cron scheduler: invalid cron expression", "job_id", job.ID, "expr", job.CronExpression, "error", err)
			continue
		}

		// Hitung previous match: kurangi 1 menit lalu lihat apakah next match berada di menit yang sama dengan now
		prevMinute := now.Add(-1 * time.Minute).Truncate(time.Minute)
		nextMatch := schedule.Next(prevMinute).Truncate(time.Minute)

		if nextMatch.Equal(now.Truncate(time.Minute)) {
			go s.RunJob(s.ctx, job.ID)
		}
	}
}

// RunJob mengeksekusi satu cron job di dalam container aplikasi secara asinkron atau manual
func (s *CronScheduler) RunJob(ctx context.Context, jobID string) (*models.ApplicationCronJob, error) {
	if _, loaded := s.runningJobs.LoadOrStore(jobID, true); loaded {
		return nil, fmt.Errorf("job is already running")
	}
	defer s.runningJobs.Delete(jobID)

	job, err := s.cronRepo.GetByIDWithContext(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("cron job not found: %w", err)
	}

	app, err := s.appRepo.GetByIDWithContext(ctx, job.ApplicationID)
	if err != nil {
		_ = s.cronRepo.UpdateExecutionWithContext(ctx, job.ID, models.CronJobStatusFailed, "Application not found: "+err.Error())
		return nil, fmt.Errorf("application not found: %w", err)
	}

	if app.ContainerID == nil || *app.ContainerID == "" {
		errMsg := "Application container is not running (no container ID)"
		_ = s.cronRepo.UpdateExecutionWithContext(ctx, job.ID, models.CronJobStatusFailed, errMsg)
		return nil, fmt.Errorf("%s", errMsg)
	}

	// Cek apakah container benar-benar running
	if s.dockerClient != nil {
		running, err := s.dockerClient.IsContainerRunning(ctx, *app.ContainerID)
		if err != nil || !running {
			errMsg := fmt.Sprintf("Application container %s is stopped or unhealthy", *app.ContainerID)
			_ = s.cronRepo.UpdateExecutionWithContext(ctx, job.ID, models.CronJobStatusFailed, errMsg)
			return nil, fmt.Errorf("%s", errMsg)
		}
	}

	// Kumpulkan environment variables aplikasi
	var envList []string
	if s.envVarRepo != nil {
		if envVars, err := s.envVarRepo.GetByApplicationIDWithContext(ctx, app.ID); err == nil {
			for _, ev := range envVars {
				if ev != nil {
					envList = append(envList, fmt.Sprintf("%s=%s", ev.Key, ev.Value))
				}
			}
		}
	}

	execCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	slog.Info("executing application cron job", "job_id", job.ID, "app_id", app.ID, "command", job.Command)
	output, err := s.dockerClient.ExecCommand(execCtx, *app.ContainerID, job.Command, envList)
	status := models.CronJobStatusSuccess
	if err != nil {
		status = models.CronJobStatusFailed
		if output == "" {
			output = err.Error()
		}
		slog.Warn("cron job execution failed", "job_id", job.ID, "error", err)
	}

	if err := s.cronRepo.UpdateExecutionWithContext(ctx, job.ID, status, output); err != nil {
		slog.Error("failed to record cron job execution status", "error", err)
	}

	return s.cronRepo.GetByIDWithContext(ctx, job.ID)
}
