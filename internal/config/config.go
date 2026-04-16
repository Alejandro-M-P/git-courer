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

// Config represents the git-courer configuration.
type Config struct {
	Ollama     OllamaConfig     `yaml:"ollama"`
	Git        GitConfig        `yaml:"git"`
	Secrets    SecretsConfig    `yaml:"secrets"`
	Validation ValidationConfig `yaml:"validation"`
	UI         UIConfig         `yaml:"ui"`
	MCP        MCPConfig        `yaml:"mcp"`
	Preview    PreviewConfig    `yaml:"preview"`
	Commit     CommitConfig     `yaml:"commit"`
	Release    ReleaseConfig    `yaml:"release"`
	Commands   CommandsConfig   `yaml:"commands"`
	Backup     BackupConfig     `yaml:"backup"`
}

// BackupConfig holds settings for the automatic backup system.
// Before every destructive _APPLY, git-courer creates a ref + optional stash.
// On success the backup is deleted. On failure it auto-restores and notifies the user.
type BackupConfig struct {
	Enabled bool `yaml:"enabled"`
}

// CommandsConfig holds settings for enabled/disabled workflow commands.
type CommandsConfig struct {
	EnabledOperations []string `yaml:"enabled_operations"`
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
type OllamaConfig struct {
	Host          string `yaml:"host"`
	Model         string `yaml:"model"`
	ContextWindow int    `yaml:"context_window"`
	AutoStart     bool   `yaml:"auto_start"`
	ModelsDir     string `yaml:"models_dir"`
}

// GitConfig holds git-related settings.
type GitConfig struct {
	WorkDir          string `yaml:"workdir"`
	AutoAddSecrets   bool   `yaml:"auto_add_secrets"`
	RequireCleanRepo bool   `yaml:"require_clean_repo"`
}

// SecretsConfig holds secrets detection settings.
type SecretsConfig struct {
	DetectionMode      string   `yaml:"detection_mode"`
	Patterns           []string `yaml:"patterns"`
	UseLLMSecurityScan string   `yaml:"use_llm_security_scan"`
}

// ValidationConfig holds validation settings.
type ValidationConfig struct {
	RequireConfirmation bool `yaml:"require_confirmation"`
	MaxCommitLength     int  `yaml:"max_commit_length"`
}

// UIConfig holds UI settings.
type UIConfig struct {
	Theme     string `yaml:"theme"`
	ShowIcons bool   `yaml:"show_icons"`
}

// MCPConfig holds MCP server settings.
type MCPConfig struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

// PreviewConfig holds preview/confirmation settings.
type PreviewConfig struct {
	Enabled    bool            `yaml:"enabled"`
	Operations map[string]bool `yaml:"operations"`
}

// IsRequired returns true if confirmation is required for the given operation key.
func (p PreviewConfig) IsRequired(operationKey string) bool {
	if !p.Enabled {
		return false
	}
	return p.Operations[operationKey]
}

// CommitConfig holds commit-related settings including plan TTL and file paths.
type CommitConfig struct {
	TTL                 DurationConfig `yaml:"ttl"`
	MaxPlanRetries      int            `yaml:"max_plan_retries"`
	LockFile            string         `yaml:"lock_file"`
	PlanFile            string         `yaml:"plan_file"`
	BlockerFile         string         `yaml:"blocker_file"`
	LogPath             string         `yaml:"log_path"`
	MaxLogLines         int            `yaml:"max_log_lines"`
	BackgroundThreshold int            `yaml:"background_threshold"`
}

// ReleaseConfig holds release-related settings.
type ReleaseConfig struct {
	LogPath            string `yaml:"log_path"`
	MaxLogLines        int    `yaml:"max_log_lines"`
	MaxCommitsPerChunk int    `yaml:"max_commits_per_chunk"`
	ChangelogPath      string `yaml:"changelog_path"`
	IntentPath         string `yaml:"intent_path"`
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
			Version: "0.2.0",
		},
		Preview: PreviewConfig{
			Enabled: true,
			Operations: map[string]bool{
				"commit":        true,
				"branch_create": true,
				"branch_delete": true,
				"release":       true,
			},
		},
		Commit: CommitConfig{
			TTL:                 NewDurationConfig(10 * time.Minute),
			MaxPlanRetries:      3,
			LockFile:            ".gcourer/git-courer.lock",
			PlanFile:            ".gcourer/git-courer_plan.json",
			BlockerFile:         ".gcourer/git-courer_commit.lock",
			LogPath:             ".gcourer/task.log",
			MaxLogLines:         500,
			BackgroundThreshold: 10000,
		},
		Release: ReleaseConfig{
			LogPath:            ".gcourer/release.log",
			MaxLogLines:        500,
			MaxCommitsPerChunk: 20,
			ChangelogPath:      ".gcourer/release_changelog.md",
			IntentPath:         ".gcourer/release_intent.json",
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
