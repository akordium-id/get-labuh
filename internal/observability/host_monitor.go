package observability

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/akordium-id/get-labuh/internal/models"
)

type SettingProvider interface {
	Get(ctx context.Context, key string) (string, error)
}

type AlertNotifier interface {
	Dispatch(event models.NotificationEvent)
}

type AlertSeverity string

const (
	SeverityNormal   AlertSeverity = "normal"
	SeverityWarning  AlertSeverity = "warning"
	SeverityCritical AlertSeverity = "critical"
)

type HostMonitor struct {
	settingProvider SettingProvider
	notifier        AlertNotifier
	diskPath        string

	lastDiskSeverity  AlertSeverity
	lastRAMSeverity   AlertSeverity
	lastDiskAlertTime time.Time
	lastRAMAlertTime  time.Time

	ticker  *time.Ticker
	stopCh  chan struct{}
	running bool
	mu      sync.Mutex
}

func NewHostMonitor(settingProvider SettingProvider, notifier AlertNotifier, diskPath string) *HostMonitor {
	if diskPath == "" {
		diskPath = "/"
	}
	return &HostMonitor{
		settingProvider:  settingProvider,
		notifier:         notifier,
		diskPath:         diskPath,
		lastDiskSeverity: SeverityNormal,
		lastRAMSeverity:  SeverityNormal,
		stopCh:           make(chan struct{}),
	}
}

func (m *HostMonitor) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.ticker = time.NewTicker(60 * time.Second)
	m.mu.Unlock()

	slog.Info("host resource monitor daemon started")

	go func() {
		for {
			select {
			case <-m.stopCh:
				return
			case <-m.ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				m.CheckOnce(ctx)
				cancel()
			}
		}
	}()
}

func (m *HostMonitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}
	m.running = false
	if m.ticker != nil {
		m.ticker.Stop()
	}
	close(m.stopCh)
	slog.Info("host resource monitor daemon stopped")
}

// CheckOnce melakukan satu siklus pengecekan dan evaluasi threshold.
func (m *HostMonitor) CheckOnce(ctx context.Context) {
	if m.settingProvider != nil {
		enabled, err := m.settingProvider.Get(ctx, "host_monitoring_enabled")
		if err == nil && enabled == "false" {
			return
		}
	}

	stats := GetHostStats(m.diskPath)
	m.Evaluate(ctx, stats)
}

// Evaluate membandingkan stats terhadap ambang batas dan mengirim notifikasi jika diperlukan.
func (m *HostMonitor) Evaluate(ctx context.Context, stats HostStats) {
	m.mu.Lock()
	defer m.mu.Unlock()

	diskWarnPct := m.getFloatSetting(ctx, "host_disk_warning_pct", 80.0)
	diskCritPct := m.getFloatSetting(ctx, "host_disk_critical_pct", 90.0)
	ramWarnPct := m.getFloatSetting(ctx, "host_ram_warning_pct", 90.0)
	cooldownMins := m.getIntSetting(ctx, "host_alert_cooldown_minutes", 30)
	cooldownDuration := time.Duration(cooldownMins) * time.Minute

	now := time.Now()

	// 1. Evaluate Disk
	var currentDiskSev AlertSeverity
	if stats.DiskUsedPercent >= diskCritPct {
		currentDiskSev = SeverityCritical
	} else if stats.DiskUsedPercent >= diskWarnPct {
		currentDiskSev = SeverityWarning
	} else {
		currentDiskSev = SeverityNormal
	}

	if currentDiskSev != m.lastDiskSeverity {
		if currentDiskSev == SeverityCritical {
			m.dispatchDiskAlert(stats, SeverityCritical)
			m.lastDiskAlertTime = now
		} else if currentDiskSev == SeverityWarning && m.lastDiskSeverity == SeverityNormal {
			m.dispatchDiskAlert(stats, SeverityWarning)
			m.lastDiskAlertTime = now
		} else if currentDiskSev == SeverityNormal && (m.lastDiskSeverity == SeverityWarning || m.lastDiskSeverity == SeverityCritical) {
			m.dispatchDiskResolved(stats)
			m.lastDiskAlertTime = time.Time{}
		}
		m.lastDiskSeverity = currentDiskSev
	} else if currentDiskSev != SeverityNormal {
		// Cooldown check for persistent alerts
		if now.Sub(m.lastDiskAlertTime) >= cooldownDuration {
			m.dispatchDiskAlert(stats, currentDiskSev)
			m.lastDiskAlertTime = now
		}
	}

	// 2. Evaluate RAM
	var currentRAMSev AlertSeverity
	if stats.MemUsedPercent >= ramWarnPct {
		currentRAMSev = SeverityWarning
	} else {
		currentRAMSev = SeverityNormal
	}

	if currentRAMSev != m.lastRAMSeverity {
		if currentRAMSev == SeverityWarning {
			m.dispatchRAMAlert(stats)
			m.lastRAMAlertTime = now
		} else if currentRAMSev == SeverityNormal && m.lastRAMSeverity == SeverityWarning {
			m.dispatchRAMResolved(stats)
			m.lastRAMAlertTime = time.Time{}
		}
		m.lastRAMSeverity = currentRAMSev
	} else if currentRAMSev != SeverityNormal {
		if now.Sub(m.lastRAMAlertTime) >= cooldownDuration {
			m.dispatchRAMAlert(stats)
			m.lastRAMAlertTime = now
		}
	}
}

