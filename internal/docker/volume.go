package docker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/docker/docker/api/types/volume"

	"github.com/akordium-id/get-labuh/internal/models"
)

type VolumeBinding struct {
	Source     string // Named Volume Name, Host Directory Path, or Runtime File Path
	Target     string // Container Mount Path
	IsReadOnly bool
}

func (vb VolumeBinding) ToBindString() string {
	mode := "rw"
	if vb.IsReadOnly {
		mode = "ro"
	}
	return fmt.Sprintf("%s:%s:%s", vb.Source, vb.Target, mode)
}

// EnsureVolume memastikan volume Docker dengan nama yang diberikan telah dibuat.
func (c *Client) EnsureVolume(ctx context.Context, volumeName string) error {
	if c.cli == nil {
		return fmt.Errorf("docker client not initialized")
	}

	_, err := c.cli.VolumeInspect(ctx, volumeName)
	if err == nil {
		return nil
	}

	_, err = c.cli.VolumeCreate(ctx, volume.CreateOptions{
		Name: volumeName,
	})
	if err != nil {
		return fmt.Errorf("failed to create docker volume %s: %w", volumeName, err)
	}

	return nil
}

// RemoveVolume menghapus volume Docker dengan nama yang diberikan.
func (c *Client) RemoveVolume(ctx context.Context, volumeName string) error {
	if c.cli == nil {
		return fmt.Errorf("docker client not initialized")
	}

	err := c.cli.VolumeRemove(ctx, volumeName, true)
	if err != nil {
		return fmt.Errorf("failed to remove docker volume %s: %w", volumeName, err)
	}

	return nil
}

// PrepareFileMounts menulis isi konten file mounts ke disk lokal host dan mengembalikan slice VolumeBinding.
func PrepareFileMounts(baseDir, appID string, mounts []*models.AppFileMount) ([]VolumeBinding, error) {
	if len(mounts) == 0 {
		return nil, nil
	}

	if baseDir == "" {
		baseDir = "/var/lib/labuh/app-files"
		if os.Geteuid() != 0 {
			if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
				baseDir = filepath.Join(homeDir, ".labuh", "app-files")
			} else {
				baseDir = "./.labuh/app-files"
			}
		}
	}

	appDir := filepath.Join(baseDir, appID)
	if err := os.MkdirAll(appDir, 0755); err != nil {
		if baseDir == "/var/lib/labuh/app-files" {
			fallback := "./.labuh/app-files"
			if homeDir, hErr := os.UserHomeDir(); hErr == nil && homeDir != "" {
				fallback = filepath.Join(homeDir, ".labuh", "app-files")
			}
			appDir = filepath.Join(fallback, appID)
			if fbErr := os.MkdirAll(appDir, 0755); fbErr != nil {
				return nil, fmt.Errorf("failed to create file mount dir %s: %w", appDir, fbErr)
			}
		} else {
			return nil, fmt.Errorf("failed to create file mount dir %s: %w", appDir, err)
		}
	}

	bindings := make([]VolumeBinding, 0, len(mounts))
	for _, m := range mounts {
		h := sha256.Sum256([]byte(m.MountPath))
		hashStr := hex.EncodeToString(h[:8]) // 16 hex chars
		baseName := filepath.Base(m.MountPath)
		fileName := fmt.Sprintf("%s_%s", hashStr, baseName)
		hostFilePath := filepath.Join(appDir, fileName)

		// Parse permissions octal (mis. "0644" -> 0644)
		perm := os.FileMode(0644)
		if m.FileMode != "" {
			if parsed, err := strconv.ParseUint(m.FileMode, 8, 32); err == nil {
				perm = os.FileMode(parsed)
			}
		}

		if err := os.WriteFile(hostFilePath, []byte(m.Content), perm); err != nil {
			return nil, fmt.Errorf("failed to write file mount %s: %w", hostFilePath, err)
		}

		bindings = append(bindings, VolumeBinding{
			Source:     hostFilePath,
			Target:     m.MountPath,
			IsReadOnly: m.IsReadOnly,
		})
	}

	return bindings, nil
}
