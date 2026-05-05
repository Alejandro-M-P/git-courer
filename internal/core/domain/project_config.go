package domain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ProjectConfig persists repository metadata for the commit classification system.
type ProjectConfig struct {
	Description string              `json:"description"`
	Areas       map[string][]string `json:"areas"`
}

// LoadProjectConfig reads the project configuration from disk.
// If the config file does not exist, it returns an empty config.
func LoadProjectConfig(repoRoot string) (*ProjectConfig, error) {
	path := filepath.Join(repoRoot, ".git-courer", "config.json")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &ProjectConfig{Areas: make(map[string][]string)}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Areas == nil {
		cfg.Areas = make(map[string][]string)
	}

	return &cfg, nil
}

// Save writes the project configuration to disk.
func (c *ProjectConfig) Save(repoRoot string) error {
	dir := filepath.Join(repoRoot, ".git-courer")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	path := filepath.Join(dir, "config.json")
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// ResolveScope maps a set of changed files to a commit scope based on configured areas.
// Rules:
// 1. Cross chunk.Files paths with area paths.
// 2. Multiple matches -> area with most files wins.
// 3. Tie -> first area in config wins.
// 4. No match -> empty string.
func (c *ProjectConfig) ResolveScope(files []string) string {
	if len(c.Areas) == 0 || len(files) == 0 {
		return ""
	}

	type areaCount struct {
		area  string
		count int
	}

	counts := make([]areaCount, 0, len(c.Areas))

	for area, prefixes := range c.Areas {
		matched := 0
		for _, file := range files {
			for _, prefix := range prefixes {
				if strings.HasPrefix(file, prefix) {
					matched++
					break
				}
			}
		}
		if matched > 0 {
			counts = append(counts, areaCount{area: area, count: matched})
		}
	}

	if len(counts) == 0 {
		return ""
	}

	winner := counts[0]
	for _, ac := range counts[1:] {
		if ac.count > winner.count {
			winner = ac
		} else if ac.count == winner.count && ac.area < winner.area {
			// Deterministic tie-break: lexicographically smallest area name wins.
			winner = ac
		}
	}

	return winner.area
}
