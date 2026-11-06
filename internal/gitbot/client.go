package gitbot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	Marker          = "<!-- labuh-pr-preview -->"
	StatusBuilding  = "building"
	StatusLive      = "live"
	StatusFailed    = "failed"
	StatusDestroyed = "destroyed"
)

type CommentParams struct {
	Status     string
	PreviewURL string
	CommitHash string
	Branch     string
	ErrorMsg   string
	UpdatedAt  time.Time
}

// FormatComment membuat teks markdown komentar dengan marker anti-spam.
func FormatComment(p CommentParams) string {
	var sb strings.Builder
	sb.WriteString(Marker + "\n")
	sb.WriteString("### ⛵ Labuh Preview Deployment\n\n")

	switch p.Status {
	case StatusBuilding:
		sb.WriteString("Deployment preview environment untuk PR ini sedang diproses...\n")
		if p.CommitHash != "" {
			sb.WriteString(fmt.Sprintf("- **Commit**: `%s`\n", p.CommitHash))
		}
		if p.Branch != "" {
			sb.WriteString(fmt.Sprintf("- **Branch**: `%s`\n", p.Branch))
		}
		sb.WriteString("- **Status**: 🟡 *Building & Deploying...*\n")

	case StatusLive:
		sb.WriteString("Preview environment telah siap diakses!\n")
		if p.PreviewURL != "" {
			sb.WriteString(fmt.Sprintf("- **Preview URL**: [%s](%s)\n", p.PreviewURL, p.PreviewURL))
		}
		if p.CommitHash != "" {
			sb.WriteString(fmt.Sprintf("- **Commit**: `%s`\n", p.CommitHash))
		}
		sb.WriteString("- **Status**: 🟢 *Live & Healthy*\n")
		timestamp := p.UpdatedAt
		if timestamp.IsZero() {
			timestamp = time.Now()
		}
		sb.WriteString(fmt.Sprintf("- **Updated At**: `%s`\n", timestamp.Format("2006-01-02 15:04:05 MST")))

	case StatusFailed:
		sb.WriteString("Deployment preview environment mengalami kegagalan.\n")
		if p.CommitHash != "" {
			sb.WriteString(fmt.Sprintf("- **Commit**: `%s`\n", p.CommitHash))
		}
		sb.WriteString("- **Status**: 🔴 *Deployment Failed*\n")
		if p.ErrorMsg != "" {
			sb.WriteString(fmt.Sprintf("- **Error**: `%s`\n", p.ErrorMsg))
		}

	case StatusDestroyed:
		sb.WriteString("Pull Request telah ditutup. Preview environment telah dinonaktifkan (teardown).\n")
		sb.WriteString("- **Status**: ⚪ *Destroyed*\n")
	}

	return sb.String()
}

// Client menangani komunikasi API ke GitHub dan GitLab.
type Client struct {
	githubBaseURL string
	gitlabBaseURL string
	httpClient    *http.Client
}

// NewClient membuat client default dengan official API base URLs.
func NewClient() *Client {
	return NewClientWithBaseURLs("https://api.github.com", "https://gitlab.com")
}

// NewClientWithBaseURLs membuat client dengan URL kustom (misal untuk testing atau self-hosted).
func NewClientWithBaseURLs(githubBaseURL, gitlabBaseURL string) *Client {
	return &Client{
		githubBaseURL: strings.TrimRight(githubBaseURL, "/"),
		gitlabBaseURL: strings.TrimRight(gitlabBaseURL, "/"),
		httpClient:    &http.Client{Timeout: 10 * time.Second},
	}
}

