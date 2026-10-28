package backup

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type BackupScheduler struct {
	runner *DatabaseBackupRunner
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewBackupScheduler(runner *DatabaseBackupRunner) *BackupScheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &BackupScheduler{
		runner: runner,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *BackupScheduler) Start() {
	s.wg.Go(func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-s.ctx.Done():
				return
			case now := <-ticker.C:
				s.checkAndRunBackups(now)
			}
		}
	})
	slog.Info("database backup scheduler started")
}

func (s *BackupScheduler) Stop() {
	s.cancel()
	s.wg.Wait()
	slog.Info("database backup scheduler stopped")
}

func (s *BackupScheduler) checkAndRunBackups(now time.Time) {
	configs, err := s.runner.backupRepo.ListActiveConfigs()
	if err != nil {
		slog.Error("failed to list active backup configs", "error", err)
		return
	}

	for _, cfg := range configs {
		if shouldRunNow(cfg.CronExpression, now) {
			go func(dbID string) {
				slog.Info("triggering scheduled database backup", "database_id", dbID)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
				defer cancel()
				if _, err := s.runner.RunBackup(ctx, dbID); err != nil {
					slog.Error("scheduled database backup failed", "error", err, "database_id", dbID)
				}
			}(cfg.DatabaseID)
		}
	}
}

// shouldRunNow mengevaluasi apakah waktu saat ini cocok dengan cron expression sederhana
// Format standard: "minute hour * * *" atau preset "@daily" (0 2 * * * = jam 02:00 UTC/lokal)
func shouldRunNow(expr string, now time.Time) bool {
	if expr == "@daily" || expr == "@midnight" || expr == "0 2 * * *" {
		return now.Hour() == 2 && now.Minute() == 0
	}
	if expr == "@hourly" {
		return now.Minute() == 0
	}
	if expr == "0 0 * * *" {
		return now.Hour() == 0 && now.Minute() == 0
	}
	return false
}
