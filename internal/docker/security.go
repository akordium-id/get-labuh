package docker

import (
	"fmt"
	"path/filepath"
	"strings"
)

var blacklistedPrefixes = []string{
	"/etc",
	"/var/run",
	"/run",
	"/proc",
	"/sys",
	"/dev",
	"/boot",
	"/root",
	"/bin",
	"/sbin",
	"/usr",
	"/lib",
	"/lib64",
	"/var/lib/docker",
	"/var/lib/labuh",
}

// ValidateHostPathSecurity memvalidasi path host untuk bind mount agar aman dari directory traversal
// dan tidak mengakses direktori sistem kritis.
func ValidateHostPathSecurity(hostPath string) error {
	if hostPath == "" {
		return fmt.Errorf("host path cannot be empty")
	}

	clean := filepath.Clean(hostPath)

	if !filepath.IsAbs(clean) {
		return fmt.Errorf("host path must be absolute: %s", hostPath)
	}

	if clean == "/" {
		return fmt.Errorf("mounting root directory / is forbidden")
	}

	for _, prefix := range blacklistedPrefixes {
		if clean == prefix || strings.HasPrefix(clean, prefix+"/") {
			return fmt.Errorf("mounting system path %s is forbidden for security", clean)
		}
	}

	// Evaluasi symlink jika path sudah ada di host filesystem
	if evalPath, err := filepath.EvalSymlinks(clean); err == nil {
		if evalPath == "/" {
			return fmt.Errorf("mounting root directory / is forbidden")
		}
		for _, prefix := range blacklistedPrefixes {
			if evalPath == prefix || strings.HasPrefix(evalPath, prefix+"/") {
				return fmt.Errorf("symlink target %s points to forbidden system path", evalPath)
			}
		}
	}

	return nil
}

// ValidateContainerMountPath memvalidasi dan membersihkan mount path container.
func ValidateContainerMountPath(mountPath string) (string, error) {
	trimmed := strings.TrimSpace(mountPath)
	if trimmed == "" {
		return "", fmt.Errorf("container mount path cannot be empty")
	}

	if !filepath.IsAbs(trimmed) || !strings.HasPrefix(trimmed, "/") {
		return "", fmt.Errorf("container mount path must be absolute: %s", trimmed)
	}

	clean := filepath.Clean(trimmed)
	if clean == "/" {
		return "", fmt.Errorf("mounting container root directory / is forbidden")
	}

	return clean, nil
}
