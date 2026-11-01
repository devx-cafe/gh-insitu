package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/devx-cafe/gh-insitu/internal/config"
	"github.com/devx-cafe/gh-insitu/internal/runner"
	"github.com/devx-cafe/gh-insitu/internal/ui"
	"github.com/spf13/cobra"
)

var (
	taskConfigFile string
	taskTimeout    string
	taskDieOnError bool
	taskVerbose    bool
	taskPlan       bool
)

var taskCmd = &cobra.Command{
	Use:   "task <inventory-id>",
	Short: "Execute one inventory entry directly",
	Long: `Execute one inventory entry directly, whether or not a wave references it.

Use --timeout and --die-on-error to override the effective values for this
invocation. Use --plan to preview the resolved task without executing it.

Examples:
  insitu task build
  insitu task build --timeout 30s
  insitu task build --die-on-error=false
  insitu task --plan build`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(taskConfigFile)
		if err != nil {
			return fmt.Errorf("could not load config: %w", err)
		}

		resolved, err := resolveTask(cfg, args[0], taskTimeout, cmd.Flags().Changed("timeout"),
			taskDieOnError, cmd.Flags().Changed("die-on-error"))
		if err != nil {
			return err
		}
		if taskPlan {
			printTaskPlan(os.Stdout, resolved)
			return nil
		}

		verbose := taskVerbose
		if !cmd.Flags().Changed("verbose") {
			verbose = os.Getenv("GITHUB_ACTIONS") == githubActionsEnv
		}
		formatter := ui.NewFormatter(os.Stdout, verbose)
		r := runner.New(cfg, formatter)
		if os.Getenv("GITHUB_ACTIONS") == githubActionsEnv {
			r.OnCheckDone = buildStatusReporter()
		}

		result := r.RunTask(resolved)
		formatter.PrintSummary(result.Success())
		if !result.Success() {
			return fmt.Errorf("task %q failed", args[0])
		}
		return nil
	},
}

func resolveTask(cfg *config.Config, id, timeout string, timeoutSet, dieOnError, dieOnErrorSet bool) (config.ResolvedCheck, error) {
	check, ok := cfg.InventoryMap()[id]
	if !ok {
		return config.ResolvedCheck{}, fmt.Errorf("inventory entry %q not found in configuration", id)
	}

	resolved := cfg.ResolveCheck(check)
	if timeoutSet {
		duration, err := time.ParseDuration(timeout)
		if err != nil {
			return config.ResolvedCheck{}, fmt.Errorf("invalid --timeout %q: %w", timeout, err)
		}
		resolved.EffectiveTimeout = duration
	}
	if dieOnErrorSet {
		resolved.EffectiveDieOnError = dieOnError
	}
	return resolved, nil
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.Flags().StringVarP(&taskConfigFile, "config", "c", config.DefaultConfigFile,
		"Path to the insitu configuration file")
	taskCmd.Flags().StringVar(&taskTimeout, "timeout", "",
		"Override the effective task timeout (Go duration, e.g. 30s)")
	taskCmd.Flags().BoolVar(&taskDieOnError, "die-on-error", false,
		"Override the effective die-on-error setting")
	taskCmd.Flags().BoolVar(&taskVerbose, "verbose", false,
		"Print command output even when the task passes (default: true in GitHub Actions)")
	taskCmd.Flags().BoolVarP(&taskPlan, "plan", "p", false,
		"Print the task execution plan without running the command")
}
