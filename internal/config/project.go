package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ProjectConfig holds per-project settings stored in .git-courer/config.json.
// These values are committable and shared by the team.
type ProjectConfig struct {
	Description string            `json:"description"`
	Areas       map[string][]string `json:"areas"`
	TestCommand string             `json:"test_command"`
}

// LoadProjectConfig reads .git-courer/config.json from the given working directory.
// Returns an error if the config file does not exist.
func LoadProjectConfig(workDir string) (*ProjectConfig, error) {
	configPath := filepath.Join(workDir, ".git-courer", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no project config found at %s", configPath)
		}
		return nil, fmt.Errorf("failed to read project config: %w", err)
	}

	cfg := &ProjectConfig{}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse project config: %w", err)
	}

	// Initialize empty map if Areas was nil (JSON missing the key)
	if cfg.Areas == nil {
		cfg.Areas = make(map[string][]string)
	}

	return cfg, nil
}

// SaveProjectConfig writes .git-courer/config.json to the given working directory.
// It performs a load-merge-write cycle to preserve unknown fields:
// 1. Read existing file (if any) as a raw map
// 2. Merge the structured fields from cfg into the map
// 3. Write back with json.MarshalIndent
func SaveProjectConfig(workDir string, cfg *ProjectConfig) error {
	gitcourerDir := filepath.Join(workDir, ".git-courer")
	if err := os.MkdirAll(gitcourerDir, 0755); err != nil {
		return fmt.Errorf("failed to create .git-courer dir: %w", err)
	}

	configPath := filepath.Join(gitcourerDir, "config.json")

	// Load existing file as raw map to preserve unknown fields
	raw := make(map[string]interface{})
	if data, err := os.ReadFile(configPath); err == nil {
		if json.Unmarshal(data, &raw) != nil {
			// If file is corrupt, start fresh
			raw = make(map[string]interface{})
		}
	}

	// Merge structured fields into raw map
	raw["description"] = cfg.Description
	raw["areas"] = cfg.Areas
	raw["test_command"] = cfg.TestCommand

	// Write with indentation
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project config: %w", err)
	}
	out = append(out, '\n')
	return os.WriteFile(configPath, out, 0644)
}
