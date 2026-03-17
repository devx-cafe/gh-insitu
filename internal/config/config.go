// Package config provides YAML configuration parsing and validation for insitu.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DefaultConfigFile is the default configuration file name.
const DefaultConfigFile = ".insitu.yml"

// DefaultTimeout is the fallback timeout when none is specified.
const DefaultTimeout = 5 * time.Minute

// Config is the top-level insitu configuration.
type Config struct {
	Defaults  Defaults `yaml:"defaults"`
	Inventory []Check  `yaml:"inventory"`
	Waves     []Wave   `yaml:"waves"`
}

// Defaults holds global default values inherited by all checks.
type Defaults struct {
	DieOnError bool   `yaml:"die-on-error"`
	Timeout    string `yaml:"timeout"`
	Verbose    bool   `yaml:"verbose"`
}

// Check defines a single named check in the inventory.
type Check struct {
	ID         string  `yaml:"id"`
	Name       string  `yaml:"name"`
	Command    string  `yaml:"command"`
	Timeout    *string `yaml:"timeout"`
	DieOnError *bool   `yaml:"die-on-error"`
}

// Wave defines an execution wave referencing checks by ID.
type Wave struct {
	ID       string   `yaml:"id"`
	Name     string   `yaml:"name"`
	Parallel bool     `yaml:"parallel"`
	Checks   []string `yaml:"checks"`
}

// ResolvedCheck is a Check with all inherited defaults applied.
type ResolvedCheck struct {
	Check
	EffectiveTimeout    time.Duration
	EffectiveDieOnError bool
}

// Load reads and parses the configuration file at the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is caller-supplied config file
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %q: %w", path, err)
	}
	return Parse(data)
}

// Parse unmarshals YAML data into a Config and validates it.
func Parse(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks the configuration for consistency errors.
func (c *Config) Validate() error {
	inventory := make(map[string]struct{}, len(c.Inventory))
	for _, check := range c.Inventory {
		if check.ID == "" {
			return fmt.Errorf("check missing required 'id' field")
		}
		if check.Command == "" {
			return fmt.Errorf("check %q missing required 'command' field", check.ID)
		}
		if check.Timeout != nil {
			if _, err := time.ParseDuration(*check.Timeout); err != nil {
				return fmt.Errorf("check %q has invalid timeout %q: %w", check.ID, *check.Timeout, err)
			}
		}
		inventory[check.ID] = struct{}{}
	}

	if c.Defaults.Timeout != "" {
		if _, err := time.ParseDuration(c.Defaults.Timeout); err != nil {
			return fmt.Errorf("defaults.timeout is invalid %q: %w", c.Defaults.Timeout, err)
		}
	}

	for _, wave := range c.Waves {
		if wave.ID == "" {
			return fmt.Errorf("wave missing required 'id' field")
		}
		for _, checkID := range wave.Checks {
			if _, ok := inventory[checkID]; !ok {
				return fmt.Errorf("wave %q references unknown check id %q", wave.ID, checkID)
			}
		}
	}

	return nil
}

// InventoryMap returns the inventory as a map for quick lookup by ID.
func (c *Config) InventoryMap() map[string]Check {
	m := make(map[string]Check, len(c.Inventory))
	for _, check := range c.Inventory {
		m[check.ID] = check
	}
	return m
}

// ResolveCheck applies defaults to produce a ResolvedCheck.
func (c *Config) ResolveCheck(check Check) ResolvedCheck {
	resolved := ResolvedCheck{Check: check}

	// Effective timeout: check override → defaults → built-in default
	if check.Timeout != nil {
		if d, err := time.ParseDuration(*check.Timeout); err == nil {
			resolved.EffectiveTimeout = d
		}
	} else if c.Defaults.Timeout != "" {
		if d, err := time.ParseDuration(c.Defaults.Timeout); err == nil {
			resolved.EffectiveTimeout = d
		}
	}
	if resolved.EffectiveTimeout == 0 {
		resolved.EffectiveTimeout = DefaultTimeout
	}

	// Effective die-on-error: check override → defaults
	if check.DieOnError != nil {
		resolved.EffectiveDieOnError = *check.DieOnError
	} else {
		resolved.EffectiveDieOnError = c.Defaults.DieOnError
	}

	return resolved
}

// DisplayName returns the human-readable name for a check.
func (c Check) DisplayName() string {
	if c.Name != "" {
		return c.Name
	}
	if c.ID == "" {
		return ""
	}
	return strings.ToUpper(c.ID[:1]) + c.ID[1:]
}

// GetWave returns the wave with the given ID, or nil if not found.
func (c *Config) GetWave(id string) (*Wave, bool) {
	for i := range c.Waves {
		if c.Waves[i].ID == id {
			return &c.Waves[i], true
		}
	}
	return nil, false
}
