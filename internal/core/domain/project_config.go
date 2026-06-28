package domain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultExcluded paths filtered from area assignment and changelog.
// All prefixes MUST end with "/" to avoid partial matches (e.g. "docs/" won't match "docsify/").
var DefaultExcluded = []string{
	"docs/", ".github/", "scripts/", "test/", "assets/", "internal/shared/testutil/",
}

// DefaultPathTypes maps directory prefixes to commit type strings.
// Used by ResolvePathType when no custom PathTypes are configured.
var DefaultPathTypes = map[string][]string{
	"test": {"test/"},
	"ci":   {".github/workflows/", "ci/"},
	"docs": {"docs/"},
}

// ProjectConfig persists repository metadata for the commit classification system.
//
// Trailing slash rule: all path prefixes in PathTypes and Excluded MUST end
// with "/" to avoid partial matches (e.g. "docs/" won't match "docsify/foo.go").
// LoadProjectConfig normalises missing trailing slashes at load time.
type ProjectConfig struct {
	Description string              `json:"description"`
	PathTypes   map[string][]string `json:"path_types,omitempty"`
	TestCommand string              `json:"test_command,omitempty"`
	BaseBranch  string              `json:"base_branch,omitempty"`
	Excluded    []string            `json:"excluded,omitempty"`
}

// LoadProjectConfig reads the project configuration from disk.
// If the config file does not exist, it returns an empty config.
// Legacy configs with "areas" field load successfully — the field is ignored.
func LoadProjectConfig(repoRoot string) (*ProjectConfig, error) {
	path := filepath.Join(ResolveMetadataDir(repoRoot), "config.json")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return &ProjectConfig{}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Excluded == nil {
		cfg.Excluded = []string{}
	}

	cfg.normalise()

	return &cfg, nil
}

// Save writes the project configuration to disk.
func (c *ProjectConfig) Save(repoRoot string) error {
	dir := ResolveMetadataDir(repoRoot)
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

// normalise ensures all path prefixes end with "/" to prevent partial matches
// (e.g. "docs" matching "docsify/foo.go"). Directories without a trailing slash
// are rare in config but would cause subtle bugs, so we fix them silently.
func (c *ProjectConfig) normalise() {
	c.PathTypes = normalisePrefixMap(c.PathTypes)
	c.Excluded = normalisePrefixSlice(c.Excluded)
}

func normalisePrefixMap(m map[string][]string) map[string][]string {
	if m == nil {
		return nil
	}
	for key, prefixes := range m {
		m[key] = normalisePrefixSlice(prefixes)
	}
	return m
}

func normalisePrefixSlice(s []string) []string {
	for i, p := range s {
		if p != "" && !strings.HasSuffix(p, "/") {
			s[i] = p + "/"
		}
	}
	return s
}



// ResolvePathType maps a set of changed files to a commit type based on path prefixes.
// Returns the type only when ALL files match its prefixes (unanimity).
// Mixed-path commits return empty — they must not be classified by path type alone.
// When PathTypes is nil/empty, DefaultPathTypes is used.
func (c *ProjectConfig) ResolvePathType(files []string) string {
	pt := c.PathTypes
	if len(pt) == 0 {
		pt = DefaultPathTypes
	}

	if len(pt) == 0 || len(files) == 0 {
		return ""
	}

	type typeCount struct {
		typeName string
		count    int
	}

	counts := make([]typeCount, 0, len(pt))

	for typeName, prefixes := range pt {
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
			counts = append(counts, typeCount{typeName: typeName, count: matched})
		}
	}

	if len(counts) == 0 {
		return ""
	}

	winner := counts[0]
	for _, tc := range counts[1:] {
		if tc.count > winner.count {
			winner = tc
		} else if tc.count == winner.count && tc.typeName < winner.typeName {
			winner = tc
		}
	}

	// Only return when ALL files match the winning type (unanimity).
	// Mixed-path commits must not be classified by path type.
	if winner.count == len(files) {
		return winner.typeName
	}
	return ""
}

// IsExcluded returns true if path starts with any Excluded prefix.
// When Excluded is nil/empty, DefaultExcluded is used.
func (c *ProjectConfig) IsExcluded(path string) bool {
	excluded := c.Excluded
	if len(excluded) == 0 {
		excluded = DefaultExcluded
	}
	for _, prefix := range excluded {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// FormatScopeContext produces a human-readable scope string from the project config,
// suitable for injecting into commit prompts via SetContext().
// Returns only the description (no areas).
func (c *ProjectConfig) FormatScopeContext() string {
	return c.Description
}
