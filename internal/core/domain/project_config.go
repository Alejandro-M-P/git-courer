package domain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

// ProjectInitResult represents the structured response from the LLM for project initialization.
// It maps directly to the JSON output constrained by the project_init prompt template.
type ProjectInitResult struct {
	Description string              `json:"description"`
	Areas       map[string][]string `json:"areas"`
}

// ToProjectConfig converts the LLM result into a ProjectConfig for persistence.
// Returns a deep copy — modifying the result does not affect the original.
func (r *ProjectInitResult) ToProjectConfig() *ProjectConfig {
	areas := make(map[string][]string, len(r.Areas))
	for k, v := range r.Areas {
		paths := make([]string, len(v))
		copy(paths, v)
		areas[k] = paths
	}
	return &ProjectConfig{
		Description: r.Description,
		Areas:       areas,
	}
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

// FormatScopeContext produces a human-readable scope string from the project config,
// suitable for injecting into commit prompts via SetContext().
// Format: "description\nareas: key=path1,path2\n..."
// Returns empty string if config has neither description nor areas.
func (c *ProjectConfig) FormatScopeContext() string {
	if c.Description == "" && len(c.Areas) == 0 {
		return ""
	}

	var parts []string

	if c.Description != "" {
		parts = append(parts, c.Description)
	}

	if len(c.Areas) > 0 {
		areaLines := make([]string, 0, len(c.Areas))
		// Sort area names for deterministic output (map iteration is random)
		sortedAreas := sortedKeys(c.Areas)
		for _, area := range sortedAreas {
			paths := c.Areas[area]
			areaLines = append(areaLines, fmt.Sprintf("%s=%s", area, strings.Join(paths, ",")))
		}
		parts = append(parts, "areas: "+strings.Join(areaLines, "; "))
	}

	return strings.Join(parts, "\n")
}

// sortedKeys returns map keys in sorted order for deterministic output.
func sortedKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
