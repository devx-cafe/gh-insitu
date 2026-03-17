// Package cmd provides command-line interface commands for the gh-insitu extension.
package cmd

import (
	"fmt"
	"os"

	"github.com/devx-cafe/gh-insitu/internal/config"
	"github.com/devx-cafe/gh-insitu/internal/runner"
	"github.com/devx-cafe/gh-insitu/internal/ui"
	"github.com/spf13/cobra"
)

var runConfigFile string

var runCmd = &cobra.Command{
	Use:   "run [wave-id...]",
	Short: "Execute one or more waves of checks",
	Long: `Execute checks defined in .insitu.yml.

If no wave IDs are provided all waves are executed in order.
Waves with parallel: true run their checks concurrently.

Examples:
  insitu run                  # run all waves
  insitu run static           # run only the 'static' wave
  insitu run static test      # run 'static' then 'test'`,
	RunE: func(_ *cobra.Command, args []string) error {
		cfg, err := config.Load(runConfigFile)
		if err != nil {
			return fmt.Errorf("could not load config: %w", err)
		}

		formatter := ui.NewFormatter(os.Stdout)
		r := runner.New(cfg, formatter)

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

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVarP(&runConfigFile, "config", "c", config.DefaultConfigFile,
		"Path to the insitu configuration file")
}
