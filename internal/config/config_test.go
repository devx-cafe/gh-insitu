package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/devx-cafe/gh-insitu/internal/config"
)

func TestParse_ValidConfig(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "valid.yml"))
	if err != nil {
		t.Fatalf("failed to read testdata: %v", err)
	}
	cfg, err := config.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}

	if len(cfg.Inventory) != 3 {
		t.Errorf("inventory len = %d, want 3", len(cfg.Inventory))
	}
	if len(cfg.Waves) != 2 {
		t.Errorf("waves len = %d, want 2", len(cfg.Waves))
	}
	if !cfg.Defaults.DieOnError {
		t.Error("defaults.die-on-error = false, want true")
	}
	if cfg.Defaults.Timeout != "5m" {
		t.Errorf("defaults.timeout = %q, want %q", cfg.Defaults.Timeout, "5m")
	}
}

func TestParse_MinimalConfig(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "minimal.yml"))
	if err != nil {
		t.Fatalf("failed to read testdata: %v", err)
	}
	cfg, err := config.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
	if len(cfg.Inventory) != 1 {
		t.Errorf("inventory len = %d, want 1", len(cfg.Inventory))
	}
	if len(cfg.Waves) != 1 {
		t.Errorf("waves len = %d, want 1", len(cfg.Waves))
	}
}

func TestParse_InvalidUnknownCheck(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "invalid_unknown_check.yml"))
	if err != nil {
		t.Fatalf("failed to read testdata: %v", err)
	}
	_, err = config.Parse(data)
	if err == nil {
		t.Fatal("Parse() error = nil, want error for unknown check reference")
	}
	if !strings.Contains(err.Error(), "unknown check id") {
		t.Errorf("error %q does not mention 'unknown check id'", err.Error())
	}
}

func TestParse_InvalidMissingCommand(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "invalid_missing_command.yml"))
	if err != nil {
		t.Fatalf("failed to read testdata: %v", err)
	}
	_, err = config.Parse(data)
	if err == nil {
		t.Fatal("Parse() error = nil, want error for missing command")
	}
	if !strings.Contains(err.Error(), "missing required 'command'") {
		t.Errorf("error %q does not mention missing command", err.Error())
	}
}

func TestParse_InvalidYAML(t *testing.T) {
	_, err := config.Parse([]byte(":\nbad yaml: [\n"))
	if err == nil {
		t.Fatal("Parse() error = nil, want error for invalid YAML")
	}
}

func TestParse_InvalidTimeout(t *testing.T) {
	yml := `
defaults:
  timeout: "notaduration"
inventory:
  - id: "x"
    command: "echo x"
waves: []
`
	_, err := config.Parse([]byte(yml))
	if err == nil {
		t.Fatal("Parse() error = nil, want error for invalid timeout")
	}
}

func TestLoad_FileNotFound(t *testing.T) {
	_, err := config.Load("/nonexistent/path/.insitu.yml")
	if err == nil {
		t.Fatal("Load() error = nil, want error for missing file")
	}
}

func TestLoad_ValidFile(t *testing.T) {
	cfg, err := config.Load(filepath.Join("testdata", "valid.yml"))
	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}
}

func TestConfig_ResolveCheck(t *testing.T) {
	tests := []struct {
		name            string
		defaults        config.Defaults
		check           config.Check
		wantTimeout     time.Duration
		wantDieOnError  bool
	}{
		{
			name:           "check uses defaults timeout",
			defaults:       config.Defaults{Timeout: "5m", DieOnError: true},
			check:          config.Check{ID: "x", Command: "echo x"},
			wantTimeout:    5 * time.Minute,
			wantDieOnError: true,
		},
		{
			name:     "check overrides timeout",
			defaults: config.Defaults{Timeout: "5m", DieOnError: true},
			check: config.Check{
				ID:      "x",
				Command: "echo x",
				Timeout: strPtr("10m"),
			},
			wantTimeout:    10 * time.Minute,
			wantDieOnError: true,
		},
		{
			name:     "check overrides die-on-error",
			defaults: config.Defaults{DieOnError: true},
			check: config.Check{
				ID:         "x",
				Command:    "echo x",
				DieOnError: boolPtr(false),
			},
			wantTimeout:    config.DefaultTimeout,
			wantDieOnError: false,
		},
		{
			name:           "no defaults - uses built-in default timeout",
			defaults:       config.Defaults{},
			check:          config.Check{ID: "x", Command: "echo x"},
			wantTimeout:    config.DefaultTimeout,
			wantDieOnError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Defaults:  tt.defaults,
				Inventory: []config.Check{tt.check},
			}
			resolved := cfg.ResolveCheck(tt.check)
			if resolved.EffectiveTimeout != tt.wantTimeout {
				t.Errorf("EffectiveTimeout = %v, want %v", resolved.EffectiveTimeout, tt.wantTimeout)
			}
			if resolved.EffectiveDieOnError != tt.wantDieOnError {
				t.Errorf("EffectiveDieOnError = %v, want %v", resolved.EffectiveDieOnError, tt.wantDieOnError)
			}
		})
	}
}

func TestCheck_DisplayName(t *testing.T) {
	tests := []struct {
		name  string
		check config.Check
		want  string
	}{
		{
			name:  "uses Name field when set",
			check: config.Check{ID: "build", Name: "Build Binary"},
			want:  "Build Binary",
		},
		{
			name:  "capitalizes ID when Name is empty",
			check: config.Check{ID: "coverage"},
			want:  "Coverage",
		},
		{
			name:  "empty check",
			check: config.Check{},
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.check.DisplayName()
			if got != tt.want {
				t.Errorf("DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfig_GetWave(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "valid.yml"))
	if err != nil {
		t.Fatalf("failed to read testdata: %v", err)
	}
	cfg, err := config.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	wave, ok := cfg.GetWave("static")
	if !ok {
		t.Fatal("GetWave('static') not found")
	}
	if wave.ID != "static" {
		t.Errorf("wave.ID = %q, want %q", wave.ID, "static")
	}

	_, ok = cfg.GetWave("nonexistent")
	if ok {
		t.Error("GetWave('nonexistent') returned true, want false")
	}
}

func TestConfig_InventoryMap(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "valid.yml"))
	if err != nil {
		t.Fatalf("failed to read testdata: %v", err)
	}
	cfg, err := config.Parse(data)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	m := cfg.InventoryMap()
	if len(m) != 3 {
		t.Errorf("InventoryMap() len = %d, want 3", len(m))
	}
	if _, ok := m["build"]; !ok {
		t.Error("InventoryMap() missing 'build' key")
	}
}

// helper functions
func strPtr(s string) *string  { return &s }
func boolPtr(b bool) *bool     { return &b }
