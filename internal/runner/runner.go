// Package runner executes insitu waves and checks.
package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/devx-cafe/gh-insitu/internal/config"
	"github.com/devx-cafe/gh-insitu/internal/ui"
)

// CheckResult holds the outcome of a single check execution.
type CheckResult struct {
	Check    config.ResolvedCheck
	Output   []byte
	ExitCode int
	Duration time.Duration
	Err      error
}

// Success reports whether the check passed (exit code 0, no execution error).
func (r CheckResult) Success() bool {
	return r.Err == nil && r.ExitCode == 0
}

// WaveResult holds the aggregated results for an entire wave.
type WaveResult struct {
	Wave    config.Wave
	Results []CheckResult
}

// Success reports whether every check in the wave passed.
func (w WaveResult) Success() bool {
	for _, r := range w.Results {
		if !r.Success() {
			return false
		}
	}
	return true
}

// Runner executes configuration waves using the provided formatter for output.
type Runner struct {
	Config      *config.Config
	Formatter   ui.Formatter
	OnCheckDone func(id, displayName string, passed bool) // called after each check; nil = no-op
}

// New creates a Runner for the given configuration and formatter.
func New(cfg *config.Config, formatter ui.Formatter) *Runner {
	return &Runner{
		Config:    cfg,
		Formatter: formatter,
	}
}

// RunWaves executes the named waves (or all waves when waveIDs is empty).
// Waves are executed sequentially; checks within a wave run in parallel when
// the wave's Parallel flag is true.  Execution stops early when a wave fails
// and the global die-on-error flag is set.
func (r *Runner) RunWaves(waveIDs []string) ([]WaveResult, error) {
	waves := r.Config.Waves
	if len(waveIDs) > 0 {
		waves = make([]config.Wave, 0, len(waveIDs))
		for _, id := range waveIDs {
			wave, ok := r.Config.GetWave(id)
			if !ok {
				return nil, fmt.Errorf("wave %q not found in configuration", id)
			}
			waves = append(waves, *wave)
		}
	}

	inventory := r.Config.InventoryMap()
	allResults := make([]WaveResult, 0, len(waves))

	for _, wave := range waves {
		r.Formatter.WaveStart(wave.ID, wave.Name)
		result, err := r.runWave(wave, inventory)
		if err != nil {
			return allResults, err
		}
		r.Formatter.WaveEnd(wave.ID, result.Success())
		allResults = append(allResults, result)

		if !result.Success() && r.Config.Defaults.DieOnError {
			break
		}
	}

	return allResults, nil
}

func (r *Runner) runWave(wave config.Wave, inventory map[string]config.Check) (WaveResult, error) {
	checks := make([]config.ResolvedCheck, 0, len(wave.Checks))
	for _, checkID := range wave.Checks {
		check, ok := inventory[checkID]
		if !ok {
			return WaveResult{}, fmt.Errorf("check %q not found in inventory", checkID)
		}
		checks = append(checks, r.Config.ResolveCheck(check))
	}

	var results []CheckResult
	if wave.Parallel {
		results = r.runParallel(checks)
	} else {
		results = r.runSequential(checks)
	}

	return WaveResult{Wave: wave, Results: results}, nil
}

func (r *Runner) runParallel(checks []config.ResolvedCheck) []CheckResult {
	results := make([]CheckResult, len(checks))
	var wg sync.WaitGroup

	for i, check := range checks {
		wg.Add(1)
		r.Formatter.CheckStart(check.ID, check.DisplayName())
		go func(idx int, chk config.ResolvedCheck) {
			defer wg.Done()
			res := execCheck(chk)
			r.Formatter.CheckEnd(chk.ID, chk.DisplayName(), res.Success(), res.Output, res.Duration)
			if r.OnCheckDone != nil {
				r.OnCheckDone(chk.ID, chk.DisplayName(), res.Success())
			}
			results[idx] = res
		}(i, check)
	}

	wg.Wait()
	return results
}

func (r *Runner) runSequential(checks []config.ResolvedCheck) []CheckResult {
	results := make([]CheckResult, len(checks))
	for i, check := range checks {
		r.Formatter.CheckStart(check.ID, check.DisplayName())
		res := execCheck(check)
		r.Formatter.CheckEnd(check.ID, check.DisplayName(), res.Success(), res.Output, res.Duration)
		if r.OnCheckDone != nil {
			r.OnCheckDone(check.ID, check.DisplayName(), res.Success())
		}
		results[i] = res
	}
	return results
}

// ResolveChecks returns the deduplicated ordered list of ResolvedChecks for the
// given wave IDs (or all waves when waveIDs is empty).  It does NOT execute
// any commands and is safe to call for dry-run operations like mark-pending.
func (r *Runner) ResolveChecks(waveIDs []string) ([]config.ResolvedCheck, error) {
	waves := r.Config.Waves
	if len(waveIDs) > 0 {
		waves = make([]config.Wave, 0, len(waveIDs))
		for _, id := range waveIDs {
			wave, ok := r.Config.GetWave(id)
			if !ok {
				return nil, fmt.Errorf("wave %q not found in configuration", id)
			}
			waves = append(waves, *wave)
		}
	}

	inventory := r.Config.InventoryMap()
	seen := make(map[string]struct{})
	var resolved []config.ResolvedCheck

	for _, wave := range waves {
		for _, checkID := range wave.Checks {
			if _, already := seen[checkID]; already {
				continue
			}
			seen[checkID] = struct{}{}
			check, ok := inventory[checkID]
			if !ok {
				return nil, fmt.Errorf("check %q not found in inventory", checkID)
			}
			resolved = append(resolved, r.Config.ResolveCheck(check))
		}
	}

	return resolved, nil
}

// execCheck runs a single check command and returns its result.
func execCheck(check config.ResolvedCheck) CheckResult {
	ctx, cancel := context.WithTimeout(context.Background(), check.EffectiveTimeout)
	defer cancel()

	start := time.Now()

	if strings.TrimSpace(check.Command) == "" {
		return CheckResult{
			Check:    check,
			Err:      fmt.Errorf("empty command for check %q", check.ID),
			Duration: time.Since(start),
		}
	}

	// #nosec G204 -- command comes from the repository-owned .insitu.yml config file
	cmd := buildShellCommand(ctx, check.Command)
	// Prevent long hangs when shell-spawned children keep stdio open after timeout.
	cmd.WaitDelay = 200 * time.Millisecond
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if err != nil {
		var exitErr *exec.ExitError
		if ok := isExitError(err, &exitErr); ok {
			exitCode = exitErr.ExitCode()
			err = nil // exit code captured; clear the error
		}
	}

	return CheckResult{
		Check:    check,
		Output:   buf.Bytes(),
		ExitCode: exitCode,
		Duration: duration,
		Err:      err,
	}
}

// buildShellCommand returns a platform-appropriate shell command invocation.
func buildShellCommand(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/C", command)
	}
	return exec.CommandContext(ctx, "sh", "-c", command)
}

// isExitError tries to cast err to *exec.ExitError, returning true on success.
func isExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}
