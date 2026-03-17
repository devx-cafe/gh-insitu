package cmd

import (
	"fmt"
	"os"

	"github.com/devx-cafe/gh-insitu/internal/config"
	"github.com/spf13/cobra"
)

var planConfigFile string

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Validate configuration and print the resolved execution plan",
	Long: `Dry-run mode: validate the insitu YAML configuration and print the
resolved execution order, effective timeouts, and command overrides.

No checks are actually executed.

Examples:
  insitu plan
  insitu plan --config custom.yml`,
	RunE: func(_ *cobra.Command, _ []string) error {
		cfg, err := config.Load(planConfigFile)
		if err != nil {
			return fmt.Errorf("could not load config: %w", err)
		}

		printPlan(cfg)
		return nil
	},
}

func printPlan(cfg *config.Config) {
	w := os.Stdout
	inventory := cfg.InventoryMap()

	_, _ = fmt.Fprintln(w, "📋 Execution Plan")
	_, _ = fmt.Fprintln(w)

	if len(cfg.Waves) == 0 {
		_, _ = fmt.Fprintln(w, "  (no waves defined)")
		return
	}

	for i, wave := range cfg.Waves {
		mode := "sequential"
		if wave.Parallel {
			mode = "parallel"
		}
		_, _ = fmt.Fprintf(w, "Wave %d: %s  [%s]\n", i+1, waveName(wave), mode)

		for _, checkID := range wave.Checks {
			check, ok := inventory[checkID]
			if !ok {
				_, _ = fmt.Fprintf(w, "  ⚠️  %s  (unknown check id)\n", checkID)
				continue
			}
			resolved := cfg.ResolveCheck(check)
			dieLabel := ""
			if !resolved.EffectiveDieOnError {
				dieLabel = "  [die-on-error: false]"
			}
			_, _ = fmt.Fprintf(w, "  ▸ %-20s  timeout: %-8s  cmd: %s%s\n",
				check.DisplayName(), resolved.EffectiveTimeout, check.Command, dieLabel)
		}
		_, _ = fmt.Fprintln(w)
	}
}

func waveName(w config.Wave) string {
	if w.Name != "" {
		return w.Name
	}
	return w.ID
}

func init() {
	rootCmd.AddCommand(planCmd)
	planCmd.Flags().StringVarP(&planConfigFile, "config", "c", config.DefaultConfigFile,
		"Path to the insitu configuration file")
}
