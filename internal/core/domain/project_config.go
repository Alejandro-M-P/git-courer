package domain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
// Trailing slash rule: all path prefixes in Areas, PathTypes, and Excluded MUST end
// with "/" because matching uses strings.HasPrefix.  A prefix like "docs" would match
// "docsify/foo.go", while "docs/" only matches paths under the docs directory.
// LoadProjectConfig normalises missing trailing slashes at load time.
type ProjectConfig struct {
	Description string              `json:"description"`
	Areas       map[string][]string `json:"areas"`
	PathTypes   map[string][]string `json:"path_types,omitempty"`
	Excluded    []string            `json:"excluded,omitempty"`
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
	if cfg.Excluded == nil {
		cfg.Excluded = []string{}
	}

	cfg.normalise()

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


// normalise ensures all path prefixes end with "/" to prevent partial matches
// (e.g. "docs" matching "docsify/foo.go"). Directories without a trailing slash
// are rare in config but would cause subtle bugs, so we fix them silently.
func (c *ProjectConfig) normalise() {
	c.Areas = normalisePrefixMap(c.Areas)
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

// NewDirectories returns directory prefixes from files that have no area
// mapping and are not excluded. Results are deduplicated and sorted.
func (c *ProjectConfig) NewDirectories(files []string) []string {
	seen := make(map[string]bool)
	var dirs []string

	for _, file := range files {
		if c.IsExcluded(file) {
			continue
		}

		// Find the longest-prefix area match
		maxLen := 0
		for _, prefixes := range c.Areas {
			for _, prefix := range prefixes {
				if strings.HasPrefix(file, prefix) && len(prefix) > maxLen {
					maxLen = len(prefix)
				}
			}
		}

		// If no area covers this file, extract the directory part
		if maxLen == 0 {
			dir := filepath.Dir(file)
			if dir == "." {
				dir = ""
			}
			if dir != "" && !seen[dir] {
				seen[dir] = true
				dirs = append(dirs, dir)
			}
		}
	}

	sort.Strings(dirs)
	return dirs
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
