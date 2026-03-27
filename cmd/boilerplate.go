package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	gh "github.com/devx-cafe/gh-insitu/internal/github"
	"github.com/spf13/cobra"
)

// gitHubCommitsBase is the URL template for the GitHub Commits API.
// It can be overridden in tests via fetchCommitDiffURL.
const gitHubCommitsBase = "https://api.github.com/repos/%s/commits/%s"

var (
	boilerplateRepo       string
	boilerplateRef        string
	boilerplateAllowDirty bool
)

var boilerplateCmd = &cobra.Command{
	Use:   "boilerplate",
	Short: "Apply a commit from another repo as a template patch",
	Long: `Fetch the tip commit of a branch (or any ref) from a remote GitHub
repository and apply its diff to the current working tree.

This is the equivalent of:
  git show <sha> --no-color > template.patch
  git apply --reject template.patch

Files that already exist will be merged where possible. Conflicts are
saved as .rej files for manual resolution.

By default the command refuses to run when the working tree is dirty or has
staged changes. Use --allow-dirty to skip that check.

A GitHub token must be available via GH_TOKEN or GITHUB_TOKEN.

Examples:
  insitu boilerplate --repo lakruzz/boilerplates --ref cspell
  insitu boilerplate --repo org/boilerplates --ref main --allow-dirty`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if boilerplateRepo == "" {
			return fmt.Errorf("--repo is required")
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

		_, _ = fmt.Fprintf(os.Stdout, "⬇️  Fetching diff from %s@%s…\n", boilerplateRepo, boilerplateRef)

		diff, err := fetchCommitDiff(boilerplateRepo, boilerplateRef, token)
		if err != nil {
			return fmt.Errorf("failed to fetch diff: %w", err)
		}

		if strings.TrimSpace(diff) == "" {
			_, _ = fmt.Fprintln(os.Stdout, "ℹ️  The commit has no file changes – nothing to apply")
			return nil
		}

		tmpFile, err := os.CreateTemp("", "insitu-boilerplate-*.patch")
		if err != nil {
			return fmt.Errorf("failed to create temp patch file: %w", err)
		}
		defer func() { _ = os.Remove(tmpFile.Name()) }()

		if _, err := tmpFile.WriteString(diff); err != nil {
			_ = tmpFile.Close()
			return fmt.Errorf("failed to write patch file: %w", err)
		}
		if err := tmpFile.Close(); err != nil {
			return fmt.Errorf("failed to close patch file: %w", err)
		}

		_, _ = fmt.Fprintln(os.Stdout, "📋 Applying patch (conflicts saved as .rej files)…")
		if err := applyPatch(tmpFile.Name()); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "⚠️  Some hunks failed to apply – review the .rej files for conflicts")
			return err
		}

		_, _ = fmt.Fprintln(os.Stdout, "✅ Patch applied successfully")
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

// fetchCommitDiff fetches the unified diff of the tip commit identified by ref
// from the given GitHub repository. token must be a valid GitHub token.
func fetchCommitDiff(repo, ref, token string) (string, error) {
	return fetchCommitDiffURL(fmt.Sprintf(gitHubCommitsBase, repo, ref), token)
}

// fetchCommitDiffURL is like fetchCommitDiff but accepts a fully-formed URL.
// This variant exists for testing.
func fetchCommitDiffURL(url, token string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.diff")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call GitHub API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return string(body), nil
}

// applyPatch runs `git apply --reject <patchFile>` in the current directory.
// Failed hunks are saved as .rej files for manual resolution.
func applyPatch(patchFile string) error {
	cmd := exec.Command("git", "apply", "--reject", patchFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git apply failed: %w", err)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(boilerplateCmd)
	boilerplateCmd.Flags().StringVar(&boilerplateRepo, "repo", "",
		"Source repository in owner/repo format (required)")
	boilerplateCmd.Flags().StringVar(&boilerplateRef, "ref", "HEAD",
		"Branch, tag, or commit SHA to fetch (default: HEAD)")
	boilerplateCmd.Flags().BoolVar(&boilerplateAllowDirty, "allow-dirty", false,
		"Apply even if the working tree has uncommitted changes")
}
