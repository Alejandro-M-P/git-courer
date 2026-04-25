// Package config loads and merges git-courer configuration.
// Merge order: defaults → global (~/.config/git-courer/config.yaml) → project (.gcourer/config.yaml).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

// ServerName is the MCP server identifier registered with AI clients.
const ServerName = "git-courer"

// Config represents the git-courer configuration.
// All fields are editable by users via config files.
type Config struct {
	Ollama   OllamaConfig   `yaml:"ollama"`   // Ollama AI settings
	Git      GitConfig      `yaml:"git"`      // Git behavior settings
	Secrets  SecretsConfig  `yaml:"secrets"`  // Secrets detection
	Preview  PreviewConfig  `yaml:"preview"`  // Preview/confirmation settings
	Commit   CommitConfig   `yaml:"commit"`   // Commit workflow settings
	Release  ReleaseConfig  `yaml:"release"`  // Release workflow settings
	Commands CommandsConfig `yaml:"commands"` // Enabled operations
	Backup   BackupConfig   `yaml:"backup"`   // Auto-backup settings
}

// BackupConfig holds settings for the automatic backup system.
// [USER] Before every destructive _APPLY, git-courer creates a ref + optional stash.
// On success the backup is deleted. On failure it auto-restores and notifies the user.
type BackupConfig struct {
	Enabled bool `yaml:"enabled"` // [USER] Enable auto-backup
}

// CommandsConfig holds settings for enabled/disabled workflow commands.
// [USER] Which operations are allowed to run.
type CommandsConfig struct {
	EnabledOperations []string `yaml:"enabled_operations"` // [USER] List of enabled operation keys
}

// IsEnabled returns true if the given operation is in the list of enabled operations.
func (c CommandsConfig) IsEnabled(operationKey string) bool {
	for _, op := range c.EnabledOperations {
		if op == operationKey {
			return true
		}
	}
	return false
}

// OllamaConfig holds Ollama-related settings.
// [USER] Configure the Ollama AI backend.
type OllamaConfig struct {
	Host          string `yaml:"host"`           // [USER] Ollama server URL
	Model         string `yaml:"model"`          // [USER] Model to use
	ContextWindow int    `yaml:"context_window"` // [USER] Context window size (0 = default)
	AutoStart     bool   `yaml:"auto_start"`     // [USER] Auto-start Ollama if not running
	ModelsDir     string `yaml:"models_dir"`     // [USER] Custom models directory
}

// GitConfig holds git-related settings.
// [USER] Configure git behavior.
type GitConfig struct {
	WorkDir          string `yaml:"workdir"`            // [USER] Default working directory
	AutoAddSecrets   bool   `yaml:"auto_add_secrets"`   // [USER] Auto-stage detected secrets
	RequireCleanRepo bool   `yaml:"require_clean_repo"` // [USER] Require clean working tree
}

// SecretsConfig holds secrets detection settings.
// [USER] Configure secrets detection.
type SecretsConfig struct {
	DetectionMode      string   `yaml:"detection_mode"`        // [USER] Detection mode: regex, ai, regex+ai
	Patterns           []string `yaml:"patterns"`              // [USER] Regex patterns for secrets
	UseLLMSecurityScan string   `yaml:"use_llm_security_scan"` // [USER] Use LLM for security scan
}

// PreviewConfig holds preview/confirmation settings.
// [USER] Configure preview dialogs and confirmations.
type PreviewConfig struct {
	Enabled    bool            `yaml:"enabled"`    // [USER] Enable preview mode
	Operations map[string]bool `yaml:"operations"` // [USER] Operations requiring preview
}

// IsRequired returns true if confirmation is required for the given operation key.
func (p PreviewConfig) IsRequired(operationKey string) bool {
	if !p.Enabled {
		return false
	}
	return p.Operations[operationKey]
}

