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

	conflicts, err := applyBoilerplateFile(path, content, "merge")
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

	_, err := applyBoilerplateFile(path, []byte("hello\n"), "merge")
	if err != nil {
		t.Fatalf("applyBoilerplateFile() error = %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("applyBoilerplateFile() did not create parent directories")
	}
}

// ─── merge strategy ───────────────────────────────────────────────────────────

// TestMerge_PreservesLocalContent verifies that the merge strategy keeps all
// existing local content intact (source-as-base semantics).
func TestMerge_PreservesLocalContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")

	// Local file has two lines.
	existing := "line-a\nline-b\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil { //nolint:gosec
		t.Fatalf("setup: %v", err)
	}

	// Template is the same as local (acts as the common ancestor).
	// With source-as-base, local content is preserved unchanged.
	template := "line-a\nline-b\n"

	conflicts, err := applyBoilerplateFile(path, []byte(template), "merge")
	if err != nil {
		t.Fatalf("applyBoilerplateFile() error = %v", err)
	}
	if conflicts {
		t.Errorf("applyBoilerplateFile() reported conflicts – want clean merge")
	}

	got, _ := os.ReadFile(path) //nolint:gosec
	for _, want := range []string{"line-a", "line-b"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("merged file missing %q", want)
		}
	}
}

// TestMerge_LocalOnlyLinesAreKept verifies that lines in the local file that
// are not in the template (i.e. local additions relative to the template) are
// preserved after the merge.
func TestMerge_LocalOnlyLinesAreKept(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")

	// Local file has an extra line that the template does not have.
	existing := "line-a\nline-b\nlocal-only\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil { //nolint:gosec
		t.Fatalf("setup: %v", err)
	}

	// Template is a subset of local.
	template := "line-a\nline-b\n"

	conflicts, err := applyBoilerplateFile(path, []byte(template), "merge")
	if err != nil {
		t.Fatalf("applyBoilerplateFile() error = %v", err)
	}
	if conflicts {
		t.Errorf("applyBoilerplateFile() reported conflicts – want clean merge")
	}

	got, _ := os.ReadFile(path) //nolint:gosec
	if !strings.Contains(string(got), "local-only") {
		t.Error("merge strategy deleted local-only line – it must be preserved")
	}
}

// TestMerge_IdenticalContentNoConflicts verifies idempotency.
func TestMerge_IdenticalContentNoConflicts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")

	content := "word1\nword2\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec
		t.Fatalf("setup: %v", err)
	}

	conflicts, err := applyBoilerplateFile(path, []byte(content), "merge")
	if err != nil {
		t.Fatalf("applyBoilerplateFile() error = %v", err)
	}
	if conflicts {
		t.Error("applyBoilerplateFile() reported conflicts for identical content")
	}
}

// ─── combine strategy ─────────────────────────────────────────────────────────

// TestCombine_UnionOfBothSides verifies that combine produces all lines from
// both the local file and the template without conflict markers.
func TestCombine_UnionOfBothSides(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")

	existing := "line-a\nline-b\n"
	if err := os.WriteFile(path, []byte(existing), 0o644); err != nil { //nolint:gosec
		t.Fatalf("setup: %v", err)
	}

	template := "line-c\nline-d\n"

	conflicts, err := applyBoilerplateFile(path, []byte(template), "combine")
	if err != nil {
		t.Fatalf("applyBoilerplateFile(combine) error = %v", err)
	}
	if conflicts {
		t.Error("combine strategy should never produce conflict markers")
	}

	got, _ := os.ReadFile(path) //nolint:gosec
	for _, want := range []string{"line-a", "line-b", "line-c", "line-d"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("combine result missing %q", want)
		}
	}
	if strings.Contains(string(got), "<<<<<<<") {
		t.Error("combine result contains conflict markers")
	}
}

// TestCombine_IdenticalContentNoConflicts verifies idempotency.
func TestCombine_IdenticalContentNoConflicts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")

	content := "word1\nword2\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil { //nolint:gosec
		t.Fatalf("setup: %v", err)
	}

	conflicts, err := applyBoilerplateFile(path, []byte(content), "combine")
	if err != nil {
		t.Fatalf("applyBoilerplateFile(combine) error = %v", err)
	}
	if conflicts {
		t.Error("combine strategy reported conflicts for identical content")
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