func (m *HostMonitor) dispatchDiskAlert(stats HostStats, sev AlertSeverity) {
	if m.notifier == nil {
		return
	}

	var status, eventType string
	if sev == SeverityCritical {
		status = "failed"
		eventType = models.EventServerAlertCritical
	} else {
		status = "warning"
		eventType = models.EventServerAlertWarning
	}

	title := fmt.Sprintf("⚠️ Host Storage Alert [%s]: %.1f%% Used", strings.ToUpper(string(sev)), stats.DiskUsedPercent)
	desc := fmt.Sprintf("Kapasitas disk host pada path `%s` telah mencapai %.1f%% (Sisa: %s dari total %s).",
		m.diskPath, stats.DiskUsedPercent, formatBytes(stats.DiskFree), formatBytes(stats.DiskTotal))

	event := models.NotificationEvent{
		Title:       title,
		Description: desc,
		Status:      status,
		EventType:   eventType,
		Fields: map[string]string{
			"Storage Path": m.diskPath,
			"Disk Used":    fmt.Sprintf("%.1f%%", stats.DiskUsedPercent),
			"Disk Free":    formatBytes(stats.DiskFree),
			"Disk Total":   formatBytes(stats.DiskTotal),
			"Severity":     string(sev),
		},
	}
	m.notifier.Dispatch(event)
}

func (m *HostMonitor) dispatchDiskResolved(stats HostStats) {
	if m.notifier == nil {
		return
	}

	event := models.NotificationEvent{
		Title:       "✅ Host Storage Alert Resolved",
		Description: fmt.Sprintf("Pemakaian disk host pada `%s` telah kembali normal di %.1f%% (Sisa: %s).", m.diskPath, stats.DiskUsedPercent, formatBytes(stats.DiskFree)),
		Status:      "success",
		EventType:   models.EventServerAlertResolved,
		Fields: map[string]string{
			"Storage Path": m.diskPath,
			"Disk Used":    fmt.Sprintf("%.1f%%", stats.DiskUsedPercent),
			"Disk Free":    formatBytes(stats.DiskFree),
		},
	}
	m.notifier.Dispatch(event)
}

func (m *HostMonitor) dispatchRAMAlert(stats HostStats) {
	if m.notifier == nil {
		return
	}

	event := models.NotificationEvent{
		Title:       fmt.Sprintf("⚠️ Host Memory Warning: %.1f%% Used", stats.MemUsedPercent),
		Description: fmt.Sprintf("Konsumsi RAM host telah mencapai %.1f%% (Sisa: %s dari total %s).", stats.MemUsedPercent, formatBytes(stats.MemFree), formatBytes(stats.MemTotal)),
		Status:      "warning",
		EventType:   models.EventServerAlertWarning,
		Fields: map[string]string{
			"Memory Used":  fmt.Sprintf("%.1f%%", stats.MemUsedPercent),
			"Memory Free":  formatBytes(stats.MemFree),
			"Memory Total": formatBytes(stats.MemTotal),
		},
	}
	m.notifier.Dispatch(event)
}

func (m *HostMonitor) dispatchRAMResolved(stats HostStats) {
	if m.notifier == nil {
		return
	}

	event := models.NotificationEvent{
		Title:       "✅ Host Memory Warning Resolved",
		Description: fmt.Sprintf("Pemakaian RAM host telah kembali normal di %.1f%%.", stats.MemUsedPercent),
		Status:      "success",
		EventType:   models.EventServerAlertResolved,
		Fields: map[string]string{
			"Memory Used": fmt.Sprintf("%.1f%%", stats.MemUsedPercent),
			"Memory Free": formatBytes(stats.MemFree),
		},
	}
	m.notifier.Dispatch(event)
}

func (m *HostMonitor) getFloatSetting(ctx context.Context, key string, defaultVal float64) float64 {
	if m.settingProvider == nil {
		return defaultVal
	}
	valStr, err := m.settingProvider.Get(ctx, key)
	if err != nil || valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return defaultVal
	}
	return val
}

func (m *HostMonitor) getIntSetting(ctx context.Context, key string, defaultVal int) int {
	if m.settingProvider == nil {
		return defaultVal
	}
	valStr, err := m.settingProvider.Get(ctx, key)
	if err != nil || valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func formatBytes(b uint64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	if b < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(b)/1024)
	}
	if b < 1024*1024*1024 {
		return fmt.Sprintf("%.1f MB", float64(b)/(1024*1024))
	}
	return fmt.Sprintf("%.1f GB", float64(b)/(1024*1024*1024))
}
