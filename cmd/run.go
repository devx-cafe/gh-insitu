// Package cmd provides command-line interface commands for the gh-insitu extension.
package cmd

import (
	"fmt"
	"os"

	"github.com/devx-cafe/gh-insitu/internal/config"
	gh "github.com/devx-cafe/gh-insitu/internal/github"
	"github.com/devx-cafe/gh-insitu/internal/runner"
	"github.com/devx-cafe/gh-insitu/internal/ui"
	"github.com/spf13/cobra"
)

var (
	runConfigFile string
	markPending   bool
)

var runCmd = &cobra.Command{
	Use:   "run [wave-id...]",
	Short: "Execute one or more waves of checks",
	Long: `Execute checks defined in .insitu.yml.

If no wave IDs are provided all waves are executed in order.
Waves with parallel: true run their checks concurrently.

When run inside a GitHub Actions workflow (GITHUB_ACTIONS=true) each check
result is automatically reported as a commit status using the check's id as
the status context.

Use --mark-pending to mark all checks in the selected wave(s) as "pending"
without executing them.  This is only valid inside a GitHub Actions workflow
and is typically run as an early step – before the real 'insitu run' – so
the commit statuses appear as pending while the workflow is in progress.

Examples:
  insitu run                          # run all waves
  insitu run static                   # run only the 'static' wave
  insitu run static test              # run 'static' then 'test'
  insitu run --mark-pending           # mark all wave checks as pending
  insitu run trunk-worthy --mark-pending  # mark trunk-worthy checks as pending`,
	RunE: func(_ *cobra.Command, args []string) error {
		cfg, err := config.Load(runConfigFile)
		if err != nil {
			return fmt.Errorf("could not load config: %w", err)
		}

		formatter := ui.NewFormatter(os.Stdout)
		r := runner.New(cfg, formatter)

		if markPending {
			return runMarkPending(r, args)
		}

		// Wire up GitHub commit status reporting when running inside a workflow.
		if os.Getenv("GITHUB_ACTIONS") == "true" {
			r.OnCheckDone = buildStatusReporter()
		}

		results, err := r.RunWaves(args)
		if err != nil {
			return err
		}

		allPassed := true
		for _, wr := range results {
			if !wr.Success() {
				allPassed = false
				break
			}
		}

		formatter.PrintSummary(allPassed)

		if !allPassed {
			os.Exit(1)
		}
		return nil
	},
}

// buildStatusReporter returns a callback that posts a GitHub commit status for
// each completed check.  Errors are silently ignored so a missing token or
// unreachable API does not abort the run.
func buildStatusReporter() func(id, displayName string, passed bool) {
	repo := os.Getenv("GITHUB_REPOSITORY")
	sha := os.Getenv("GITHUB_SHA")
	token := gh.Token()

	return func(id, displayName string, passed bool) {
		if repo == "" || sha == "" || token == "" {
			return
		}
		state := gh.StateSuccess
		desc := displayName + " check passed"
		if !passed {
			state = gh.StateFailure
			desc = displayName + " check failed"
		}
		_ = gh.SetCommitStatus(repo, sha, token, state, desc, id)
	}
}

// runMarkPending resolves all checks for the selected wave(s) and marks each
// one as "pending" via the GitHub Statuses API.
func runMarkPending(r *runner.Runner, waveIDs []string) error {
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		return fmt.Errorf("--mark-pending is only valid inside a GitHub Actions workflow")
	}

	repo := os.Getenv("GITHUB_REPOSITORY")
	sha := os.Getenv("GITHUB_SHA")
	token := gh.Token()

	if repo == "" || sha == "" || token == "" {
		return fmt.Errorf("--mark-pending requires GITHUB_REPOSITORY, GITHUB_SHA, and GH_TOKEN/GITHUB_TOKEN to be set")
	}

	checks, err := r.ResolveChecks(waveIDs)
	if err != nil {
		return err
	}

	for _, c := range checks {
		if setErr := gh.SetCommitStatus(repo, sha, token, gh.StatePending, c.DisplayName()+" check", c.ID); setErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "⚠️  could not mark %s as pending: %v\n", c.ID, setErr)
		} else {
			_, _ = fmt.Fprintf(os.Stdout, "⏳ %s marked pending\n", c.DisplayName())
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&runConfigFile, "config", "c", config.DefaultConfigFile,
		"Path to the insitu configuration file")
	runCmd.Flags().BoolVar(&markPending, "mark-pending", false,
		"Mark all checks in the selected wave(s) as 'pending' without running them (GitHub Actions only)")
}
