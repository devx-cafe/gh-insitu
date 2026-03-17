package github_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gh "github.com/devx-cafe/gh-insitu/internal/github"
)

func TestToken_PrefersGHToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "gh-token-value")
	t.Setenv("GITHUB_TOKEN", "github-token-value")

	got := gh.Token()
	if got != "gh-token-value" {
		t.Errorf("Token() = %q, want %q", got, "gh-token-value")
	}
}

func TestToken_FallsBackToGitHubToken(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "fallback-token")

	got := gh.Token()
	if got != "fallback-token" {
		t.Errorf("Token() = %q, want %q", got, "fallback-token")
	}
}

func TestToken_EmptyWhenNoneSet(t *testing.T) {
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	got := gh.Token()
	if got != "" {
		t.Errorf("Token() = %q, want empty string", got)
	}
}

func TestCurrentSHA_UsesEnvVar(t *testing.T) {
	t.Setenv("GITHUB_SHA", "abc123def456")

	got, err := gh.CurrentSHA()
	if err != nil {
		t.Fatalf("CurrentSHA() error = %v, want nil", err)
	}
	if got != "abc123def456" {
		t.Errorf("CurrentSHA() = %q, want %q", got, "abc123def456")
	}
}

func TestCurrentRepo_UsesEnvVar(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")

	got, err := gh.CurrentRepo()
	if err != nil {
		t.Fatalf("CurrentRepo() error = %v, want nil", err)
	}
	if got != "owner/repo" {
		t.Errorf("CurrentRepo() = %q, want %q", got, "owner/repo")
	}
}

func TestSetCommitStatus_Success(t *testing.T) {
	var receivedBody map[string]string
	var receivedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	// Patch the URL by replacing the API base; for testing we use a custom transport
	// via a thin wrapper. Since SetCommitStatus uses http.DefaultClient directly,
	// we use an httptest server that intercepts via the repo path in the URL.
	// We call a helper that rewires the base URL for testing.
	err := gh.SetCommitStatusURL(srv.URL+"/repos/%s/statuses/%s",
		"owner/repo", "abc123", "test-token",
		gh.StateSuccess, "Build passed", "ci/build")
	if err != nil {
		t.Fatalf("SetCommitStatus() error = %v, want nil", err)
	}

	if receivedBody["state"] != "success" {
		t.Errorf("state = %q, want %q", receivedBody["state"], "success")
	}
	if receivedBody["context"] != "ci/build" {
		t.Errorf("context = %q, want %q", receivedBody["context"], "ci/build")
	}
	if receivedBody["description"] != "Build passed" {
		t.Errorf("description = %q, want %q", receivedBody["description"], "Build passed")
	}
	if !strings.HasPrefix(receivedAuth, "Bearer ") {
		t.Errorf("Authorization header %q missing Bearer prefix", receivedAuth)
	}
}

func TestSetCommitStatus_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	err := gh.SetCommitStatusURL(srv.URL+"/repos/%s/statuses/%s",
		"owner/repo", "abc123", "bad-token",
		gh.StateFailure, "Build failed", "ci/build")
	if err == nil {
		t.Fatal("SetCommitStatus() error = nil, want error for 401")
	}
}

func TestSetCommitStatus_MissingParams(t *testing.T) {
	tests := []struct {
		name  string
		repo  string
		sha   string
		token string
	}{
		{"missing repo", "", "abc", "token"},
		{"missing sha", "owner/repo", "", "token"},
		{"missing token", "owner/repo", "abc", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gh.SetCommitStatus(tt.repo, tt.sha, tt.token, gh.StatePending, "", "ctx")
			if err == nil {
				t.Errorf("SetCommitStatus() error = nil, want validation error")
			}
		})
	}
}
