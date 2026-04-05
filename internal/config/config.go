package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the git-courer configuration
type Config struct {
	Ollama     OllamaConfig     `yaml:"ollama"`
	Git        GitConfig        `yaml:"git"`
	Secrets    SecretsConfig    `yaml:"secrets"`
	Validation ValidationConfig `yaml:"validation"`
	UI         UIConfig         `yaml:"ui"`
	MCP        MCPConfig        `yaml:"mcp"`
}

// OllamaConfig holds Ollama-related settings
type OllamaConfig struct {
	Host      string `yaml:"host"`
	Model     string `yaml:"model"`
	AutoStart bool   `yaml:"auto_start"`
}

// GitConfig holds git-related settings
type GitConfig struct {
	WorkDir          string `yaml:"workdir"`
	AutoAddSecrets   bool   `yaml:"auto_add_secrets"`
	RequireCleanRepo bool   `yaml:"require_clean_repo"`
}

// SecretsConfig holds secrets detection settings
type SecretsConfig struct {
	DetectionMode string   `yaml:"_detection_mode"`
	Patterns      []string `yaml:"patterns"`
}

// ValidationConfig holds validation settings
type ValidationConfig struct {
	RequireConfirmation bool `yaml:"require_confirmation"`
	MaxCommitLength     int  `yaml:"max_commit_length"`
}

// UIConfig holds UI settings
type UIConfig struct {
	Theme     string `yaml:"theme"`
	ShowIcons bool   `yaml:"show_icons"`
}

// MCPConfig holds MCP server settings
type MCPConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// Default returns the default configuration
func Default() *Config {
	return &Config{
		Ollama: OllamaConfig{
			Host:      "http://localhost:11434",
			Model:     "llama3.2",
			AutoStart: false,
		},
		Git: GitConfig{
			WorkDir:          ".",
			AutoAddSecrets:   true,
			RequireCleanRepo: false,
		},
		Secrets: SecretsConfig{
			DetectionMode: "regex+ai",
			Patterns: []string{
				"*.key",
				"*.pem",
				".env*",
				"credentials.json",
				"secrets.yaml",
				"*.password",
				"*.token",
			},
		},
		Validation: ValidationConfig{
			RequireConfirmation: true,
			MaxCommitLength:     72,
		},
		UI: UIConfig{
			Theme:     "dark",
			ShowIcons: true,
		},
		MCP: MCPConfig{
			Name:    "git-courer",
			Version: "1.0.0",
		},
	}
}

// Load loads configuration from file
// It looks for config in this order:
// 1. Current directory ./git-courer.yaml
// 2. Home directory ~/.config/git-courer.yaml
// If no config found, returns default
func Load() (*Config, error) {
	cfg := Default()

	// Search paths
	searchPaths := []string{
		"git-courer.yaml",
		filepath.Join(os.Getenv("HOME"), ".config", "git-courer.yaml"),
	}

	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read config: %w", err)
			}

			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse config: %w", err)
			}

			return cfg, nil
		}
	}

	return cfg, nil
}
