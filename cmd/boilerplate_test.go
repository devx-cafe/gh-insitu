package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ─── fetchCommitDiffURL ───────────────────────────────────────────────────────

func TestFetchCommitDiffURL_ReturnsBody(t *testing.T) {
	const wantDiff = "diff --git a/foo.txt b/foo.txt\n+hello\n"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Accept") != "application/vnd.github.diff" {
			http.Error(w, "wrong accept", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(wantDiff))
	}))
	defer srv.Close()

	got, err := fetchCommitDiffURL(srv.URL, "tok")
	if err != nil {
		t.Fatalf("fetchCommitDiffURL() error = %v", err)
	}
	if got != wantDiff {
		t.Errorf("fetchCommitDiffURL() = %q, want %q", got, wantDiff)
	}
}

func TestFetchCommitDiffURL_NonOKStatusReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := fetchCommitDiffURL(srv.URL, "tok")
	if err == nil {
		t.Fatal("fetchCommitDiffURL() error = nil, want error on 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error %q does not mention status code 404", err.Error())
	}
}

func TestFetchCommitDiffURL_SetsAuthHeader(t *testing.T) {
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, err := fetchCommitDiffURL(srv.URL, "mytoken")
	if err != nil {
		t.Fatalf("fetchCommitDiffURL() error = %v", err)
	}
	if gotAuth != "Bearer mytoken" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer mytoken")
	}
}

func TestFetchCommitDiffURL_SetsAPIVersionHeader(t *testing.T) {
	var gotVersion string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.Header.Get("X-GitHub-Api-Version")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	_, _ = fetchCommitDiffURL(srv.URL, "tok")
	if gotVersion != "2022-11-28" {
		t.Errorf("X-GitHub-Api-Version = %q, want %q", gotVersion, "2022-11-28")
	}
}

// ─── checkCleanWorkingTree ────────────────────────────────────────────────────

func TestCheckCleanWorkingTree_FailsOutsideGitRepo(t *testing.T) {
	// Change to a temp dir that is not a git repository so git status fails.
	nonGitDir := t.TempDir()
	t.Chdir(nonGitDir)

	err := checkCleanWorkingTree()
	if err == nil {
		t.Fatal("checkCleanWorkingTree() error = nil, want error outside git repo")
	}
}
