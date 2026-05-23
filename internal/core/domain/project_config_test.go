package domain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProjectConfig_LoadSave(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	config := &ProjectConfig{
		Description: "A git commit helper",
		Areas: map[string][]string{
			"security": {"internal/auth", "internal/crypto"},
		},
	}

	if err := config.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error = %v", err)
	}

	if loaded.Description != config.Description {
		t.Errorf("Description = %q, want %q", loaded.Description, config.Description)
	}
	if len(loaded.Areas["security"]) != 2 {
		t.Errorf("Areas[security] len = %d, want 2", len(loaded.Areas["security"]))
	}
}

func TestProjectConfig_MissingConfig(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	loaded, err := LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error = %v", err)
	}
	if loaded.Description != "" {
		t.Errorf("Description = %q, want empty string", loaded.Description)
	}
	if loaded.Areas == nil {
		t.Error("Areas should be empty map, not nil")
	}
	if len(loaded.Areas) != 0 {
		t.Errorf("len(Areas) = %d, want 0", len(loaded.Areas))
	}
}

func TestProjectConfig_MalformedJSON(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, ".git-courer")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "config.json"), []byte("not json"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := LoadProjectConfig(tmpDir)
	if err == nil {
		t.Fatal("LoadProjectConfig() expected error for malformed JSON, got nil")
	}
}

func TestProjectConfig_ResolveScope_SingleMatch(t *testing.T) {
	t.Parallel()
	config := &ProjectConfig{
		Areas: map[string][]string{
			"security": {"internal/auth", "internal/crypto"},
		},
	}
	scope := config.ResolveScope([]string{"internal/auth/login.go", "internal/auth/tokens.go"})
	if scope != "security" {
		t.Errorf("ResolveScope = %q, want security", scope)
	}
}

func TestProjectConfig_ResolveScope_MultipleAreasTie(t *testing.T) {
	t.Parallel()
	config := &ProjectConfig{
		Areas: map[string][]string{
			"core":  {"internal/core"},
			"tui":   {"internal/ui"},
		},
	}
	scope := config.ResolveScope([]string{"internal/core/domain.go", "internal/ui/screen.go"})
	if scope != "core" {
		t.Errorf("ResolveScope = %q, want core (first in config wins)", scope)
	}
}

func TestProjectConfig_ResolveScope_NoMatch(t *testing.T) {
	t.Parallel()
	config := &ProjectConfig{
		Areas: map[string][]string{
			"security": {"internal/auth"},
		},
	}
	scope := config.ResolveScope([]string{"pkg/utils/helpers.go"})
	if scope != "" {
		t.Errorf("ResolveScope = %q, want empty string", scope)
	}
}

func TestProjectConfig_ResolveScope_NoAreasConfigured(t *testing.T) {
	t.Parallel()
	config := &ProjectConfig{}
	scope := config.ResolveScope([]string{"internal/auth/login.go"})
	if scope != "" {
		t.Errorf("ResolveScope = %q, want empty string", scope)
	}
}

func TestProjectConfig_ResolveScope_MostFilesWins(t *testing.T) {
	t.Parallel()
	config := &ProjectConfig{
		Areas: map[string][]string{
			"security": {"internal/auth"},
			"core":     {"internal/core"},
		},
	}
	scope := config.ResolveScope([]string{
		"internal/core/domain.go",
		"internal/core/ports.go",
		"internal/auth/login.go",
	})
	if scope != "core" {
		t.Errorf("ResolveScope = %q, want core (most files wins)", scope)
	}
}

// --- FormatScopeContext tests ---

func TestProjectConfig_FormatScopeContext_WithAreas(t *testing.T) {
	t.Parallel()
	config := &ProjectConfig{
		Description: "A git helper",
		Areas: map[string][]string{
			"security": {"internal/auth/", "internal/crypto/"},
			"core":     {"internal/core/domain/", "internal/core/ports/"},
		},
	}

	result := config.FormatScopeContext()

	if result == "" {
		t.Fatal("FormatScopeContext() returned empty string, expected non-empty scope context")
	}
	// Must contain description
	if !strings.Contains(result, "A git helper") {
		t.Errorf("FormatScopeContext() missing description; got:\n%s", result)
	}
	// Must contain area names
	if !strings.Contains(result, "security") {
		t.Errorf("FormatScopeContext() missing area 'security'; got:\n%s", result)
	}
	if !strings.Contains(result, "core") {
		t.Errorf("FormatScopeContext() missing area 'core'; got:\n%s", result)
	}
	// Must contain path mappings
	if !strings.Contains(result, "internal/auth/") {
		t.Errorf("FormatScopeContext() missing path 'internal/auth/'; got:\n%s", result)
	}
}

