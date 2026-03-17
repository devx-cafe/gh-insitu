package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// defaultInsituYML is written by `insitu init` when no config exists.
const defaultInsituYML = `# .insitu.yml – insitu parallel task runner configuration
# Run 'insitu plan' to validate this file.
# Run 'insitu run' to execute all waves.

defaults:
  die-on-error: true
  timeout: 5m
  verbose: false

# Inventory: define every check once here.
inventory:
  - id: "build"
    name: "Build"
    command: "make build"

  - id: "coverage"
    name: "Unit Test with Coverage"
    command: "make coverage"

# Waves: compose checks into ordered execution groups.
waves:
  - id: "static"
    name: "Static Analysis & Build"
    parallel: true
    checks:
      - "build"

  - id: "test"
    name: "Post-Build Validation"
    parallel: true
    checks:
      - "coverage"
`

// preCommitHook is written by `insitu init` into .git/hooks/pre-commit.
const preCommitHook = `#!/bin/sh
# Pre-commit hook installed by 'insitu init'.
# Runs all insitu waves before allowing a commit.
insitu run
`

var initConfigFile string
var initForce bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Bootstrap .insitu.yml and install the pre-commit hook",
	Long: `Create a starter .insitu.yml configuration file and install a
.git/hooks/pre-commit hook that runs 'insitu run' before every commit.

Use --force to overwrite an existing configuration file.

Examples:
  insitu init
  insitu init --force
  insitu init --config custom.yml`,
	RunE: func(_ *cobra.Command, _ []string) error {
		if err := writeConfig(initConfigFile, initForce); err != nil {
			return err
		}
		if err := writePreCommitHook(); err != nil {
			return err
		}
		w := os.Stdout
		_, _ = fmt.Fprintln(w, "✅ insitu initialised")
		_, _ = fmt.Fprintf(w, "   config  → %s\n", initConfigFile)
		_, _ = fmt.Fprintln(w, "   hook    → .git/hooks/pre-commit")
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "Edit .insitu.yml to define your checks and waves, then run:")
		_, _ = fmt.Fprintln(w, "   insitu plan   # validate and preview")
		_, _ = fmt.Fprintln(w, "   insitu run    # execute all waves")
		return nil
	},
}

func writeConfig(path string, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		_, _ = fmt.Fprintf(os.Stdout, "⚠️  %s already exists (use --force to overwrite)\n", path)
		return nil
	}
	// 0o644 is intentional: config file is repo-committed and should be readable by all
	if err := os.WriteFile(path, []byte(defaultInsituYML), 0o644); err != nil { //nolint:gosec
		return fmt.Errorf("failed to write %s: %w", path, err)
	}
	return nil
}

func writePreCommitHook() error {
	const hookPath = ".git/hooks/pre-commit"
	// 0o755 is intentional: hook directory must be traversable and hook must be executable
	if err := os.MkdirAll(".git/hooks", 0o755); err != nil { //nolint:gosec
		return fmt.Errorf("failed to create .git/hooks: %w", err)
	}
	if err := os.WriteFile(hookPath, []byte(preCommitHook), 0o755); err != nil { //nolint:gosec
		return fmt.Errorf("failed to write pre-commit hook: %w", err)
	}
	return nil
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringVarP(&initConfigFile, "config", "c", ".insitu.yml",
		"Path for the generated configuration file")
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false,
		"Overwrite the configuration file if it already exists")
}
