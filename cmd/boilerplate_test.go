package cmd

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── fetchCommitInfoURL ───────────────────────────────────────────────────────

func TestFetchCommitInfoURL_ReturnsFiles(t *testing.T) {
	want := commitInfo{
		SHA: "abc123",
		Files: []commitFile{
			{SHA: "blobsha", Filename: "foo.txt", Status: "added"},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	}))
	defer srv.Close()

	got, err := fetchCommitInfoURL(srv.URL, "tok")
	if err != nil {
		t.Fatalf("fetchCommitInfoURL() error = %v", err)
	}
	if got.SHA != want.SHA {
		t.Errorf("SHA = %q, want %q", got.SHA, want.SHA)
	}
	if len(got.Files) != 1 || got.Files[0].Filename != "foo.txt" {
		t.Errorf("Files = %+v, want one entry for foo.txt", got.Files)
	}
}

func TestFetchCommitInfoURL_NonOKStatusReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := fetchCommitInfoURL(srv.URL, "tok")
	if err == nil {
		t.Fatal("fetchCommitInfoURL() error = nil, want error on 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error %q does not mention status code 404", err.Error())
	}
}

func TestFetchCommitInfoURL_SetsAuthHeader(t *testing.T) {
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sha":"x","files":[]}`))
	}))
	defer srv.Close()

	_, err := fetchCommitInfoURL(srv.URL, "mytoken")
	if err != nil {
		t.Fatalf("fetchCommitInfoURL() error = %v", err)
	}
	if gotAuth != "Bearer mytoken" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer mytoken")
	}
}

func TestFetchCommitInfoURL_SetsAPIVersionHeader(t *testing.T) {
	var gotVersion string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.Header.Get("X-GitHub-Api-Version")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"sha":"x","files":[]}`))
	}))
	defer srv.Close()

	_, _ = fetchCommitInfoURL(srv.URL, "tok")
	if gotVersion != "2022-11-28" {
		t.Errorf("X-GitHub-Api-Version = %q, want %q", gotVersion, "2022-11-28")
	}
}

func TestFetchCommitInfoURL_InvalidJSONReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	_, err := fetchCommitInfoURL(srv.URL, "tok")
	if err == nil {
		t.Fatal("fetchCommitInfoURL() error = nil, want error for invalid JSON")
	}
}

// ─── fetchBlobURL ─────────────────────────────────────────────────────────────

func TestFetchBlobURL_DecodesBase64Content(t *testing.T) {
	raw := []byte("hello from template\n")
	encoded := base64.StdEncoding.EncodeToString(raw)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload := blobResponse{Content: encoded, Encoding: "base64"}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	got, err := fetchBlobURL(srv.URL, "tok")
	if err != nil {
		t.Fatalf("fetchBlobURL() error = %v", err)
	}
	if string(got) != string(raw) {
		t.Errorf("fetchBlobURL() = %q, want %q", got, raw)
	}
}

func TestFetchBlobURL_NonOKStatusReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := fetchBlobURL(srv.URL, "tok")
	if err == nil {
		t.Fatal("fetchBlobURL() error = nil, want error on 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error %q does not mention status code 404", err.Error())
	}
}

func TestFetchBlobURL_UnexpectedEncodingReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		payload := blobResponse{Content: "aGVsbG8=", Encoding: "utf-8"}
		_ = json.NewEncoder(w).Encode(payload)
	}))
	defer srv.Close()

	_, err := fetchBlobURL(srv.URL, "tok")
	if err == nil {
		t.Fatal("fetchBlobURL() error = nil, want error for non-base64 encoding")
	}
	if !strings.Contains(err.Error(), "encoding") {
		t.Errorf("error %q should mention encoding", err.Error())
	}
}

// ─── applyBoilerplateFile ─────────────────────────────────────────────────────

func TestApplyBoilerplateFile_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")
	content := []byte("template content\n")

	conflicts, err := applyBoilerplateFile(path, content)
	if err != nil {
		t.Fatalf("applyBoilerplateFile() error = %v", err)
	}
	if conflicts {
		t.Error("applyBoilerplateFile() reported conflicts for a new file")
	}

	got, _ := os.ReadFile(path) //nolint:gosec
	if string(got) != string(content) {
		t.Errorf("file content = %q, want %q", got, content)
	}
}

func TestApplyBoilerplateFile_CreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "subdir", "file.txt")

	_, err := applyBoilerplateFile(path, []byte("hello\n"))
	if err != nil {
		t.Fatalf("applyBoilerplateFile() error = %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("applyBoilerplateFile() did not create parent directories")
	}
}

func TestApplyBoilerplateFile_MergesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")

	// Existing file with two lines.
	existing := "line-a\nline-b\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil { //nolint:gosec
		t.Fatalf("setup: %v", err)
	}

	// Template is a superset: same two lines plus one new one.
	template := "line-a\nline-b\nline-c\n"

	// With an empty ancestor, the added line-c causes a conflict marker but
	// all content is present in the result.
	_, err := applyBoilerplateFile(path, []byte(template))
	if err != nil {
		t.Fatalf("applyBoilerplateFile() error = %v", err)
	}

	merged, _ := os.ReadFile(path) //nolint:gosec
	if !strings.Contains(string(merged), "line-a") {
		t.Error("merged file missing original 'line-a'")
	}
	if !strings.Contains(string(merged), "line-b") {
		t.Error("merged file missing original 'line-b'")
	}
	if !strings.Contains(string(merged), "line-c") {
		t.Error("merged file missing template addition 'line-c'")
	}
}

func TestApplyBoilerplateFile_IdenticalContentNoConflicts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")

	// When local file and template are identical, the merge is always clean.
	content := "word1\nword2\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec
		t.Fatalf("setup: %v", err)
	}

	conflicts, err := applyBoilerplateFile(path, []byte(content))
	if err != nil {
		t.Fatalf("applyBoilerplateFile() error = %v", err)
	}
	if conflicts {
		t.Error("applyBoilerplateFile() reported conflicts for identical content")
	}
}

func TestApplyBoilerplateFile_PreservesExistingContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")

	// Existing file has local content not present in the template.
	if err := os.WriteFile(path, []byte("local-setting\n"), 0o644); err != nil { //nolint:gosec
		t.Fatalf("setup: %v", err)
	}

	// Template has completely different content.
	// With an empty ancestor, both sides are "additions", so both lines
	// must appear in the result (possibly as conflict markers).
	_, err := applyBoilerplateFile(path, []byte("template-setting\n"))
	if err != nil {
		t.Fatalf("applyBoilerplateFile() error = %v", err)
	}

	merged, _ := os.ReadFile(path) //nolint:gosec
	// The local setting must still be present (inside conflict markers or merged).
	if !strings.Contains(string(merged), "local-setting") {
		t.Error("merge discarded existing local-setting – it should be preserved")
	}
	// The template setting must also be present.
	if !strings.Contains(string(merged), "template-setting") {
		t.Error("merge did not include template-setting from the template")
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
