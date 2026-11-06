package settings

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

// CleanupScheduler mengelola background scheduler untuk otomatisasi Docker disk cleanup.
type CleanupScheduler struct {
	dockerClient *docker.Client
	settingRepo  *SettingRepo
	auditRepo    *AuditLogRepo
	cronParser   cron.Parser
	ticker       *time.Ticker
	stopCh       chan struct{}
	ctx          context.Context
	cancel       context.CancelFunc
	running      bool
	lastRunMin   int
	mu           sync.Mutex
}

func NewCleanupScheduler(dockerClient *docker.Client, settingRepo *SettingRepo, auditRepo *AuditLogRepo) *CleanupScheduler {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	ctx, cancel := context.WithCancel(context.Background())
	return &CleanupScheduler{
		dockerClient: dockerClient,
		settingRepo:  settingRepo,
		auditRepo:    auditRepo,
		cronParser:   parser,
		stopCh:       make(chan struct{}),
		ctx:          ctx,
		cancel:       cancel,
		lastRunMin:   -1,
	}
}

func (s *CleanupScheduler) SetDockerClient(c *docker.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dockerClient = c
}

// Start memulai ticker background per 1 menit untuk evaluasi jadwal cleanup.
func (s *CleanupScheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.ticker = time.NewTicker(1 * time.Minute)
	s.mu.Unlock()

	slog.Info("docker disk cleanup scheduler started")

	go func() {
		for {
			select {
			case <-s.stopCh:
				return
			case now := <-s.ticker.C:
				s.checkAndTrigger(now)
			}
		}
	}()
}

// Stop menghentikan background scheduler secara graceful.
func (s *CleanupScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	if s.ticker != nil {
		s.ticker.Stop()
	}
	close(s.stopCh)
	s.cancel()

	slog.Info("docker disk cleanup scheduler stopped")
}

func (s *CleanupScheduler) checkAndTrigger(now time.Time) {
	if s.settingRepo == nil || s.dockerClient == nil {
		return
	}

	enabled, err := s.settingRepo.Get(s.ctx, "docker_cleanup_enabled")
	if err != nil || enabled != "true" {
		return
	}

	scheduleExpr, err := s.settingRepo.Get(s.ctx, "docker_cleanup_schedule")
	if err != nil || scheduleExpr == "" {
		scheduleExpr = "0 0 * * *" // default midnight
	}

	schedule, err := s.cronParser.Parse(scheduleExpr)
	if err != nil {
		slog.Warn("cleanup scheduler: invalid cron expression", "expr", scheduleExpr, "error", err)
		return
	}

	currentMinute := now.Hour()*60 + now.Minute()
	if currentMinute == s.lastRunMin {
		return
	}

	// Cek apakah jadwal cocok untuk waktu saat ini
	prev := now.Add(-1 * time.Minute)
	next := schedule.Next(prev)
	if next.Year() == now.Year() && next.Month() == now.Month() && next.Day() == now.Day() &&
		next.Hour() == now.Hour() && next.Minute() == now.Minute() {
		s.lastRunMin = currentMinute
		slog.Info("cleanup scheduler: schedule matched, running automated cleanup", "time", now.Format(time.RFC3339))
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()
			_, _ = s.RunCleanup(ctx)
		}()
	}
}

// RunCleanup menjalankan pembersihan sistem docker dan memperbarui setting / audit log.
func (s *CleanupScheduler) RunCleanup(ctx context.Context) (*docker.PruneReport, error) {
	s.mu.Lock()
	client := s.dockerClient
	s.mu.Unlock()

	if client == nil {
		return nil, fmt.Errorf("docker client is not available")
	}

	report, err := client.PruneSystem(ctx, docker.DefaultPruneOptions())
	if err != nil {
		slog.Error("cleanup scheduler: docker prune failed", "error", err)
		return nil, err
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)

	if s.settingRepo != nil {
		_ = s.settingRepo.Set(ctx, "docker_cleanup_last_run", nowStr)
		_ = s.settingRepo.Set(ctx, "docker_cleanup_last_reclaimed", report.SpaceFormatted)
	}

	if s.auditRepo != nil {
		_, _ = s.auditRepo.Create(ctx, models.CreateAuditLogInput{
			Action:   "docker_cleanup",
			Resource: "system",
			UserAgent: fmt.Sprintf("Containers: %d, Images: %d, Space Reclaimed: %s",
				len(report.ContainersDeleted), len(report.ImagesDeleted), report.SpaceFormatted),
		})
	}

	slog.Info("cleanup scheduler: docker cleanup completed", "space_reclaimed", report.SpaceFormatted)
	return report, nil
}
