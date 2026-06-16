package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// ProjectConfig holds per-project settings stored in .git/git-courer/config.json.
// These values are committable and shared by the team.
// Legacy configs may contain "areas" field — it is silently ignored on load and not written on save.
type ProjectConfig struct {
	Description string              `json:"description"`
	PathTypes   map[string][]string `json:"path_types,omitempty"`
	TestCommand string              `json:"test_command"`
	UserName    string              `json:"user_name,omitempty"`
	UserEmail   string              `json:"user_email,omitempty"`
	SigningKey  string              `json:"signing_key,omitempty"`
	Excluded    []string            `json:"excluded,omitempty"`
}

// LoadProjectConfig reads .git/git-courer/config.json from the given working directory.
// Returns an error if the config file does not exist.
func LoadProjectConfig(workDir string) (*ProjectConfig, error) {
	configPath := filepath.Join(workDir, domain.MetadataDir, "config.json")
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

	// Initialize empty slice if Excluded was nil (JSON missing the key)
	if cfg.Excluded == nil {
		cfg.Excluded = []string{}
	}

	return cfg, nil
}

// SaveProjectConfig writes .git-courer/config.json to the given working directory.
// It performs a load-merge-write cycle to preserve unknown fields:
// 1. Read existing file (if any) as a raw map
// 2. Merge the structured fields from cfg into the map
// 3. Write back with json.MarshalIndent
func SaveProjectConfig(workDir string, cfg *ProjectConfig) error {
	gitcourerDir := filepath.Join(workDir, domain.MetadataDir)
	if err := os.MkdirAll(gitcourerDir, 0755); err != nil {
		return fmt.Errorf("failed to create .git/git-courer dir: %w", err)
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
	if len(cfg.PathTypes) > 0 {
		raw["path_types"] = cfg.PathTypes
	} else {
		delete(raw, "path_types")
	}
	raw["test_command"] = cfg.TestCommand
	if cfg.UserName != "" {
		raw["user_name"] = cfg.UserName
	} else {
		delete(raw, "user_name")
	}
	if cfg.UserEmail != "" {
		raw["user_email"] = cfg.UserEmail
	} else {
		delete(raw, "user_email")
	}
	if cfg.SigningKey != "" {
		raw["signing_key"] = cfg.SigningKey
	} else {
		delete(raw, "signing_key")
	}
	if len(cfg.Excluded) > 0 {
		raw["excluded"] = cfg.Excluded
	} else {
		delete(raw, "excluded")
	}

	// Write with indentation
	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal project config: %w", err)
	}
	out = append(out, '\n')
	return os.WriteFile(configPath, out, 0644)
}
