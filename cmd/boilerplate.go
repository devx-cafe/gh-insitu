package cmd

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gh "github.com/devx-cafe/gh-insitu/internal/github"
	"github.com/spf13/cobra"
)

// URL templates for the GitHub REST API.
const (
	gitHubCommitsBase = "https://api.github.com/repos/%s/commits/%s"
	gitHubBlobBase    = "https://api.github.com/repos/%s/git/blobs/%s"
)

// commitInfo holds the subset of the GitHub Commits API response that insitu needs.
type commitInfo struct {
	SHA   string       `json:"sha"`
	Files []commitFile `json:"files"`
}

// commitFile describes a single file touched by a commit.
type commitFile struct {
	SHA      string `json:"sha"`      // blob SHA of the new version of the file
	Filename string `json:"filename"` // relative path in the repository
	Status   string `json:"status"`   // added | modified | removed | renamed | copied
}

// blobResponse is the payload returned by the GitHub Blobs API.
type blobResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

var (
	boilerplateRepo       string
	boilerplateRef        string
	boilerplateAllowDirty bool
	boilerplateStrategy   string
)

var boilerplateCmd = &cobra.Command{
	Use:   "boilerplate",
	Short: "Apply a commit from another repo as a boilerplate template",
	Long: `Fetch the files changed by the tip commit of a branch (or any ref) from a
remote GitHub repository and apply them to the current working tree.

  - Files that do not yet exist locally are written directly (parent
    directories are created as needed).
  - Files that already exist are merged or injected according to --strategy.
  - Files removed by the template commit are skipped.

Merge strategies (--strategy):

  merge (default)
    Performs a git 3-way merge with the existing file used as both the base
    and the current side. The template is the "other" side. Because the base
    and the current are identical, git sees only the template's additions and
    applies them cleanly – guaranteed no conflicts as long as the template
    never removes content from the target.

  inject
    Programmatic, format-aware deep-merge. For each key or array item present
    in the template, the value is added or updated in the existing file.
    Supported formats: .json, .jsonc (comments are stripped on write),
    .yml, .yaml. Other file types fall back to the merge strategy.
    This approach never produces conflict markers.

By default the command refuses to run when the working tree is dirty or has
staged changes. Use --allow-dirty to skip that check.

A GitHub token must be available via GH_TOKEN or GITHUB_TOKEN.

Examples:
  insitu boilerplate --repo lakruzz/boilerplates --ref cspell
  insitu boilerplate --repo org/boilerplates --ref main --allow-dirty
  insitu boilerplate --repo org/boilerplates --ref cspell --strategy inject`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if boilerplateRepo == "" {
			return fmt.Errorf("--repo is required")
		}
		if boilerplateStrategy != "merge" && boilerplateStrategy != "inject" {
			return fmt.Errorf("--strategy must be 'merge' or 'inject', got %q", boilerplateStrategy)
		}

		if !boilerplateAllowDirty {
			if err := checkCleanWorkingTree(); err != nil {
				return err
			}
		}

		token := gh.Token()
		if token == "" {
			return fmt.Errorf("no GitHub token found; set GH_TOKEN or GITHUB_TOKEN")
		}

		_, _ = fmt.Fprintf(os.Stdout, "⬇️  Fetching boilerplate commit from %s@%s…\n", boilerplateRepo, boilerplateRef)

		commit, err := fetchCommitInfo(boilerplateRepo, boilerplateRef, token)
		if err != nil {
			return fmt.Errorf("failed to fetch commit: %w", err)
		}

		if len(commit.Files) == 0 {
			_, _ = fmt.Fprintln(os.Stdout, "ℹ️  The commit has no file changes – nothing to apply")
			return nil
		}

		var applied, skipped, conflicted int
		for _, f := range commit.Files {
			if f.Status == "removed" {
				_, _ = fmt.Fprintf(os.Stdout, "⏭️  %-50s skipped (removal)\n", f.Filename)
				skipped++
				continue
			}

			content, err := fetchBlob(boilerplateRepo, f.SHA, token)
			if err != nil {
				return fmt.Errorf("failed to fetch blob for %s: %w", f.Filename, err)
			}

			hadConflicts, err := applyBoilerplateFile(f.Filename, content, boilerplateStrategy)
			if err != nil {
				return fmt.Errorf("failed to apply %s: %w", f.Filename, err)
			}

			if hadConflicts {
				_, _ = fmt.Fprintf(os.Stdout, "⚠️  %-50s merged with conflicts\n", f.Filename)
				conflicted++
			} else {
				_, _ = fmt.Fprintf(os.Stdout, "✅ %-50s applied\n", f.Filename)
				applied++
			}
		}

		_, _ = fmt.Fprintln(os.Stdout)
		if conflicted > 0 {
			_, _ = fmt.Fprintf(os.Stdout, "⚠️  Done: %d applied, %d with conflicts, %d skipped – resolve conflict markers manually\n",
				applied, conflicted, skipped)
		} else {
			_, _ = fmt.Fprintf(os.Stdout, "✅ Done: %d applied, %d skipped\n", applied, skipped)
		}
		return nil
	},
}

