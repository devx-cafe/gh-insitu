package cmd

import (
	"fmt"
	"io"

	"github.com/devx-cafe/gh-insitu/internal/config"
)

func selectWaves(cfg *config.Config, waveIDs []string) ([]config.Wave, error) {
	waves := make([]config.Wave, 0, len(waveIDs))
	for _, id := range waveIDs {
		wave, ok := cfg.GetWave(id)
		if !ok {
			return nil, fmt.Errorf("wave %q not found in configuration", id)
		}
		waves = append(waves, *wave)
	}
	return waves, nil
}

func printWavePlan(w io.Writer, cfg *config.Config, waves []config.Wave) {
	inventory := cfg.InventoryMap()
	_, _ = fmt.Fprintln(w, "📋 Execution Plan")
	_, _ = fmt.Fprintln(w)

	for i, wave := range waves {
		mode := "sequential"
		if wave.Parallel {
			mode = "parallel"
		}
		_, _ = fmt.Fprintf(w, "Wave %d: %s  [%s]\n", i+1, waveName(wave), mode)

		for _, checkID := range wave.Checks {
			check := inventory[checkID]
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

func printTaskPlan(w io.Writer, check config.ResolvedCheck) {
	_, _ = fmt.Fprintln(w, "📋 Task Plan")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "  ▸ %-20s  timeout: %-8s  cmd: %s  [die-on-error: %t]\n",
		check.DisplayName(), check.EffectiveTimeout, check.Command, check.EffectiveDieOnError)
}

func waveName(w config.Wave) string {
	if w.Name != "" {
		return w.Name
	}
	return w.ID
}
