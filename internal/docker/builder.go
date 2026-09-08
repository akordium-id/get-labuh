package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (c *Client) BuildFromDockerfile(ctx context.Context, appID, buildContextPath, dockerfilePath string) (string, error) {
	if appID == "" {
		return "", fmt.Errorf("invalid application")
	}

	imageTag := fmt.Sprintf("labuh-%s:%s", appID, time.Now().Format("20060102150405"))

	buildContext, err := os.Open(buildContextPath)
	if err != nil {
		return "", err
	}
	defer buildContext.Close()

	_ = imageTag
	_ = dockerfilePath

	return "", fmt.Errorf("not implemented without docker client")
}

func (c *Client) BuildFromGit(ctx context.Context, repoURL, branch, dockerfilePath string) (string, error) {
	return "", fmt.Errorf("not implemented: git clone + build requires git binary and tar archiving")
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
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