// checkCleanWorkingTree returns an error when the working tree has staged or
// unstaged changes. It wraps `git status --porcelain`.
func checkCleanWorkingTree() error {
	out, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return fmt.Errorf("failed to check git status: %w", err)
	}
	if strings.TrimSpace(string(out)) != "" {
		return fmt.Errorf("working tree is dirty; commit or stash your changes, or use --allow-dirty")
	}
	return nil
}

// fetchCommitInfo retrieves commit metadata (SHA and file list) from GitHub.
func fetchCommitInfo(repo, ref, token string) (*commitInfo, error) {
	return fetchCommitInfoURL(fmt.Sprintf(gitHubCommitsBase, repo, ref), token)
}

// fetchCommitInfoURL is like fetchCommitInfo but accepts a fully-formed URL
// so that tests can point at a local httptest server.
func fetchCommitInfoURL(url, token string) (*commitInfo, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call GitHub API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var info commitInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("failed to parse commit JSON: %w", err)
	}
	return &info, nil
}

// fetchBlob retrieves the decoded byte content of a git blob from GitHub.
func fetchBlob(repo, sha, token string) ([]byte, error) {
	return fetchBlobURL(fmt.Sprintf(gitHubBlobBase, repo, sha), token)
}

// fetchBlobURL is like fetchBlob but accepts a fully-formed URL for testing.
func fetchBlobURL(url, token string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call GitHub API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var blob blobResponse
	if err := json.Unmarshal(body, &blob); err != nil {
		return nil, fmt.Errorf("failed to parse blob JSON: %w", err)
	}

	if blob.Encoding != "base64" {
		return nil, fmt.Errorf("unexpected blob encoding %q (expected base64)", blob.Encoding)
	}

	// GitHub wraps base64 at 60 chars with newlines; strip them before decoding.
	cleaned := strings.ReplaceAll(blob.Content, "\n", "")
	content, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, fmt.Errorf("failed to decode blob content: %w", err)
	}
	return content, nil
}

// applyBoilerplateFile writes or merges a single boilerplate file into the
// working tree.
//
// If the file does not exist it is created (parent directories are made as
// needed). If it already exists, strategy controls how to merge:
//
//   - "merge": git 3-way merge with the target used as the common ancestor
//   - "inject": programmatic format-aware deep-merge (falls back to merge for
//     unsupported file types)
//
// Returns hadConflicts=true only when git merge-file left inline conflict markers.
func applyBoilerplateFile(path string, content []byte, strategy string) (hadConflicts bool, err error) {
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr != nil { //nolint:gosec
			return false, fmt.Errorf("failed to create directory for %s: %w", path, mkErr)
		}
		if writeErr := os.WriteFile(path, content, 0o644); writeErr != nil { //nolint:gosec
			return false, fmt.Errorf("failed to write %s: %w", path, writeErr)
		}
		return false, nil
	}

	if strategy == "inject" {
		return injectIntoExisting(path, content)
	}
	return mergeIntoExisting(path, content)
}

