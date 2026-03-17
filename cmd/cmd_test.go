package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/devx-cafe/gh-insitu/internal/config"
	"github.com/devx-cafe/gh-insitu/internal/runner"
	"github.com/devx-cafe/gh-insitu/internal/ui"
)

// ─── printPlan ───────────────────────────────────────────────────────────────

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

	// Redirect stdout temporarily
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printPlan(cfg)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
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

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printPlan(cfg)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "no waves defined") {
		t.Errorf("output %q missing 'no waves defined'", out)
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

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printPlan(cfg)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
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

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printPlan(cfg)

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "die-on-error: false") {
		t.Errorf("output %q missing 'die-on-error: false'", out)
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

	err := runMarkPending(r, nil)
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

	err := runMarkPending(r, nil)
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