func TestProjectConfig_FormatScopeContext_Empty(t *testing.T) {
	t.Parallel()
	config := &ProjectConfig{}

	result := config.FormatScopeContext()

	if result != "" {
		t.Errorf("FormatScopeContext() on empty config = %q, want empty string", result)
	}
}

func TestProjectConfig_FormatScopeContext_DescriptionOnly(t *testing.T) {
	t.Parallel()
	config := &ProjectConfig{
		Description: "My project",
	}

	result := config.FormatScopeContext()

	if !strings.Contains(result, "My project") {
		t.Errorf("FormatScopeContext() missing description; got:\n%s", result)
	}
	// Should still have the description even without areas
	if result == "" {
		t.Error("FormatScopeContext() returned empty string for description-only config")
	}
	// Should NOT contain "areas:" if there are no areas
	if strings.Contains(result, "areas:") {
		t.Errorf("FormatScopeContext() should not contain 'areas:' with no areas; got:\n%s", result)
	}
}

// --- IsExcluded tests ---

func TestProjectConfig_IsExcluded_DefaultPaths(t *testing.T) {
	t.Parallel()
	cfg := &ProjectConfig{}
	// When Excluded is nil, DefaultExcluded is used
	tests := []struct {
		path string
		want bool
	}{
		{"docs/README.md", true},
		{".github/workflows/ci.yml", true},
		{"scripts/build.sh", true},
		{"test/integration_test.go", true},
		{"assets/logo.png", true},
		{"internal/shared/testutil/mock.go", true},
		{"internal/core/domain.go", false},
		{"cmd/main.go", false},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := cfg.IsExcluded(tc.path)
			if got != tc.want {
				t.Errorf("IsExcluded(%q) = %v, want %v (using DefaultExcluded)", tc.path, got, tc.want)
			}
		})
	}
}

func TestProjectConfig_IsExcluded_CustomExcludedOverridesDefaults(t *testing.T) {
	t.Parallel()
	cfg := &ProjectConfig{
		Excluded: []string{"vendor"},
	}
	// Custom excluded replaces defaults, not merges
	if cfg.IsExcluded("docs/README.md") {
		t.Error("IsExcluded(docs/) = true, want false — custom excluded should NOT include default paths")
	}
	if !cfg.IsExcluded("vendor/pkg.go") {
		t.Error("IsExcluded(vendor/) = false, want true — custom excluded path should match")
	}
}

func TestProjectConfig_IsExcluded_EmptySliceUsesDefaults(t *testing.T) {
	t.Parallel()
	cfg := &ProjectConfig{
		Excluded: []string{},
	}
	// Empty slice should use defaults (same as nil)
	if !cfg.IsExcluded("docs/README.md") {
		t.Error("IsExcluded(docs/) = false with empty slice, want true — should use DefaultExcluded")
	}
}

func TestProjectConfig_IsExcluded_PrefixMatching(t *testing.T) {
	t.Parallel()
	cfg := &ProjectConfig{
		Excluded: []string{"internal/shared/testutil"},
	}
	// Path starting with the excluded prefix should match
	if !cfg.IsExcluded("internal/shared/testutil/mock.go") {
		t.Error("IsExcluded(internal/shared/testutil/mock.go) = false, want true — prefix match")
	}
}

// --- NewDirectories tests ---

func TestProjectConfig_NewDirectories_SomeDirsHaveNoArea(t *testing.T) {
	t.Parallel()
	cfg := &ProjectConfig{
		Areas: map[string][]string{
			"core": {"internal/core"},
		},
	}
	files := []string{
		"internal/core/domain.go",
		"internal/infra/cfg/db.go",
	}
	got := cfg.NewDirectories(files)
	want := []string{"internal/infra/cfg"}
	if len(got) != len(want) {
		t.Fatalf("NewDirectories = %v, want %v", got, want)
	}
	for i, d := range got {
		if d != want[i] {
			t.Errorf("NewDirectories[%d] = %q, want %q", i, d, want[i])
		}
	}
}

func TestProjectConfig_NewDirectories_AllDirsMapped(t *testing.T) {
	t.Parallel()
	cfg := &ProjectConfig{
		Areas: map[string][]string{
			"security": {"internal/auth"},
		},
	}
	files := []string{"internal/auth/login.go"}
	got := cfg.NewDirectories(files)
	if len(got) != 0 {
		t.Errorf("NewDirectories = %v, want empty (all mapped)", got)
	}
}

