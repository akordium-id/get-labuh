package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func ValidateRepositoryURL(repoURL string) error {
	if repoURL == "" {
		return fmt.Errorf("repository URL is empty")
	}
	if !strings.HasPrefix(repoURL, "https://") &&
		!strings.HasPrefix(repoURL, "http://") &&
		!strings.HasPrefix(repoURL, "git@") &&
		!strings.HasPrefix(repoURL, "git://") {
		return fmt.Errorf("unsupported repository URL scheme: %s", repoURL)
	}
	return nil
}

func (c *Client) CloneGitRepo(ctx context.Context, repoURL, branch, targetDir string) error {
	if err := ValidateRepositoryURL(repoURL); err != nil {
		return fmt.Errorf("clone error: %w", err)
	}

	sanitizedBranch := SanitizeBranch(branch)
	slog.Info("cloning git repository", "url", repoURL, "branch", sanitizedBranch, "target", targetDir)
	return fmt.Errorf("git clone requires git binary and network access")
}

func StreamBuildOutput(ctx context.Context, reader io.Reader, logFile *os.File) {
	if reader == nil || logFile == nil {
		return
	}

	buf := make([]byte, 4096)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := reader.Read(buf)
		if n > 0 {
			if _, writeErr := logFile.Write(buf[:n]); writeErr != nil {
				slog.Error("failed to stream build output", "error", writeErr)
				return
			}
			if syncErr := logFile.Sync(); syncErr != nil {
				slog.Error("failed to sync build log", "error", syncErr)
			}
		}
		if err != nil {
			return
		}
	}
}

func (c *Client) BuildFromDockerfile(ctx context.Context, appID, buildContextPath, dockerfilePath string) (string, error) {
	return "", fmt.Errorf("use BuildFromDockerfileWithCache instead")
}

func (c *Client) BuildFromDockerfileWithCache(ctx context.Context, appID, branch, buildContextPath, dockerfilePath, cacheRef string) (string, error) {
	if appID == "" {
		return "", fmt.Errorf("build error: invalid application ID")
	}
	if buildContextPath == "" {
		return "", fmt.Errorf("build error: build context path is empty")
	}

	if err := EnsureCacheDir(); err != nil {
		return "", fmt.Errorf("build error: failed to prepare cache dir: %w", err)
	}

	imageTag := fmt.Sprintf("labuh-%s:%s", appID, time.Now().Format("20060102150405"))
	cacheDir := getCacheDir()

	buildContext, err := os.Open(buildContextPath)
	if err != nil {
		return "", fmt.Errorf("build error: failed to open build context: %w", err)
	}
	defer buildContext.Close()

	_ = dockerfilePath
	_ = buildContext
	_ = cacheRef
	_ = cacheDir
	_ = imageTag
	_ = branch

	return "", fmt.Errorf("not implemented without docker client")
}

func (c *Client) BuildWithCache(ctx context.Context, appID, buildContextPath, cacheRef string) (string, error) {
	return c.BuildFromDockerfileWithCache(ctx, appID, "", buildContextPath, "Dockerfile", cacheRef)
}

func (c *Client) BuildFromGit(ctx context.Context, repoURL, branch, dockerfilePath string) (string, error) {
	return "", fmt.Errorf("git clone + build requires git binary and tar archiving")
}

func CreateTarFromDirectory(srcDir string) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	defer tw.Close()

	err := filepath.Walk(srcDir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, file)
		if err != nil {
			return err
		}
		header.Name = relPath

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if fi.Mode().IsRegular() {
			f, err := os.Open(file)
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.Copy(tw, f)
			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &buf, nil
}

func SanitizeBranch(branch string) string {
	branch = strings.TrimSpace(branch)
	if branch == "" {
		return "main"
	}
	return branch
}

func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh %dm", h, m)
}

func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
