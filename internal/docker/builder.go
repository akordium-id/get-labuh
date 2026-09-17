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

func SanitizeBranch(branch string) string {
	b := strings.TrimSpace(branch)
	if b == "" {
		return "main"
	}
	return b
}

func (c *Client) CloneGitRepo(ctx context.Context, repoURL, branch, targetDir string) error {
	if err := ValidateRepositoryURL(repoURL); err != nil {
		return fmt.Errorf("clone error: %w", err)
	}

	sanitizedBranch := SanitizeBranch(branch)
	slog.Info("cloning git repository", "url", repoURL, "branch", sanitizedBranch, "target", targetDir)

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", sanitizedBranch, repoURL, targetDir)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %v, output: %s", err, string(output))
	}

	return nil
}

func StreamBuildOutput(ctx context.Context, reader io.Reader, logFile io.Writer) {
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
		}
		if err != nil {
			return
		}
	}
}

func TarDirectory(srcDir string) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	err := filepath.Walk(srcDir, func(file string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, file)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(fi, fi.Name())
		if err != nil {
			return err
		}

		header.Name = filepath.ToSlash(relPath)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !fi.Mode().IsRegular() {
			return nil
		}

		f, err := os.Open(file)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = io.Copy(tw, f)
		return err
	})

	if err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}

	return &buf, nil
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

	if dockerfilePath == "" {
		dockerfilePath = "Dockerfile"
	}

	imageTag := fmt.Sprintf("labuh-%s:%s", appID, time.Now().Format("20060102150405"))

	tarStream, err := TarDirectory(buildContextPath)
	if err != nil {
		return "", fmt.Errorf("failed to tar build context: %w", err)
	}

	tags := []string{imageTag}
	if cacheRef != "" {
		tags = append(tags, cacheRef)
	}

	builtTag, err := c.BuildImage(ctx, tarStream, dockerfilePath, tags)
	if err != nil {
		return "", err
	}

	return builtTag, nil
}

func (c *Client) BuildWithCache(ctx context.Context, appID, branch, buildContextPath, cacheRef string) (string, error) {
	if cacheRef == "" {
		cacheRef = GenerateCacheRef(appID, branch)
	}
	return c.BuildFromDockerfileWithCache(ctx, appID, buildContextPath, "Dockerfile", cacheRef)
}

func (c *Client) BuildWithNixpacks(ctx context.Context, appID, srcDir string, envs map[string]string, logWriter io.Writer) (string, error) {
	if appID == "" {
		return "", fmt.Errorf("invalid app ID")
	}

	nixpacksBin, err := exec.LookPath("nixpacks")
	if err != nil {
		return "", fmt.Errorf("nixpacks binary not found on host. Please install nixpacks (curl -sSL https://nixpacks.com/install.sh | bash): %w", err)
	}

	imageTag := fmt.Sprintf("labuh-%s:%s", appID, time.Now().Format("20060102150405"))
	args := []string{"build", srcDir, "--name", imageTag}

	for k, v := range envs {
		args = append(args, "--env", fmt.Sprintf("%s=%s", k, v))
	}

	cmd := exec.CommandContext(ctx, nixpacksBin, args...)
	if logWriter != nil {
		cmd.Stdout = logWriter
		cmd.Stderr = logWriter
	}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("nixpacks build failed: %w", err)
	}

	return imageTag, nil
}
