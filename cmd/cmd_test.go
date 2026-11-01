package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devx-cafe/gh-insitu/internal/config"
	"github.com/devx-cafe/gh-insitu/internal/runner"
	"github.com/devx-cafe/gh-insitu/internal/ui"
	"github.com/spf13/cobra"
)

// ─── plan output ────────────────────────────────────────────────────────────

func TestPrintPlan_PrintsAllWaves(t *testing.T) {
	cfg := &config.Config{
		Inventory: []config.Check{
			{ID: "build", Name: "Build", Command: "make build"},
			{ID: "lint", Name: "Lint", Command: "make lint"},
		},
		Waves: []config.Wave{
			{ID: "static", Name: "Static Analysis", Parallel: true, Checks: []string{"build", "lint"}},
		},
	}

	var buf bytes.Buffer
	printWavePlan(&buf, cfg, cfg.Waves)
	out := buf.String()

	if !strings.Contains(out, "Execution Plan") {
		t.Errorf("output %q missing 'Execution Plan'", out)
	}
	if !strings.Contains(out, "Static Analysis") {
		t.Errorf("output %q missing wave name", out)
	}
	if !strings.Contains(out, "parallel") {
		t.Errorf("output %q missing 'parallel'", out)
	}
	if !strings.Contains(out, "make build") {
		t.Errorf("output %q missing command", out)
	}
}

func TestPrintPlan_EmptyWaves(t *testing.T) {
	cfg := &config.Config{}

	var buf bytes.Buffer
	printWavePlan(&buf, cfg, nil)
	out := buf.String()

	if strings.Contains(out, "Wave ") {
		t.Errorf("empty selection printed a wave: %q", out)
	}
}

func TestPrintPlan_SequentialWave(t *testing.T) {
	cfg := &config.Config{
		Inventory: []config.Check{
			{ID: "test", Command: "go test"},
		},
		Waves: []config.Wave{
			{ID: "test-wave", Parallel: false, Checks: []string{"test"}},
		},
	}

	var buf bytes.Buffer
	printWavePlan(&buf, cfg, cfg.Waves)
	out := buf.String()

	if !strings.Contains(out, "sequential") {
		t.Errorf("output %q missing 'sequential'", out)
	}
}

func TestPrintPlan_DieOnErrorFalse(t *testing.T) {
	falseVal := false
	cfg := &config.Config{
		Defaults: config.Defaults{DieOnError: true},
		Inventory: []config.Check{
			{ID: "x", Command: "echo x", DieOnError: &falseVal},
		},
		Waves: []config.Wave{
			{ID: "w", Parallel: false, Checks: []string{"x"}},
		},
	}

	var buf bytes.Buffer
	printWavePlan(&buf, cfg, cfg.Waves)
	out := buf.String()

	if !strings.Contains(out, "die-on-error: false") {
		t.Errorf("output %q missing 'die-on-error: false'", out)
	}
}

func TestSelectWaves_PreservesSelectionOrder(t *testing.T) {
	cfg := &config.Config{Waves: []config.Wave{{ID: "first"}, {ID: "second"}}}
	waves, err := selectWaves(cfg, []string{"second", "first"})
	if err != nil {
		t.Fatalf("selectWaves() error = %v", err)
	}
	if len(waves) != 2 || waves[0].ID != "second" || waves[1].ID != "first" {
		t.Errorf("selected waves = %#v, want [second first]", waves)
	}
}

func TestSelectWaves_UnknownID(t *testing.T) {
	cfg := &config.Config{Waves: []config.Wave{{ID: "first"}}}
	if _, err := selectWaves(cfg, []string{"first", "missing"}); err == nil {
		t.Fatal("selectWaves() error = nil, want error for unknown wave")
	}
}

func TestPrintTaskPlan_IncludesEffectiveSettings(t *testing.T) {
	check := config.ResolvedCheck{
		Check:               config.Check{ID: "task", Name: "Task", Command: "echo task"},
		EffectiveTimeout:    30 * time.Second,
		EffectiveDieOnError: false,
	}
	var buf bytes.Buffer
	printTaskPlan(&buf, check)
	for _, expected := range []string{"Task", "30s", "echo task", "die-on-error: false"} {
		if !strings.Contains(buf.String(), expected) {
			t.Errorf("task plan %q missing %q", buf.String(), expected)
		}
	}
}

