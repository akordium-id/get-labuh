package observability

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

type HostStats struct {
	DiskTotal       uint64  `json:"disk_total"`
	DiskUsed        uint64  `json:"disk_used"`
	DiskFree        uint64  `json:"disk_free"`
	DiskUsedPercent float64 `json:"disk_used_percent"`
	MemTotal        uint64  `json:"mem_total"`
	MemUsed         uint64  `json:"mem_used"`
	MemFree         uint64  `json:"mem_free"`
	MemUsedPercent  float64 `json:"mem_used_percent"`
	CPULoad1        float64 `json:"cpu_load_1"`
	CPULoad5        float64 `json:"cpu_load_5"`
	CPULoad15       float64 `json:"cpu_load_15"`
	CPUNum          int     `json:"cpu_num"`
}

// GetHostStats mengambil snapshot realtime resource host (Disk, RAM, CPU).
func GetHostStats(diskPath string) HostStats {
	if diskPath == "" {
		diskPath = "/"
	}

	stats := HostStats{
		CPUNum: runtime.NumCPU(),
	}

	diskTotal, diskUsed, diskFree, diskUsedPct, _ := GetDiskStats(diskPath)
	stats.DiskTotal = diskTotal
	stats.DiskUsed = diskUsed
	stats.DiskFree = diskFree
	stats.DiskUsedPercent = diskUsedPct

	memTotal, memUsed, memFree, memUsedPct, _ := GetMemoryStats()
	stats.MemTotal = memTotal
	stats.MemUsed = memUsed
	stats.MemFree = memFree
	stats.MemUsedPercent = memUsedPct

	load1, load5, load15, numCPU := GetCPULoad()
	stats.CPULoad1 = load1
	stats.CPULoad5 = load5
	stats.CPULoad15 = load15
	stats.CPUNum = numCPU

	return stats
}

// GetDiskStats mengambil statistik kapasitas disk untuk path tertentu.
func GetDiskStats(path string) (total, used, free uint64, usedPercent float64, err error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0, 0, 0, err
	}

	total = stat.Blocks * uint64(stat.Bsize)
	avail := stat.Bavail * uint64(stat.Bsize)
	bfree := stat.Bfree * uint64(stat.Bsize)

	if total > 0 {
		used = total - bfree
		free = avail
		usedPercent = float64(used) / float64(total) * 100.0
	}
	return total, used, free, usedPercent, nil
}

// GetMemoryStats membaca statistik RAM dari /proc/meminfo dengan fallback runtime.MemStats.
func GetMemoryStats() (total, used, free uint64, usedPercent float64, err error) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		sys := m.Sys
		alloc := m.Alloc
		if sys > 0 {
			return sys, alloc, sys - alloc, float64(alloc) / float64(sys) * 100.0, nil
		}
		return 0, 0, 0, 0, nil
	}
	defer file.Close()

	var memTotal, memAvailable, memFree, buffers, cached uint64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		valBytes := val * 1024 // /proc/meminfo values are in kB

		switch fields[0] {
		case "MemTotal:":
			memTotal = valBytes
		case "MemAvailable:":
			memAvailable = valBytes
		case "MemFree:":
			memFree = valBytes
		case "Buffers:":
			buffers = valBytes
		case "Cached:":
			cached = valBytes
		}
	}

	if memTotal > 0 {
		if memAvailable > 0 {
			used = memTotal - memAvailable
			free = memAvailable
		} else {
			free = memFree + buffers + cached
			if memTotal > free {
				used = memTotal - free
			}
		}
		usedPercent = float64(used) / float64(memTotal) * 100.0
	}
	return memTotal, used, free, usedPercent, nil
}

// GetCPULoad membaca load average 1, 5, 15 menit dari /proc/loadavg.
func GetCPULoad() (load1, load5, load15 float64, numCPU int) {
	numCPU = runtime.NumCPU()
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, 0, 0, numCPU
	}
	fields := strings.Fields(string(data))
	if len(fields) >= 3 {
		load1, _ = strconv.ParseFloat(fields[0], 64)
		load5, _ = strconv.ParseFloat(fields[1], 64)
		load15, _ = strconv.ParseFloat(fields[2], 64)
	}
	return load1, load5, load15, numCPU
}
