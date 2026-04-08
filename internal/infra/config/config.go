package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

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
	Preview    PreviewConfig    `yaml:"preview"`
	Commit     CommitConfig     `yaml:"commit"`
}

// OllamaConfig holds Ollama-related settings
type OllamaConfig struct {
	Host          string `yaml:"host"`
	Model         string `yaml:"model"`
	ContextWindow int    `yaml:"context_window"` // Max tokens for context (0 = default)
	AutoStart     bool   `yaml:"auto_start"`
	ModelsDir     string `yaml:"models_dir"` // Custom models directory (for distrobox, etc.)
}

// GitConfig holds git-related settings
type GitConfig struct {
	WorkDir          string `yaml:"workdir"`
	AutoAddSecrets   bool   `yaml:"auto_add_secrets"`
	RequireCleanRepo bool   `yaml:"require_clean_repo"`
}

// SecretsConfig holds secrets detection settings
type SecretsConfig struct {
	DetectionMode      string   `yaml:"detection_mode"`
	Patterns           []string `yaml:"patterns"`
	UseLLMSecurityScan string   `yaml:"use_llm_security_scan"` // "auto", "true", "false"
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

// PreviewConfig holds preview feature settings
type PreviewConfig struct {
	Enabled    bool            `yaml:"enabled"`
	Operations map[string]bool `yaml:"operations"`
}

// CommitConfig holds commit-related settings including plan TTL
type CommitConfig struct {
	TTL            DurationConfig `yaml:"ttl"`              // Plan time-to-live (default: 10 minutes)
	MaxPlanRetries int            `yaml:"max_plan_retries"` // Max retries for plan operations
	LockFile       string         `yaml:"lock_file"`        // Lock file path (relative to workDir)
	PlanFile       string         `yaml:"plan_file"`        // Plan file path (relative to workDir)
}

// DurationConfig wraps time.Duration for YAML unmarshaling
type DurationConfig struct {
	time.Duration
}

// NewDurationConfig creates a DurationConfig with the given duration
func NewDurationConfig(d time.Duration) DurationConfig {
	return DurationConfig{Duration: d}
}

// UnmarshalYAML implements custom unmarshaling for DurationConfig
func (d *DurationConfig) UnmarshalYAML(node *yaml.Node) error {
	var str string
	if err := node.Decode(&str); err != nil {
		// Try decoding as raw duration value (int seconds)
		var seconds int
		if err2 := node.Decode(&seconds); err2 == nil {
			d.Duration = time.Duration(seconds) * time.Second
			return nil
		}
		return err
	}
	parsed, err := time.ParseDuration(str)
	if err != nil {
		return fmt.Errorf("invalid duration format %q: %w", str, err)
	}
	d.Duration = parsed
	return nil
}

// MarshalYAML implements custom marshaling for DurationConfig
func (d DurationConfig) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

// Default returns the default configuration (base defaults, no files)
func Default() *Config {
	return &Config{
		Ollama: OllamaConfig{
			Host:          "http://localhost:11434",
			Model:         "qwen3.5",
			ContextWindow: 0, // 0 = use Ollama default
			AutoStart:     false,
			ModelsDir:     "", // Empty = use Ollama default (~/.ollama/models)
		},
		Git: GitConfig{
			WorkDir:          ".",
			AutoAddSecrets:   true,
			RequireCleanRepo: false,
		},
		Secrets: SecretsConfig{
			DetectionMode:      "regex+ai",
			UseLLMSecurityScan: "auto",
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
		Preview: PreviewConfig{
			Enabled: true,
			Operations: map[string]bool{
				"commit":        true,
				"branch_create": true,
				"branch_delete": true,
				"merge":         true,
				"push_force":    true,
				"reset_hard":    true,
				"rebase":        true,
				"stash_drop":    true,
			},
		},
		Commit: CommitConfig{
			TTL:            NewDurationConfig(10 * time.Minute),
			MaxPlanRetries: 3,
			LockFile:       ".gcourer/git-courer.lock",
			PlanFile:       ".gcourer/git-courer_plan.json",
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
		filepath.Join(workDir, "git-courer", "config.yaml"),
		filepath.Join(workDir, "git-courer.yaml"),
		filepath.Join(workDir, ".git-courer.yaml"),
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
