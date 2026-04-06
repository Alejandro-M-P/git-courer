package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

// Config represents the git-courer configuration.
// Fields are merged: global defaults → global file → project file.
// Project settings override global settings.
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
	Host          string `yaml:"host"`
	Model         string `yaml:"model"`
	ContextWindow int    `yaml:"context_window"` // Max tokens for context (0 = default)
	AutoStart     bool   `yaml:"auto_start"`
}

// GitConfig holds git-related settings
type GitConfig struct {
	WorkDir          string `yaml:"workdir"`
	AutoAddSecrets   bool   `yaml:"auto_add_secrets"`
	RequireCleanRepo bool   `yaml:"require_clean_repo"`
}

// SecretsConfig holds secrets detection settings
type SecretsConfig struct {
	DetectionMode string   `yaml:"detection_mode"`
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

// Default returns the default configuration (base defaults, no files)
func Default() *Config {
	return &Config{
		Ollama: OllamaConfig{
			Host:          "http://localhost:11434",
			Model:         "qwen3.5:0.8b",
			ContextWindow: 0, // 0 = use Ollama default
			AutoStart:     false,
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

// GlobalConfigPath returns the path to the global config file.
// Linux:   ~/.config/git-courer/config.yaml
// macOS:   ~/.config/git-courer/config.yaml (or ~/Library/Application Support/git-courer/config.yaml)
// Windows: %APPDATA%/git-courer/config.yaml
func GlobalConfigPath() string {
	home, _ := os.UserHomeDir()

	// Check XDG_CONFIG_HOME first
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "git-courer", "config.yaml")
	}

	// Platform-specific defaults
	switch detectOS() {
	case "darwin":
		// macOS: prefer ~/.config (consistent with other CLI tools)
		return filepath.Join(home, ".config", "git-courer", "config.yaml")
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "git-courer", "config.yaml")
		}
		return filepath.Join(home, "AppData", "Roaming", "git-courer", "config.yaml")
	default:
		// Linux
		return filepath.Join(home, ".config", "git-courer", "config.yaml")
	}
}

// ProjectConfigPaths returns the list of possible project config paths,
// in order of priority (first match wins).
func ProjectConfigPaths(workDir string) []string {
	return []string{
		filepath.Join(workDir, "git-courer.yaml"),
		filepath.Join(workDir, ".git-courer.yaml"),
		filepath.Join(workDir, ".git-courer", "config.yaml"),
	}
}

// Load loads configuration with merge: defaults → global → project.
// Project settings override global settings.
func Load() (*Config, error) {
	return LoadFromDir(".")
}

// LoadFromDir loads configuration from a specific working directory.
// Merge order: defaults → global file → project file.
func LoadFromDir(workDir string) (*Config, error) {
	cfg := Default()

	// 1. Load global config (if exists)
	globalPath := GlobalConfigPath()
	if data, err := os.ReadFile(globalPath); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse global config %s: %w", globalPath, err)
		}
	}

	// 2. Load project config (if exists) — overrides global
	for _, projectPath := range ProjectConfigPaths(workDir) {
		if data, err := os.ReadFile(projectPath); err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse project config %s: %w", projectPath, err)
			}
			break // First match wins
		}
	}

	return cfg, nil
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

// SaveProject saves the config to a project config path.
func (c *Config) SaveProject(workDir string) error {
	path := filepath.Join(workDir, "git-courer.yaml")

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

func detectOS() string {
	return runtime.GOOS
}
