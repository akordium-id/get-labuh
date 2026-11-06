package worker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/akordium-id/get-labuh/internal/caddy"
	"github.com/akordium-id/get-labuh/internal/database/repo"
	"github.com/akordium-id/get-labuh/internal/deploy"
	"github.com/akordium-id/get-labuh/internal/docker"
	"github.com/akordium-id/get-labuh/internal/encryption"
	"github.com/akordium-id/get-labuh/internal/gitbot"
	"github.com/akordium-id/get-labuh/internal/models"
)

type EventNotifier interface {
	Dispatch(event models.NotificationEvent)
}

type DeployWorker struct {
	jobQueue      chan models.DeploymentJob
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	semaphore     chan struct{}
	maxConcurrent int
	workerCount   int
	notifier      EventNotifier
	envRepo       *repo.ProjectRepo
	gitbotClient  *gitbot.Client
	stopOnce      sync.Once
}

func NewDeployWorker(bufferSize int) *DeployWorker {
	ctx, cancel := context.WithCancel(context.Background())
	return &DeployWorker{
		jobQueue:      make(chan models.DeploymentJob, bufferSize),
		ctx:           ctx,
		cancel:        cancel,
		maxConcurrent: 3,
		workerCount:   1,
		gitbotClient:  gitbot.NewClient(),
	}
}

// SetGitBotClient menyetel gitbot client khusus (berguna untuk testing).
func (w *DeployWorker) SetGitBotClient(c *gitbot.Client) {
	w.gitbotClient = c
}

func (w *DeployWorker) SetNotifier(n EventNotifier, envRepo *repo.ProjectRepo) {
	w.notifier = n
	w.envRepo = envRepo
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

func (w *DeployWorker) Start(dockerClient *docker.Client, caddyClient *caddy.Client, settingRepo *repo.SettingRepo, appRepo *repo.ApplicationRepo, deployRepo *repo.DeploymentRepo, dockerPool *docker.ClientPool) {
	pipeline := deploy.NewPipeline(dockerClient, caddyClient, settingRepo, appRepo, deployRepo)
	if dockerPool != nil {
		pipeline.SetDockerPool(dockerPool)
	}

	if w.semaphore == nil {
		w.semaphore = make(chan struct{}, w.maxConcurrent)
	}

	for i := 0; i < w.workerCount; i++ {
		w.wg.Go(func() {
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
						ServerID:       job.ServerID,
					}
					err := pipeline.Execute(jobCtx, deployJob)
					cancel()
					<-w.semaphore

					var deployedApp *models.Application
					var appName string
					var projectID *string
					if app, getErr := appRepo.GetByID(job.ApplicationID); getErr == nil && app != nil {
						deployedApp = app
						appName = app.Name
						if w.envRepo != nil {
							if env, envErr := w.envRepo.GetEnvironmentByID(app.EnvironmentID); envErr == nil && env != nil {
								projectID = &env.ProjectID
							}
						}
					}

					if err != nil {
						slog.Error("deployment job failed", "error", err, "deployment_id", job.DeploymentID)
						if w.notifier != nil {
							w.notifier.Dispatch(models.NotificationEvent{
								Title:       fmt.Sprintf("Deploy Gagal: %s", appName),
								Description: fmt.Sprintf("Deployment untuk aplikasi %s gagal: %v", appName, err),
								Status:      "failure",
								EventType:   "deploy_failure",
								ProjectID:   projectID,
								Fields: map[string]string{
									"Application": appName,
									"Branch":      job.Branch,
									"Error":       err.Error(),
								},
							})
						}
						w.dispatchPRPreviewComment(deployedApp, appRepo, deployRepo, settingRepo, job, err)
					} else {
						if w.notifier != nil {
							w.notifier.Dispatch(models.NotificationEvent{
								Title:       fmt.Sprintf("Deploy Sukses: %s", appName),
								Description: fmt.Sprintf("Aplikasi %s berhasil di-deploy dan traffic aktif.", appName),
								Status:      "success",
								EventType:   "deploy_success",
								ProjectID:   projectID,
								Fields: map[string]string{
									"Application":   appName,
									"Branch":        job.Branch,
									"Deployment ID": job.DeploymentID,
								},
							})
						}
						w.dispatchPRPreviewComment(deployedApp, appRepo, deployRepo, settingRepo, job, nil)
					}
				case <-w.ctx.Done():
					return
				}
			}
		})
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
		ServerID:       app.ServerID,
	}
}

func (w *DeployWorker) Stop() {
	w.stopOnce.Do(func() {
		w.cancel()
		close(w.jobQueue)
		w.wg.Wait()
	})
}

func (w *DeployWorker) dispatchPRPreviewComment(app *models.Application, appRepo *repo.ApplicationRepo, deployRepo *repo.DeploymentRepo, settingRepo *repo.SettingRepo, job models.DeploymentJob, deployErr error) {
	if app == nil || !app.IsPreview || app.PRNumber == nil || app.ParentAppID == nil {
		return
	}

	parentApp, err := appRepo.GetByID(*app.ParentAppID)
	if err != nil || parentApp == nil || parentApp.RepositoryURL == nil || *parentApp.RepositoryURL == "" {
		return
	}

	provider, _, _, err := gitbot.ParseRepo(*parentApp.RepositoryURL)
	if err != nil {
		return
	}

	var token string
	if parentApp.GitToken != nil && *parentApp.GitToken != "" {
		if decrypted, err := encryption.Decrypt(*parentApp.GitToken); err == nil && decrypted != "" {
			token = decrypted
		} else {
			token = *parentApp.GitToken
		}
	}

	if token == "" && settingRepo != nil {
		if provider == "github" {
			token, _ = settingRepo.Get("github_bot_token")
		} else if provider == "gitlab" {
			token, _ = settingRepo.Get("gitlab_bot_token")
		}
	}

	if token == "" {
		return
	}

	var params gitbot.CommentParams
	if deployRepo != nil && job.DeploymentID != "" {
		if dep, depErr := deployRepo.GetByID(job.DeploymentID); depErr == nil && dep != nil && dep.CommitHash != nil {
			params.CommitHash = *dep.CommitHash
		}
	}
	if deployErr != nil {
		params.Status = gitbot.StatusFailed
		params.ErrorMsg = deployErr.Error()
	} else {
		params.Status = gitbot.StatusLive
		if app.CustomDomain != nil && *app.CustomDomain != "" {
			params.PreviewURL = "http://" + *app.CustomDomain
		}
	}

	body := gitbot.FormatComment(params)
	if w.gitbotClient == nil {
		w.gitbotClient = gitbot.NewClient()
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var postErr error
		if provider == "github" {
			postErr = w.gitbotClient.PostOrUpdateGitHubComment(ctx, *parentApp.RepositoryURL, *app.PRNumber, token, body)
		} else if provider == "gitlab" {
			postErr = w.gitbotClient.PostOrUpdateGitLabNote(ctx, *parentApp.RepositoryURL, *app.PRNumber, token, body)
		}

		if postErr != nil {
			slog.Warn("worker failed to update pr preview comment", "provider", provider, "pr_number", *app.PRNumber, "error", postErr)
		}
	}()
}
