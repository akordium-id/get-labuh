package observability

import (
	"context"
	stdtesting "testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/akordium-id/get-labuh/internal/models"
)

type mockSettingProvider struct {
	settings map[string]string
}

func (m *mockSettingProvider) Get(ctx context.Context, key string) (string, error) {
	if val, ok := m.settings[key]; ok {
		return val, nil
	}
	return "", nil
}

type mockAlertNotifier struct {
	events []models.NotificationEvent
}

func (m *mockAlertNotifier) Dispatch(event models.NotificationEvent) {
	m.events = append(m.events, event)
}

func TestHostMonitor_Evaluate_DiskStateMachine(t *stdtesting.T) {
	settings := &mockSettingProvider{
		settings: map[string]string{
			"host_monitoring_enabled":     "true",
			"host_disk_warning_pct":       "80.0",
			"host_disk_critical_pct":      "90.0",
			"host_ram_warning_pct":        "85.0",
			"host_alert_cooldown_minutes": "30",
		},
	}
	notifier := &mockAlertNotifier{}
	monitor := NewHostMonitor(settings, notifier, "/")

	ctx := context.Background()

	// 1. Initial Normal state (Disk 50%) -> no alerts
	statsNormal := HostStats{
		DiskTotal:       100 * 1024 * 1024 * 1024,
		DiskUsed:        50 * 1024 * 1024 * 1024,
		DiskFree:        50 * 1024 * 1024 * 1024,
		DiskUsedPercent: 50.0,
		MemTotal:        16 * 1024 * 1024 * 1024,
		MemUsedPercent:  40.0,
	}
	monitor.Evaluate(ctx, statsNormal)
	assert.Empty(t, notifier.events)

	// 2. Disk crosses Warning threshold (Disk 82%) -> 1 Warning alert
	statsWarning := statsNormal
	statsWarning.DiskUsedPercent = 82.0
	monitor.Evaluate(ctx, statsWarning)
	require.Len(t, notifier.events, 1)
	assert.Equal(t, models.EventServerAlertWarning, notifier.events[0].EventType)
	assert.Equal(t, "warning", notifier.events[0].Status)

	// 3. Disk stays at 82% immediately -> suppressed by cooldown
	monitor.Evaluate(ctx, statsWarning)
	assert.Len(t, notifier.events, 1, "Immediate repeat alert should be suppressed by cooldown")

	// 4. Disk escalates to Critical threshold (Disk 92%) -> Escalation alert fires immediately
	statsCritical := statsNormal
	statsCritical.DiskUsedPercent = 92.0
	monitor.Evaluate(ctx, statsCritical)
	require.Len(t, notifier.events, 2)
	assert.Equal(t, models.EventServerAlertCritical, notifier.events[1].EventType)
	assert.Equal(t, "failed", notifier.events[1].Status)

	// 5. Disk de-escalates to Warning threshold (Disk 85%) -> Warning alert fires
	statsDeescalated := statsNormal
	statsDeescalated.DiskUsedPercent = 85.0
	monitor.Evaluate(ctx, statsDeescalated)
	require.Len(t, notifier.events, 3)
	assert.Equal(t, models.EventServerAlertWarning, notifier.events[2].EventType)
	assert.Equal(t, "warning", notifier.events[2].Status)

	// 6. Disk drops back to Normal (Disk 60%) -> Resolved alert fires
	statsRecovered := statsNormal
	statsRecovered.DiskUsedPercent = 60.0
	monitor.Evaluate(ctx, statsRecovered)
	require.Len(t, notifier.events, 4)
	assert.Equal(t, models.EventServerAlertResolved, notifier.events[3].EventType)
	assert.Equal(t, "success", notifier.events[3].Status)
}

func TestHostMonitor_Evaluate_RAMAlert(t *stdtesting.T) {
	settings := &mockSettingProvider{
		settings: map[string]string{
			"host_monitoring_enabled":     "true",
			"host_disk_warning_pct":       "80.0",
			"host_disk_critical_pct":      "90.0",
			"host_ram_warning_pct":        "85.0",
			"host_alert_cooldown_minutes": "30",
		},
	}
	notifier := &mockAlertNotifier{}
	monitor := NewHostMonitor(settings, notifier, "/")

	ctx := context.Background()

	// RAM crosses warning (88%)
	stats := HostStats{
		DiskUsedPercent: 40.0,
		MemTotal:        16 * 1024 * 1024 * 1024,
		MemUsedPercent:  88.0,
	}
	monitor.Evaluate(ctx, stats)
	require.Len(t, notifier.events, 1)
	assert.Equal(t, models.EventServerAlertWarning, notifier.events[0].EventType)
	assert.Contains(t, notifier.events[0].Title, "Host Memory Warning")

	// RAM recovers (50%)
	stats.MemUsedPercent = 50.0
	monitor.Evaluate(ctx, stats)
	require.Len(t, notifier.events, 2)
	assert.Equal(t, models.EventServerAlertResolved, notifier.events[1].EventType)
}

func TestHostMonitor_StartStop(t *stdtesting.T) {
	monitor := NewHostMonitor(nil, nil, "/")
	monitor.Start()
	// starting again shouldn't panic
	monitor.Start()
	time.Sleep(10 * time.Millisecond)
	monitor.Stop()
	// stopping again shouldn't panic
	monitor.Stop()

	// restarting after stop should work cleanly without panic or leaking goroutine
	monitor.Start()
	time.Sleep(10 * time.Millisecond)
	monitor.Stop()
}