// PostOrUpdateGitHubComment mengirim komentar baru atau me-replace komentar lama jika marker ditemukan.
func (c *Client) PostOrUpdateGitHubComment(ctx context.Context, repoURL string, prNumber int, token, body string) error {
	if token == "" {
		return nil
	}

	_, owner, repo, err := ParseRepo(repoURL)
	if err != nil {
		return err
	}

	// 1. List existing comments to find one with Marker
	listURL := fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments", c.githubBaseURL, owner, repo, prNumber)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var existingCommentID int64
	if resp.StatusCode == http.StatusOK {
		var comments []struct {
			ID   int64  `json:"id"`
			Body string `json:"body"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&comments); err == nil {
			for _, comment := range comments {
				if strings.Contains(comment.Body, Marker) {
					existingCommentID = comment.ID
					break
				}
			}
		}
	}

	payload := map[string]string{"body": body}
	payloadBytes, _ := json.Marshal(payload)

	if existingCommentID > 0 {
		// Update existing comment
		patchURL := fmt.Sprintf("%s/repos/%s/%s/issues/comments/%d", c.githubBaseURL, owner, repo, existingCommentID)
		patchReq, err := http.NewRequestWithContext(ctx, http.MethodPatch, patchURL, bytes.NewReader(payloadBytes))
		if err != nil {
			return err
		}
		patchReq.Header.Set("Authorization", "Bearer "+token)
		patchReq.Header.Set("Content-Type", "application/json")
		patchReq.Header.Set("Accept", "application/vnd.github+json")

		patchResp, err := c.httpClient.Do(patchReq)
		if err != nil {
			return err
		}
		defer patchResp.Body.Close()
		if patchResp.StatusCode >= 400 {
			resBody, _ := io.ReadAll(patchResp.Body)
			return fmt.Errorf("github api patch error %d: %s", patchResp.StatusCode, string(resBody))
		}
		return nil
	}

	// Create new comment
	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, listURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	postReq.Header.Set("Authorization", "Bearer "+token)
	postReq.Header.Set("Content-Type", "application/json")
	postReq.Header.Set("Accept", "application/vnd.github+json")

	postResp, err := c.httpClient.Do(postReq)
	if err != nil {
		return err
	}
	defer postResp.Body.Close()
	if postResp.StatusCode >= 400 {
		resBody, _ := io.ReadAll(postResp.Body)
		return fmt.Errorf("github api post error %d: %s", postResp.StatusCode, string(resBody))
	}

	return nil
}

// PostOrUpdateGitLabNote mengirim note baru pada Merge Request atau mengedit jika marker ditemukan.
func (c *Client) PostOrUpdateGitLabNote(ctx context.Context, repoURL string, mrIID int, token, body string) error {
	if token == "" {
		return nil
	}

	_, ownerOrPath, repo, err := ParseRepo(repoURL)
	if err != nil {
		return err
	}

	projectPath := fmt.Sprintf("%s/%s", ownerOrPath, repo)
	encodedPath := url.PathEscape(projectPath)

	listURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d/notes", c.gitlabBaseURL, encodedPath, mrIID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, listURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("PRIVATE-TOKEN", token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var existingNoteID int64
	if resp.StatusCode == http.StatusOK {
		var notes []struct {
			ID   int64  `json:"id"`
			Body string `json:"body"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&notes); err == nil {
			for _, note := range notes {
				if strings.Contains(note.Body, Marker) {
					existingNoteID = note.ID
					break
				}
			}
		}
	}

	payload := map[string]string{"body": body}
	payloadBytes, _ := json.Marshal(payload)

	if existingNoteID > 0 {
		putURL := fmt.Sprintf("%s/api/v4/projects/%s/merge_requests/%d/notes/%d", c.gitlabBaseURL, encodedPath, mrIID, existingNoteID)
		putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, putURL, bytes.NewReader(payloadBytes))
		if err != nil {
			return err
		}
		putReq.Header.Set("PRIVATE-TOKEN", token)
		putReq.Header.Set("Content-Type", "application/json")

		putResp, err := c.httpClient.Do(putReq)
		if err != nil {
			return err
		}
		defer putResp.Body.Close()
		if putResp.StatusCode >= 400 {
			resBody, _ := io.ReadAll(putResp.Body)
			return fmt.Errorf("gitlab api put error %d: %s", putResp.StatusCode, string(resBody))
		}
		return nil
	}

	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, listURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}
	postReq.Header.Set("PRIVATE-TOKEN", token)
	postReq.Header.Set("Content-Type", "application/json")

	postResp, err := c.httpClient.Do(postReq)
	if err != nil {
		return err
	}
	defer postResp.Body.Close()
	if postResp.StatusCode >= 400 {
		resBody, _ := io.ReadAll(postResp.Body)
		return fmt.Errorf("gitlab api post error %d: %s", postResp.StatusCode, string(resBody))
	}

	return nil
}
