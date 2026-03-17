package cmd

import (
	"fmt"
	"os"

	gh "github.com/devx-cafe/gh-insitu/internal/github"
	"github.com/spf13/cobra"
)

var (
	statusRepo        string
	statusSHA         string
	statusDescription string
	statusTargetURL   string
)

var setStatusCmd = &cobra.Command{
	Use:   "set-status <state> <context>",
	Short: "Set a GitHub commit status",
	Long: `Set a commit status on GitHub without requiring the gh-set-status extension.

STATE must be one of: pending, success, failure, error

The commit SHA and repository are automatically detected from:
  - GITHUB_SHA / GITHUB_REPOSITORY environment variables (in GitHub Actions)
  - git rev-parse HEAD / gh repo view (locally)

A GitHub token must be available via GH_TOKEN or GITHUB_TOKEN.

Examples:
  insitu set-status success "ci/build"
  insitu set-status pending "ci/lint" --description "Running lint checks..."
  insitu set-status failure "ci/test" --sha abc123 --repo owner/repo`,
	Args: cobra.ExactArgs(2),
	RunE: func(_ *cobra.Command, args []string) error {
		state := gh.State(args[0])
		context := args[1]

		switch state {
		case gh.StatePending, gh.StateSuccess, gh.StateFailure, gh.StateError:
			// valid
		default:
			return fmt.Errorf("invalid state %q: must be one of pending, success, failure, error", state)
		}

		repo := statusRepo
		if repo == "" {
			var err error
			repo, err = gh.CurrentRepo()
			if err != nil {
				return fmt.Errorf("could not determine repository: %w", err)
			}
		}

		sha := statusSHA
		if sha == "" {
			var err error
			sha, err = gh.CurrentSHA()
			if err != nil {
				return fmt.Errorf("could not determine commit SHA: %w", err)
			}
		}

		token := gh.Token()
		if token == "" {
			return fmt.Errorf("no GitHub token found; set GH_TOKEN or GITHUB_TOKEN")
		}

		if err := gh.SetCommitStatus(repo, sha, token, state, statusDescription, context); err != nil {
			return fmt.Errorf("failed to set commit status: %w", err)
		}

		_, _ = fmt.Fprintf(os.Stdout, "✅ Set %s → %s on %s@%s\n", context, state, repo, sha[:min(len(sha), 8)])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setStatusCmd)
	setStatusCmd.Flags().StringVar(&statusRepo, "repo", "", "Repository in owner/repo format (auto-detected when omitted)")
	setStatusCmd.Flags().StringVar(&statusSHA, "sha", "", "Commit SHA (auto-detected when omitted)")
	setStatusCmd.Flags().StringVar(&statusDescription, "description", "", "Short description shown alongside the status")
	setStatusCmd.Flags().StringVar(&statusTargetURL, "target-url", "", "URL to link from the status check")
}