// mergeIntoExisting performs a conflict-free git 3-way merge of the template
// content into a file that already exists on disk.
//
// Strategy: the existing file is used as both the current side AND the common
// ancestor. The template is the "other" side. Because base == current, git
// sees no local modifications – only the template's additions are visible and
// applied cleanly. As long as the template never removes content already in
// the target, this is guaranteed to produce zero merge conflicts.
//
// git merge-file exit codes: 0 = clean, positive = N conflict regions, negative = error.
func mergeIntoExisting(path string, templateContent []byte) (hadConflicts bool, err error) {
	existing, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return false, fmt.Errorf("failed to read %s: %w", path, err)
	}

	// Write the template content to a temp file ("other" side of the merge).
	tmpOther, err := os.CreateTemp("", "insitu-bp-other-*")
	if err != nil {
		return false, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpOther.Name()) }()
	if _, err := tmpOther.Write(templateContent); err != nil {
		_ = tmpOther.Close()
		return false, fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmpOther.Close(); err != nil {
		return false, fmt.Errorf("failed to close temp file: %w", err)
	}

	// Copy existing → tmpCurrent (git merge-file overwrites the first arg in
	// place, so we work on a copy and write the result back at the end).
	tmpCurrent, err := os.CreateTemp("", "insitu-bp-current-*")
	if err != nil {
		return false, fmt.Errorf("failed to create current temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpCurrent.Name()) }()
	if _, err := tmpCurrent.Write(existing); err != nil {
		_ = tmpCurrent.Close()
		return false, fmt.Errorf("failed to write current temp file: %w", err)
	}
	if err := tmpCurrent.Close(); err != nil {
		return false, fmt.Errorf("failed to close current temp file: %w", err)
	}

	// Copy existing → tmpBase (the common ancestor).
	// With base == current, git sees no local changes and only applies what
	// the template adds; result is always conflict-free.
	tmpBase, err := os.CreateTemp("", "insitu-bp-base-*")
	if err != nil {
		return false, fmt.Errorf("failed to create base temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpBase.Name()) }()
	if _, err := tmpBase.Write(existing); err != nil {
		_ = tmpBase.Close()
		return false, fmt.Errorf("failed to write base temp file: %w", err)
	}
	if err := tmpBase.Close(); err != nil {
		return false, fmt.Errorf("failed to close base temp file: %w", err)
	}

	// git merge-file <current> <base> <other> — modifies <current> in place.
	cmd := exec.Command("git", "merge-file", tmpCurrent.Name(), tmpBase.Name(), tmpOther.Name())
	runErr := cmd.Run()

	// Read the result (possibly with conflict markers) and overwrite the real file.
	merged, readErr := os.ReadFile(tmpCurrent.Name()) //nolint:gosec
	if readErr != nil {
		return false, fmt.Errorf("failed to read merge result for %s: %w", path, readErr)
	}
	if writeErr := os.WriteFile(path, merged, 0o644); writeErr != nil { //nolint:gosec
		return false, fmt.Errorf("failed to write merge result for %s: %w", path, writeErr)
	}

	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) && exitErr.ExitCode() > 0 {
			// Positive exit code = number of conflict regions.
			return true, nil
		}
		return false, fmt.Errorf("git merge-file failed for %s: %w", path, runErr)
	}
	return false, nil
}

func init() {
	rootCmd.AddCommand(boilerplateCmd)
	boilerplateCmd.Flags().StringVar(&boilerplateRepo, "repo", "",
		"Source repository in owner/repo format (required)")
	boilerplateCmd.Flags().StringVar(&boilerplateRef, "ref", "HEAD",
		"Branch, tag, or commit SHA to fetch (default: HEAD)")
	boilerplateCmd.Flags().BoolVar(&boilerplateAllowDirty, "allow-dirty", false,
		"Apply even if the working tree has uncommitted changes")
	boilerplateCmd.Flags().StringVar(&boilerplateStrategy, "strategy", "merge",
		"How to apply files that already exist: merge (git 3-way, target-as-base) or inject (programmatic, format-aware)")
}
