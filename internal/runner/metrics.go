package runner

import (
	"bufio"
	"context"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/akordium-id/get-labuh/internal/docker"
)

type MetricsCollector struct {
	dockerClient *docker.Client
	mu           sync.Mutex
	lastCPUTotal uint64
	lastCPUIdle  uint64
}

func NewMetricsCollector(dockerClient *docker.Client) *MetricsCollector {
	return &MetricsCollector{
		dockerClient: dockerClient,
	}
}

func (m *MetricsCollector) Collect(ctx context.Context) HeartbeatPayload {
	hb := HeartbeatPayload{
		Timestamp: time.Now().Unix(),
	}

	hb.CPUPercent = m.collectCPU()
	hb.MemoryUsedMB, hb.MemoryTotalMB = m.collectMemory()
	hb.DiskUsedPercent = m.collectDisk()

	if m.dockerClient != nil {
		if ver, err := m.dockerClient.GetServerVersion(ctx); err == nil {
			hb.DockerVersion = ver
		}
		if count, err := m.dockerClient.CountRunningContainers(ctx); err == nil {
			hb.ActiveContainers = count
		}
	}

	return hb
}

func (m *MetricsCollector) collectCPU() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()

	if runtime.GOOS != "linux" {
		return 0.0
	}

	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0.0
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)[1:]
			var total, idle uint64
			for i, valStr := range fields {
				val, _ := strconv.ParseUint(valStr, 10, 64)
				total += val
				if i == 3 { // idle field
					idle = val
				}
			}

			if m.lastCPUTotal == 0 {
				m.lastCPUTotal = total
				m.lastCPUIdle = idle
				return 0.0
			}

			totalDelta := total - m.lastCPUTotal
			idleDelta := idle - m.lastCPUIdle

			m.lastCPUTotal = total
			m.lastCPUIdle = idle

			if totalDelta == 0 {
				return 0.0
			}

			usedDelta := totalDelta - idleDelta
			percent := (float64(usedDelta) / float64(totalDelta)) * 100.0
			if percent < 0 {
				percent = 0
			}
			if percent > 100 {
				percent = 100
			}
			return float64(int(percent*10)) / 10.0 // round 1 decimal
		}
	}

	return 0.0
}

func (m *MetricsCollector) collectMemory() (usedMB uint64, totalMB uint64) {
	if runtime.GOOS == "linux" {
		file, err := os.Open("/proc/meminfo")
		if err == nil {
			defer file.Close()
			var memTotalKB, memAvailableKB uint64
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "MemTotal:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						memTotalKB, _ = strconv.ParseUint(fields[1], 10, 64)
					}
				} else if strings.HasPrefix(line, "MemAvailable:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						memAvailableKB, _ = strconv.ParseUint(fields[1], 10, 64)
					}
				}
			}
			if memTotalKB > 0 {
				totalMB = memTotalKB / 1024
				if memTotalKB >= memAvailableKB {
					usedMB = (memTotalKB - memAvailableKB) / 1024
				}
				return usedMB, totalMB
			}
		}
	}

	// Fallback to Go runtime stats
	var rtm runtime.MemStats
	runtime.ReadMemStats(&rtm)
	usedMB = rtm.Alloc / (1024 * 1024)
	totalMB = rtm.Sys / (1024 * 1024)
	if totalMB < usedMB {
		totalMB = usedMB
	}
	return usedMB, totalMB
}

func (m *MetricsCollector) collectDisk() float64 {
	path := "/"
	if runtime.GOOS == "windows" {
		path = "C:\\"
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0.0
	}

	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	if total == 0 {
		return 0.0
	}

	used := total - free
	percent := (float64(used) / float64(total)) * 100.0
	return float64(int(percent*10)) / 10.0
}