func TestResolveTaskOverrides(t *testing.T) {
	inventoryTimeout := "20s"
	inventoryDieOnError := false
	cfg := &config.Config{
		Defaults: config.Defaults{Timeout: "10s", DieOnError: true},
		Inventory: []config.Check{{
			ID: "task", Command: "echo task",
			Timeout: &inventoryTimeout, DieOnError: &inventoryDieOnError,
		}},
	}

	tests := []struct {
		name           string
		timeout        string
		timeoutSet     bool
		dieOnError     bool
		dieOnErrorSet  bool
		wantTimeout    time.Duration
		wantDieOnError bool
		wantErr        bool
	}{
		{name: "configured values", wantTimeout: 20 * time.Second, wantDieOnError: false},
		{name: "timeout override", timeout: "30s", timeoutSet: true, wantTimeout: 30 * time.Second},
		{name: "enable override", dieOnError: true, dieOnErrorSet: true, wantTimeout: 20 * time.Second, wantDieOnError: true},
		{name: "disable override", dieOnErrorSet: true, wantTimeout: 20 * time.Second, wantDieOnError: false},
		{name: "invalid timeout", timeout: "bad", timeoutSet: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveTask(cfg, "task", tt.timeout, tt.timeoutSet, tt.dieOnError, tt.dieOnErrorSet)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveTask() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && (got.EffectiveTimeout != tt.wantTimeout || got.EffectiveDieOnError != tt.wantDieOnError) {
				t.Errorf("resolved task = (%s, %t), want (%s, %t)",
					got.EffectiveTimeout, got.EffectiveDieOnError, tt.wantTimeout, tt.wantDieOnError)
			}
		})
	}

	if _, err := resolveTask(cfg, "missing", "", false, false, false); err == nil {
		t.Fatal("resolveTask() error = nil, want unknown inventory ID error")
	}
}

func TestCommandArgumentCardinality(t *testing.T) {
	tests := []struct {
		name    string
		cmd     *cobra.Command
		args    []string
		wantErr bool
	}{
		{name: "run requires wave", cmd: runCmd, args: nil, wantErr: true},
		{name: "run accepts wave", cmd: runCmd, args: []string{"wave-a"}},
		{name: "task requires inventory ID", cmd: taskCmd, args: nil, wantErr: true},
		{name: "task accepts one inventory ID", cmd: taskCmd, args: []string{"task-a"}},
		{name: "task rejects multiple inventory IDs", cmd: taskCmd, args: []string{"task-a", "task-b"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Args(tt.cmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("Args(%v) error = %v, wantErr %v", tt.args, err, tt.wantErr)
			}
		})
	}
}

func TestCommandRegistrationAndFlags(t *testing.T) {
	for _, command := range rootCmd.Commands() {
		if command.Name() == "plan" {
			t.Fatal("standalone plan command is still registered")
		}
	}
	for _, name := range []string{"plan", "verbose"} {
		if runCmd.Flags().Lookup(name) == nil {
			t.Errorf("run command is missing --%s", name)
		}
		if taskCmd.Flags().Lookup(name) == nil {
			t.Errorf("task command is missing --%s", name)
		}
	}
	for _, name := range []string{"timeout", "die-on-error"} {
		if runCmd.Flags().Lookup(name) != nil {
			t.Errorf("run command unexpectedly accepts --%s", name)
		}
		if taskCmd.Flags().Lookup(name) == nil {
			t.Errorf("task command is missing --%s", name)
		}
	}
	if runCmd.Flags().ShorthandLookup("p") == nil || taskCmd.Flags().ShorthandLookup("p") == nil {
		t.Error("both run and task must provide -p as a shorthand for --plan")
	}
}

// ─── waveName ────────────────────────────────────────────────────────────────

func TestWaveName_UsesNameField(t *testing.T) {
	w := config.Wave{ID: "static", Name: "Static Analysis"}
	if got := waveName(w); got != "Static Analysis" {
		t.Errorf("waveName() = %q, want %q", got, "Static Analysis")
	}
}

func TestWaveName_FallsBackToID(t *testing.T) {
	w := config.Wave{ID: "static"}
	if got := waveName(w); got != "static" {
		t.Errorf("waveName() = %q, want %q", got, "static")
	}
}

// ─── buildStatusReporter ─────────────────────────────────────────────────────