func TestProjectConfig_NewDirectories_ExcludedDirFilteredOut(t *testing.T) {
	t.Parallel()
	cfg := &ProjectConfig{}
	// DefaultExcluded includes "docs"
	files := []string{"docs/api.md", "internal/core/domain.go"}
	got := cfg.NewDirectories(files)
	for _, d := range got {
		if d == "docs" || strings.HasPrefix(d, "docs/") {
			t.Errorf("NewDirectories should not include excluded dir %q", d)
		}
	}
}

func TestProjectConfig_NewDirectories_NoAreasConfigured(t *testing.T) {
	t.Parallel()
	cfg := &ProjectConfig{}
	// No areas, no excluded — everything is new
	files := []string{"internal/core/domain.go"}
	got := cfg.NewDirectories(files)
	if len(got) != 1 || got[0] != "internal/core" {
		t.Errorf("NewDirectories = %v, want [internal/core]", got)
	}
}

func TestProjectConfig_NewDirectories_DeduplicatedAndSorted(t *testing.T) {
	t.Parallel()
	cfg := &ProjectConfig{
		Areas: map[string][]string{
			"core": {"internal/core"},
		},
	}
	// Two files in same directory should produce one entry
	files := []string{
		"internal/core/domain.go",
		"internal/core/ports.go",
		"internal/infra/cfg/a.go",
		"internal/infra/cfg/b.go",
	}
	got := cfg.NewDirectories(files)
	if len(got) != 1 {
		t.Errorf("NewDirectories = %v, want 1 unique directory, got %d", got, len(got))
	}
	if len(got) > 0 && got[0] != "internal/infra/cfg" {
		t.Errorf("NewDirectories[0] = %q, want %q", got[0], "internal/infra/cfg")
	}
}

// --- ResolvePathType tests ---

func TestProjectConfig_ResolvePathType_DefaultPathTypes_TestDir(t *testing.T) {
	t.Parallel()
	cfg := &ProjectConfig{} // uses DefaultPathTypes
	got := cfg.ResolvePathType([]string{"test/pipeline/runner.go", "test/helpers/setup.go"})
	if got != "test" {
		t.Errorf("ResolvePathType = %q, want %q", got, "test")
	}
}

func TestProjectConfig_ResolvePathType_DefaultPathTypes_CI(t *testing.T) {
	t.Parallel()
	cfg := &ProjectConfig{}
	got := cfg.ResolvePathType([]string{".github/workflows/ci.yml"})
	if got != "ci" {
		t.Errorf("ResolvePathType = %q, want %q", got, "ci")
	}
}

func TestProjectConfig_ResolvePathType_EmptyPathTypesReturnsEmpty(t *testing.T) {
	t.Parallel()
	// Nil PathTypes uses DefaultPathTypes, but if files don't match any default, return ""
	cfg := &ProjectConfig{}
	got := cfg.ResolvePathType([]string{"src/app/main.go"})
	if got != "" {
		t.Errorf("ResolvePathType = %q, want empty string (no match)", got)
	}
}

func TestProjectConfig_ResolvePathType_AmbiguousPathsResolveByMajority(t *testing.T) {
	t.Parallel()
	cfg := &ProjectConfig{
		PathTypes: map[string][]string{
			"test": {"test/"},
			"ci":   {"ci/", ".github/workflows/"},
		},
	}
	got := cfg.ResolvePathType([]string{"test/runner.go", ".github/workflows/build.yml", ".github/workflows/deploy.yml"})
	if got != "ci" {
		t.Errorf("ResolvePathType = %q, want %q (2 ci vs 1 test)", got, "ci")
	}
}

func TestProjectConfig_ResolvePathType_NoMatchReturnsEmpty(t *testing.T) {
	t.Parallel()
	cfg := &ProjectConfig{
		PathTypes: map[string][]string{
			"test": {"test/"},
		},
	}
	got := cfg.ResolvePathType([]string{"src/app/main.go"})
	if got != "" {
		t.Errorf("ResolvePathType = %q, want empty string", got)
	}
}

func TestProjectConfig_ResolvePathType_CustomOverridesDefaults(t *testing.T) {
	t.Parallel()
	cfg := &ProjectConfig{
		PathTypes: map[string][]string{
			"scripts": {"scripts/"},
		},
	}
	got := cfg.ResolvePathType([]string{"scripts/build.sh"})
	if got != "scripts" {
		t.Errorf("ResolvePathType = %q, want %q", got, "scripts")
	}
}

func TestProjectConfig_ResolvePathType_SingleFileMatch(t *testing.T) {
	t.Parallel()
	cfg := &ProjectConfig{} // uses DefaultPathTypes
	got := cfg.ResolvePathType([]string{"docs/guide.md"})
	if got != "docs" {
		t.Errorf("ResolvePathType = %q, want %q", got, "docs")
	}
}