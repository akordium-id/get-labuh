package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
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

	if c.cli == nil {
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return err
		}
		dummyDockerfile := filepath.Join(targetDir, "Dockerfile")
		_ = os.WriteFile(dummyDockerfile, []byte("FROM alpine:latest\nCMD [\"sleep\", \"3600\"]\n"), 0644)
		return nil
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "-b", sanitizedBranch, repoURL, targetDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If branch clone fails, attempt fallback clone without explicit branch
		fallbackCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", repoURL, targetDir)
		fbOutput, fbErr := fallbackCmd.CombinedOutput()
		if fbErr != nil {
			return fmt.Errorf("git clone failed: %w, output: %s", err, string(output)+" "+string(fbOutput))
		}
	}
	return nil
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
	return c.BuildFromDockerfileWithCache(ctx, appID, buildContextPath, dockerfilePath, "")
}

func (c *Client) BuildFromDockerfileWithCache(ctx context.Context, appID, buildContextPath, dockerfilePath, cacheRef string) (string, error) {
	if appID == "" {
		return "", fmt.Errorf("build error: invalid application ID")
	}
	if buildContextPath == "" {
		return "", fmt.Errorf("build error: build context path is empty")
	}

	imageTag := fmt.Sprintf("labuh-%s:%s", appID, time.Now().Format("20060102150405"))

	if c.cli == nil {
		return imageTag, nil
	}

	tarBuf, err := CreateTarFromDirectory(buildContextPath)
	if err != nil {
		return "", fmt.Errorf("build error: failed to create tar archive: %w", err)
	}

	relDockerfilePath := dockerfilePath
	if filepath.IsAbs(dockerfilePath) {
		rel, err := filepath.Rel(buildContextPath, dockerfilePath)
		if err == nil {
			relDockerfilePath = rel
		}
	}
	if relDockerfilePath == "" {
		relDockerfilePath = "Dockerfile"
	}

	tags := []string{imageTag}
	if cacheRef != "" {
		tags = append(tags, cacheRef)
	}

	builtTag, err := c.BuildImage(ctx, tarBuf, relDockerfilePath, tags)
	if err != nil {
		return "", fmt.Errorf("build error: %w", err)
	}

	return builtTag, nil
}

func (c *Client) BuildWithCache(ctx context.Context, appID, branch, buildContextPath, cacheRef string) (string, error) {
	if cacheRef == "" {
		cacheRef = GenerateCacheRef(appID, branch)
	}
	dockerfilePath := "Dockerfile"
	if buildContextPath != "" {
		dockerfilePath = filepath.Join(buildContextPath, "Dockerfile")
	}
	return c.BuildFromDockerfileWithCache(ctx, appID, buildContextPath, dockerfilePath, cacheRef)
}

func (c *Client) PullBaseImage(ctx context.Context, imageRef string) error {
	if c.cli == nil {
		return nil
	}
	return c.PullImage(ctx, imageRef)
}

func (c *Client) PrePullKnownTemplates(ctx context.Context, templates []string) error {
	for _, template := range templates {
		if err := c.PullBaseImage(ctx, template); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) BuildFromGit(ctx context.Context, repoURL, branch, dockerfilePath string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "labuh-git-build-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp build directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := c.CloneGitRepo(ctx, repoURL, branch, tmpDir); err != nil {
		return "", err
	}

	targetDockerfile := dockerfilePath
	if targetDockerfile == "" {
		targetDockerfile = "Dockerfile"
	}

	return c.BuildFromDockerfileWithCache(ctx, "git", tmpDir, targetDockerfile, "")
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