func TestBuildStatusReporter_ReturnsCallback(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "")
	t.Setenv("GITHUB_SHA", "")
	t.Setenv("GH_TOKEN", "")

	fn := buildStatusReporter()
	if fn == nil {
		t.Fatal("buildStatusReporter() returned nil")
	}
	// Calling with missing env vars should be a no-op (not panic)
	fn("build", "Build", true)
}

func TestBuildStatusReporter_NoopWhenMissingEnv(t *testing.T) {
	t.Setenv("GITHUB_REPOSITORY", "")
	t.Setenv("GITHUB_SHA", "abc123")
	t.Setenv("GH_TOKEN", "token")

	fn := buildStatusReporter()
	// Should not panic – silently skipped when repo is missing
	fn("build", "Build", false)
}

// ─── runMarkPending ──────────────────────────────────────────────────────────

func TestRunMarkPending_FailsOutsideCI(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "")

	cfg := &config.Config{
		Inventory: []config.Check{{ID: "x", Command: "echo x"}},
		Waves:     []config.Wave{{ID: "w", Checks: []string{"x"}}},
	}
	f := ui.NewFormatter(os.Stdout, false)
	r := runner.New(cfg, f)

	err := runMarkPending(r, []string{"w"})
	if err == nil {
		t.Fatal("runMarkPending() error = nil, want error outside CI")
	}
	if !strings.Contains(err.Error(), "GitHub Actions") {
		t.Errorf("error %q missing 'GitHub Actions'", err.Error())
	}
}

func TestRunMarkPending_FailsWithoutToken(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	t.Setenv("GITHUB_SHA", "abc123")
	t.Setenv("GH_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")

	cfg := &config.Config{
		Inventory: []config.Check{{ID: "x", Command: "echo x"}},
		Waves:     []config.Wave{{ID: "w", Checks: []string{"x"}}},
	}
	f := ui.NewFormatter(os.Stdout, false)
	r := runner.New(cfg, f)

	err := runMarkPending(r, []string{"w"})
	if err == nil {
		t.Fatal("runMarkPending() error = nil, want error without token")
	}
}

func TestRunMarkPending_UnknownWave(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_REPOSITORY", "owner/repo")
	t.Setenv("GITHUB_SHA", "abc123")
	t.Setenv("GH_TOKEN", "token")

	cfg := &config.Config{
		Inventory: []config.Check{{ID: "x", Command: "echo x"}},
		Waves:     []config.Wave{{ID: "w", Checks: []string{"x"}}},
	}

	f := ui.NewFormatter(os.Stdout, false)
	r := runner.New(cfg, f)

	err := runMarkPending(r, []string{"no-such-wave"})
	if err == nil {
		t.Fatal("runMarkPending() error = nil, want error for unknown wave")
	}
}

func TestRunMarkPending_RequiresWaveSelection(t *testing.T) {
	if err := runMarkPending(nil, nil); err == nil || !strings.Contains(err.Error(), "wave ID") {
		t.Fatalf("runMarkPending(nil) error = %v, want required wave ID error", err)
	}
}

// ─── writeConfig ─────────────────────────────────────────────────────────────

func TestWriteConfig_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".insitu.yml")

	if err := writeConfig(path, false); err != nil {
		t.Fatalf("writeConfig() error = %v", err)
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("writeConfig() did not create the file")
	}
}

func TestWriteConfig_SkipsExistingWithoutForce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".insitu.yml")

	// Write something different first
	original := []byte("# original")
	if err := os.WriteFile(path, original, 0o644); err != nil { //nolint:gosec
		t.Fatalf("setup: %v", err)
	}

	if err := writeConfig(path, false); err != nil {
		t.Fatalf("writeConfig() error = %v", err)
	}

	// File should be unchanged
	data, _ := os.ReadFile(path) //nolint:gosec
	if string(data) != string(original) {
		t.Errorf("writeConfig() overwrote existing file without --force")
	}
}

func TestWriteConfig_OverwritesWithForce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".insitu.yml")

	if err := os.WriteFile(path, []byte("# old"), 0o644); err != nil { //nolint:gosec
		t.Fatalf("setup: %v", err)
	}

	if err := writeConfig(path, true); err != nil {
		t.Fatalf("writeConfig(force=true) error = %v", err)
	}

	data, _ := os.ReadFile(path) //nolint:gosec
	if !strings.Contains(string(data), "insitu") {
		t.Error("writeConfig(force=true) did not write default content")
	}
}
