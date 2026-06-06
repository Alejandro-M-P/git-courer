package domain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectConfig_LoadSave(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	config := &ProjectConfig{
		Description: "A git commit helper",
		PathTypes: map[string][]string{
			"test": {"test/"},
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
	if len(loaded.PathTypes["test"]) != 1 {
		t.Errorf("PathTypes[test] len = %d, want 1", len(loaded.PathTypes["test"]))
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
	if len(loaded.PathTypes) != 0 {
		t.Errorf("len(PathTypes) = %d, want 0", len(loaded.PathTypes))
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

// --- FormatScopeContext tests ---

func TestProjectConfig_FormatScopeContext_DescriptionOnly(t *testing.T) {
	t.Parallel()
	config := &ProjectConfig{
		Description: "My project",
	}

	result := config.FormatScopeContext()

	if result != "My project" {
		t.Errorf("FormatScopeContext() = %q, want %q", result, "My project")
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

// Test to verify ProjectConfig without Areas field compiles and loads correctly
func TestProjectConfig_NoAreasField(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	config := &ProjectConfig{
		Description: "No areas project",
	}

	if err := config.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error = %v", err)
	}

	if loaded.Description != "No areas project" {
		t.Errorf("Description = %q, want %q", loaded.Description, "No areas project")
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

func TestProjectConfig_ResolvePathType_AmbiguousPathsReturnsEmpty(t *testing.T) {
	t.Parallel()
	cfg := &ProjectConfig{
		PathTypes: map[string][]string{
			"test": {"test/"},
			"ci":   {"ci/", ".github/workflows/"},
		},
	}
	// Mixed paths: 2 ci + 1 test → no unanimity → empty
	got := cfg.ResolvePathType([]string{"test/runner.go", ".github/workflows/build.yml", ".github/workflows/deploy.yml"})
	if got != "" {
		t.Errorf("ResolvePathType = %q, want empty string (mixed ci+test, no unanimity)", got)
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

func TestProjectConfig_BaseBranch_RoundTrip(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	config := &ProjectConfig{
		Description: "test project",
		BaseBranch:  "main",
	}

	if err := config.Save(tmpDir); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	loaded, err := LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error = %v", err)
	}

	if loaded.BaseBranch != "main" {
		t.Errorf("BaseBranch = %q, want %q", loaded.BaseBranch, "main")
	}
}

func TestProjectConfig_BaseBranch_DefaultEmpty(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, ".git-courer")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	// Config with no base_branch key
	configJSON := `{"description":"test"}`
	if err := os.WriteFile(filepath.Join(repoDir, "config.json"), []byte(configJSON), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	loaded, err := LoadProjectConfig(tmpDir)
	if err != nil {
		t.Fatalf("LoadProjectConfig() error = %v", err)
	}

	if loaded.BaseBranch != "" {
		t.Errorf("BaseBranch = %q, want empty string (default)", loaded.BaseBranch)
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