// CommitConfig holds commit-related settings including plan TTL and file paths.
// [USER] Configure commit workflow behavior.
type CommitConfig struct {
	TTL         DurationConfig `yaml:"ttl"`           // Plan TTL
	LogPath     string         `yaml:"log_path"`      // Log file path
	MaxLogLines int            `yaml:"max_log_lines"` // Max log lines
}

// ReleaseConfig holds release-related settings.
// [USER] Configure release workflow behavior.
type ReleaseConfig struct {
	LogPath            string `yaml:"log_path"`              // [USER] Log file path
	MaxLogLines        int    `yaml:"max_log_lines"`         // [USER] Max log lines
	MaxCommitsPerChunk int    `yaml:"max_commits_per_chunk"` // [USER] Max commits per chunk
}

// DurationConfig wraps time.Duration for YAML unmarshaling.
type DurationConfig struct {
	time.Duration
}

// NewDurationConfig creates a DurationConfig with the given duration.
func NewDurationConfig(d time.Duration) DurationConfig {
	return DurationConfig{Duration: d}
}

// UnmarshalYAML implements custom unmarshaling for DurationConfig.
func (d *DurationConfig) UnmarshalYAML(node *yaml.Node) error {
	var str string
	if err := node.Decode(&str); err != nil {
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

// MarshalYAML implements custom marshaling for DurationConfig.
func (d DurationConfig) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		Ollama: OllamaConfig{
			Host:          "http://localhost:11434",
			Model:         "gemma4:26b",
			ContextWindow: 0,
			AutoStart:     false,
			ModelsDir:     "",
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
				"*.key", "*.pem", ".env*", "credentials.json",
				"secrets.yaml", "*.password", "*.token",
				"(?i)DUMMY_AWS[0-9A-Z]{16}",         // AWS Access Key
				"(?i)SECRET_?[A-Z0-9]{16,64}", // Generic secret
				"(?i)TOKEN_?[A-Z0-9]{16,64}",  // Generic token
			},
		},
		Preview: PreviewConfig{
			Enabled: true,
			Operations: map[string]bool{
				"commit":            true,
				"branch_create":     true,
				"branch_delete":     true,
				"release":           true,
				"tag_create":        false,
				"tag_delete":        false,
				"tag_push":          false,
				"tag_delete_remote": false,
			},
		},
		Commit: CommitConfig{
			TTL:         NewDurationConfig(10 * time.Minute),
			LogPath:     ".gcourer/task.log",
			MaxLogLines: 500,
		},
		Release: ReleaseConfig{
			LogPath:            ".gcourer/release.log",
			MaxLogLines:        500,
			MaxCommitsPerChunk: 20,
		},
		Commands: CommandsConfig{
			EnabledOperations: []string{
				"commit",
				"release",
				"push",
				"pull",
				"branch_create",
				"branch_delete",
				"merge",
				"tag_create",
				"tag_delete",
				"tag_push",
				"tag_delete_remote",
			},
		},
		Backup: BackupConfig{
			Enabled: true,
		},
	}
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

// ProjectConfigPaths returns possible project config paths (first match wins).
func ProjectConfigPaths(workDir string) []string {
	return []string{
		filepath.Join(workDir, ".gcourer", "config.yaml"),
		filepath.Join(workDir, "git-courer", "config.yaml"),
		filepath.Join(workDir, "git-courer.yaml"),
		filepath.Join(workDir, ".git-courer.yaml"),
	}
}

// Load loads configuration from the current directory.
func Load() (*Config, error) {
	return LoadFromDir(".")
}

// LoadFromDir loads configuration with cascade: defaults → global → project.
func LoadFromDir(workDir string) (*Config, error) {
	cfg := Default()

	if data, err := os.ReadFile(GlobalConfigPath()); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse global config: %w", err)
		}
	}

	for _, projectPath := range ProjectConfigPaths(workDir) {
		if data, err := os.ReadFile(projectPath); err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("failed to parse project config %s: %w", projectPath, err)
			}
			break
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
