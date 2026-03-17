package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "insitu",
	Short: "Self-contained parallel task runner for GitHub CLI",
	Long: `insitu is a GitHub CLI extension that brings parity between local
development and CI environments through a repository-native "Inventory
and Wave" system.

Define checks once in .insitu.yml and run them identically both locally
and inside GitHub Actions workflows.

Run 'insitu init' to bootstrap a starter configuration.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
}
