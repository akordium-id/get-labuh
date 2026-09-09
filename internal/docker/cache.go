package docker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CacheStats struct {
	TotalSize int64 `json:"total_size"`
	EntryCount int `json:"entry_count"`
}

const DefaultCacheDir = "/var/lib/labuh/docker-cache"

func GenerateCacheRef(appID, branch string) string {
	appPart := strings.ToLower(strings.TrimSpace(appID))
	branchPart := strings.ToLower(strings.TrimSpace(branch))
	if branchPart == "" {
		branchPart = "main"
	}
	return fmt.Sprintf("labuh-%s-%s", appPart, branchPart)
}

func CleanOldCache(olderThan time.Duration) error {
	cacheDir := getCacheDir()
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	cutoff := time.Now().Add(-olderThan)
	var lastErr error
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			lastErr = err
			continue
		}
		if info.ModTime().Before(cutoff) {
			path := filepath.Join(cacheDir, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

func GetCacheStats() (CacheStats, error) {
	cacheDir := getCacheDir()
	var stats CacheStats
	err := filepath.WalkDir(cacheDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return nil
			}
			stats.TotalSize += info.Size()
			stats.EntryCount++
		}
		return nil
	})
	if err != nil {
		if os.IsNotExist(err) {
			return stats, nil
		}
		return stats, err
	}
	return stats, nil
}

func getCacheDir() string {
	base := os.Getenv("LABUH_CACHE_DIR")
	if base == "" {
		base = DefaultCacheDir
	}
	return base
}

func EnsureCacheDir() error {
	cacheDir := getCacheDir()
	return os.MkdirAll(cacheDir, 0755)
}
