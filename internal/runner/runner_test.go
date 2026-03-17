package runner_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/devx-cafe/gh-insitu/internal/config"
	"github.com/devx-cafe/gh-insitu/internal/runner"
	"github.com/devx-cafe/gh-insitu/internal/ui"
)

// loadRunnerConfig loads the shared runner test configuration.
func loadRunnerConfig(t *testing.T) *config.Config {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "config", "testdata", "runner_test.yml"))
	if err != nil {
		t.Fatalf("failed to read runner test config: %v", err)
	}
	cfg, err := config.Parse(data)
	if err != nil {
		t.Fatalf("failed to parse runner test config: %v", err)
	}
	return cfg
}

// discardFormatter is a no-op Formatter used to silence output in tests.
type discardFormatter struct{}

func (discardFormatter) WaveStart(_, _ string)                                      {}
func (discardFormatter) WaveEnd(_ string, _ bool)                                   {}
func (discardFormatter) CheckStart(_, _ string)                                     {}
func (discardFormatter) CheckEnd(_, _ string, _ bool, _ []byte, _ time.Duration)    {}
func (discardFormatter) PrintSummary(_ bool)                                        {}

var _ ui.Formatter = discardFormatter{}

func TestRunner_ParallelWave_AllPass(t *testing.T) {
	cfg := loadRunnerConfig(t)
	r := runner.New(cfg, discardFormatter{})

	results, err := r.RunWaves([]string{"parallel-wave"})
	if err != nil {
		t.Fatalf("RunWaves() error = %v, want nil", err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if !results[0].Success() {
		t.Error("parallel-wave should pass, got failure")
	}
	if len(results[0].Results) != 2 {
		t.Errorf("len(results[0].Results) = %d, want 2", len(results[0].Results))
	}
	for _, cr := range results[0].Results {
		if !cr.Success() {
			t.Errorf("check %q failed unexpectedly", cr.Check.ID)
		}
	}
}

func TestRunner_SequentialWave_AllPass(t *testing.T) {
	cfg := loadRunnerConfig(t)
	r := runner.New(cfg, discardFormatter{})

	results, err := r.RunWaves([]string{"sequential-wave"})
	if err != nil {
		t.Fatalf("RunWaves() error = %v, want nil", err)
	}
	if !results[0].Success() {
		t.Error("sequential-wave should pass, got failure")
	}
}

func TestRunner_FailingWave(t *testing.T) {
	cfg := loadRunnerConfig(t)
	r := runner.New(cfg, discardFormatter{})

	results, err := r.RunWaves([]string{"failing-wave"})
	if err != nil {
		t.Fatalf("RunWaves() error = %v, want nil", err)
	}
	if results[0].Success() {
		t.Error("failing-wave should fail, got success")
	}
}

func TestRunner_DieOnError_StopsAfterFailingWave(t *testing.T) {
	cfg := loadRunnerConfig(t)
	cfg.Defaults.DieOnError = true
	r := runner.New(cfg, discardFormatter{})

	// Run failing-wave first, then mixed-wave. With die-on-error=true the second
	// wave must not execute.
	results, err := r.RunWaves([]string{"failing-wave", "mixed-wave"})
	if err != nil {
		t.Fatalf("RunWaves() error = %v, want nil", err)
	}
	if len(results) != 1 {
		t.Errorf("len(results) = %d, want 1 (stopped after first failing wave)", len(results))
	}
}

func TestRunner_DieOnError_False_ContinuesAfterFailure(t *testing.T) {
	cfg := loadRunnerConfig(t)
	cfg.Defaults.DieOnError = false
	r := runner.New(cfg, discardFormatter{})

	results, err := r.RunWaves([]string{"failing-wave", "sequential-wave"})
	if err != nil {
		t.Fatalf("RunWaves() error = %v, want nil", err)
	}
	if len(results) != 2 {
		t.Errorf("len(results) = %d, want 2 (continued despite failure)", len(results))
	}
}

func TestRunner_AllWaves(t *testing.T) {
	cfg := loadRunnerConfig(t)
	cfg.Defaults.DieOnError = false
	r := runner.New(cfg, discardFormatter{})

	results, err := r.RunWaves(nil)
	if err != nil {
		t.Fatalf("RunWaves(nil) error = %v, want nil", err)
	}
	if len(results) != len(cfg.Waves) {
		t.Errorf("len(results) = %d, want %d", len(results), len(cfg.Waves))
	}
}

func TestRunner_UnknownWaveID(t *testing.T) {
	cfg := loadRunnerConfig(t)
	r := runner.New(cfg, discardFormatter{})

	_, err := r.RunWaves([]string{"no-such-wave"})
	if err == nil {
		t.Fatal("RunWaves() error = nil, want error for unknown wave id")
	}
}

func TestRunner_CheckResultOutput(t *testing.T) {
	// Run a check that produces known output and verify we capture it.
	yml := `
defaults:
  die-on-error: false
  timeout: 10s
inventory:
  - id: "greet"
    name: "Greeting"
    command: "echo hello-world"
waves:
  - id: "greet-wave"
    parallel: false
    checks:
      - "greet"
`
	cfg, err := config.Parse([]byte(yml))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	r := runner.New(cfg, discardFormatter{})
	results, err := r.RunWaves(nil)
	if err != nil {
		t.Fatalf("RunWaves() error = %v", err)
	}
	if len(results[0].Results) == 0 {
		t.Fatal("no check results")
	}
	cr := results[0].Results[0]
	if !cr.Success() {
		t.Errorf("check failed unexpectedly: %v", cr.Err)
	}
	if string(cr.Output) == "" {
		t.Error("expected non-empty output from echo command")
	}
}

func TestRunner_Timeout(t *testing.T) {
	yml := `
defaults:
  die-on-error: false
inventory:
  - id: "slow"
    name: "Slow"
    command: "sleep 60"
    timeout: "50ms"
waves:
  - id: "slow-wave"
    parallel: false
    checks:
      - "slow"
`
	cfg, err := config.Parse([]byte(yml))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	r := runner.New(cfg, discardFormatter{})
	start := time.Now()
	results, err := r.RunWaves(nil)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("RunWaves() error = %v", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("runner took %v, expected timeout < 5s", elapsed)
	}
	if results[0].Success() {
		t.Error("slow check should have failed due to timeout, got success")
	}
}

func TestRunner_NewFormatterLocal(t *testing.T) {
	// Verify NewFormatter returns a non-nil formatter when not in CI
	t.Setenv("GITHUB_ACTIONS", "")
	f := ui.NewFormatter(io.Discard)
	if f == nil {
		t.Fatal("NewFormatter() returned nil")
	}
}

func TestRunner_NewFormatterCI(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	f := ui.NewFormatter(io.Discard)
	if f == nil {
		t.Fatal("NewFormatter() returned nil in CI mode")
	}
}
