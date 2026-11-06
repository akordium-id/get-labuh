package gitbot

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseRepo mengekstrak provider, owner/path, dan repo name dari repository URL (HTTPS atau SSH).
func ParseRepo(rawURL string) (provider, ownerOrPath, repoName string, err error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", "", "", fmt.Errorf("empty repository url")
	}

	var host, path string

	if after, ok := strings.CutPrefix(rawURL, "git@"); ok {
		// Contoh: git@github.com:akordium-id/get-labuh.git
		parts := strings.SplitN(after, ":", 2)
		if len(parts) != 2 {
			return "", "", "", fmt.Errorf("invalid ssh repository url: %s", rawURL)
		}
		host = parts[0]
		path = parts[1]
	} else if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		parsed, parseErr := url.Parse(rawURL)
		if parseErr != nil {
			return "", "", "", fmt.Errorf("invalid http url: %w", parseErr)
		}
		host = parsed.Host
		path = strings.TrimPrefix(parsed.Path, "/")
	} else {
		return "", "", "", fmt.Errorf("unsupported repository url format: %s", rawURL)
	}

	path = strings.TrimSuffix(path, ".git")
	path = strings.Trim(path, "/")

	segments := strings.Split(path, "/")
	if len(segments) < 2 {
		return "", "", "", fmt.Errorf("invalid repository path: %s", path)
	}

	repoName = segments[len(segments)-1]
	ownerOrPath = strings.Join(segments[:len(segments)-1], "/")

	if strings.Contains(host, "github") {
		provider = "github"
	} else if strings.Contains(host, "gitlab") {
		provider = "gitlab"
	} else {
		provider = "generic"
	}

	return provider, ownerOrPath, repoName, nil
}
