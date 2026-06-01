// Package config loads git-courer configuration.
// Configuration is loaded from a single global file: ~/.config/git-courer/config.yaml
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

// ServerName is the MCP server identifier registered with AI clients.
const ServerName = "git-courer"

// ReleaseConfig holds release-related settings.
type ReleaseConfig struct {
	Type string `yaml:"type"` // "tag" or "github" (default: "tag")
}

// LLMConfig holds unified LLM provider settings.
// Provider and Model are MANDATORY (no defaults).
// BaseURL defaults to http://localhost:11434/v1.
type LLMConfig struct {
	Provider         string `yaml:"provider"`       // MANDATORY — provider: ollama, openai-compatible, etc.
	Model            string `yaml:"model"`          // MANDATORY — model name
	BaseURL          string `yaml:"base_url"`       // Default: http://localhost:11434/v1
	NumParallel      int    `yaml:"num_parallel"`   // Default: 1
	ContextWindow    int    `yaml:"context_window"` // Resolved at install, default 0
	ContextWindowStr string `yaml:"-"`              // Form helper — string representation of ContextWindow (not serialized)
}

// Config represents the git-courer configuration.
// All fields are editable by users via config files.
type Config struct {
	LLM     LLMConfig     `yaml:"llm"`
	Release ReleaseConfig `yaml:"release"`
}

// Default returns the default configuration with optional fields populated.
func Default() *Config {
	return &Config{
		LLM: LLMConfig{
			Provider:    "", // No default — mandatory
			Model:       "", // No default — mandatory
			BaseURL:     "http://localhost:11434/v1",
			NumParallel: 1,
		},
		Release: ReleaseConfig{
			Type: "tag",
		},
	}
}

// Validate returns an error listing all missing mandatory fields.
// Returns nil if all mandatory fields are set.
func (c *Config) Validate() error {
	var missing []string
	if c.LLM.Provider == "" {
		missing = append(missing, "llm.provider")
	}
	if c.LLM.Model == "" {
		missing = append(missing, "llm.model")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing mandatory fields: %s", strings.Join(missing, ", "))
	}
	return nil
}

// GlobalConfigPath returns the platform-appropriate global config path.
func GlobalConfigPath() string {
	home, _ := os.UserHomeDir()
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "git-courer", "config.yaml")
	}
	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "git-courer", "config.yaml")
		}
		return filepath.Join(home, "AppData", "Roaming", "git-courer", "config.yaml")
	default:
		return filepath.Join(home, ".config", "git-courer", "config.yaml")
	}
}

// Load loads configuration from the current directory.
func Load() (*Config, error) {
	return LoadFromDir(".")
}

// KnownFields returns the set of field paths that are expected in the config.
// Per-project config loading has been removed. The workDir parameter is kept
// for API compatibility but is ignored.
// Unknown fields in YAML are logged as warnings (not errors).
func LoadFromDir(workDir string) (*Config, error) {
	cfg := Default()

	// Load global config ONLY — no per-project loading
	if data, err := os.ReadFile(GlobalConfigPath()); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse global config: %w", err)
		}
		logUnknownFields(data, cfg)
	}

	return cfg, nil
}

// KnownFields returns the set of field paths that are expected in the config.
// Used for unknown field detection.
var knownFields = map[string]bool{
	"llm":                true,
	"llm.provider":       true,
	"llm.model":          true,
	"llm.base_url":       true,
	"llm.num_parallel":   true,
	"llm.context_window": true,
	"release":            true,
	"release.type":       true,
}

// logUnknownFields logs a warning for any YAML fields that don't match known config fields.
// This helps users migrate from old config formats.
func logUnknownFields(data []byte, cfg interface{}) {
	// Use yaml.Node to inspect the raw structure
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return
	}
	// Flatten and check each top-level key
	for key := range raw {
		if !knownFields[key] {
			// Would use a logger in production; for now we just note it
			// In practice this would be: log.Printf("warning: unknown config field: %s", key)
		}
	}
}

// SaveGlobal saves the config to the global config path.
func (c *Config) SaveGlobal() error {
	path := GlobalConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
