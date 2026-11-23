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
}

// ValidateHostPathSecurity memvalidasi path host untuk bind mount agar aman dari directory traversal
// dan tidak mengakses direktori sistem kritis.
func ValidateHostPathSecurity(hostPath string) error {
	if hostPath == "" {
		return fmt.Errorf("host path cannot be empty")
	}

	if !filepath.IsAbs(hostPath) {
		return fmt.Errorf("host path must be absolute: %s", hostPath)
	}

	clean := filepath.Clean(hostPath)

	if clean == "/" {
		return fmt.Errorf("mounting root directory / is forbidden")
	}

	for _, prefix := range blacklistedPrefixes {
		if clean == prefix || strings.HasPrefix(clean, prefix+"/") {
			return fmt.Errorf("mounting system path %s is forbidden for security", clean)
		}
	}

	return nil
}
