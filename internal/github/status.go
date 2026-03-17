// Package github provides GitHub API helpers for insitu.
package github

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// State represents the allowed GitHub commit status states.
type State string

// GitHub commit status state values.
const (
	StatePending State = "pending"
	StateSuccess State = "success"
	StateFailure State = "failure"
	StateError   State = "error"
)

// statusPayload is the JSON body for the GitHub Statuses API.
type statusPayload struct {
	State       State  `json:"state"`
	TargetURL   string `json:"target_url,omitempty"`
	Description string `json:"description,omitempty"`
	Context     string `json:"context,omitempty"`
}

// SetCommitStatus creates or updates a commit status on GitHub.
//
// repo must be in "owner/repo" format. sha is the full or short commit SHA.
// token is the GitHub personal access token (GH_TOKEN / GITHUB_TOKEN).
func SetCommitStatus(repo, sha, token string, state State, description, context string) error {
	if repo == "" {
		return fmt.Errorf("repo is required")
	}
	if sha == "" {
		return fmt.Errorf("sha is required")
	}
	if token == "" {
		return fmt.Errorf("GitHub token is required (set GH_TOKEN or GITHUB_TOKEN)")
	}

	payload := statusPayload{
		State:       state,
		Description: description,
		Context:     context,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to encode payload: %w", err)
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/statuses/%s", repo, sha)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call GitHub API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}
	return nil
}

// CurrentSHA returns the commit SHA to use for status updates.
// It prefers the GITHUB_SHA environment variable (set in GitHub Actions)
// and falls back to running `git rev-parse HEAD`.
func CurrentSHA() (string, error) {
	if sha := os.Getenv("GITHUB_SHA"); sha != "" {
		return sha, nil
	}
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current commit SHA: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CurrentRepo returns the repository in "owner/repo" format.
// It prefers the GITHUB_REPOSITORY environment variable (set in GitHub Actions)
// and falls back to running `gh repo view`.
func CurrentRepo() (string, error) {
	if repo := os.Getenv("GITHUB_REPOSITORY"); repo != "" {
		return repo, nil
	}
	out, err := exec.Command("gh", "repo", "view", "--json", "nameWithOwner", "--jq", ".nameWithOwner").Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current repo: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Token returns the GitHub token from environment variables.
// It checks GH_TOKEN first, then GITHUB_TOKEN.
func Token() string {
	if t := os.Getenv("GH_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("GITHUB_TOKEN")
}
