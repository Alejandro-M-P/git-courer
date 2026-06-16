package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProjectConfig_LoadExisting reads an existing config.json with description,
// verifies test_command is empty string (default) if not present.
// Legacy configs with "areas" field load without error.
func TestProjectConfig_LoadExisting(t *testing.T) {
	tmpDir := t.TempDir()
	gitcourerDir := filepath.Join(tmpDir, domain.MetadataDir)
	require.NoError(t, os.MkdirAll(gitcourerDir, 0755))

	existing := map[string]interface{}{
		"description": "test project",
		"areas": map[string]interface{}{
			"core": []string{"internal/"},
		},
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(gitcourerDir, "config.json"), data, 0644))

	cfg, err := LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "test project", cfg.Description)
	assert.Equal(t, "", cfg.TestCommand, "test_command should default to empty string when missing")
	// Legacy "areas" field is ignored — not in struct
}

// TestProjectConfig_LoadWithTestCommand verifies loading config with test_command set.
func TestProjectConfig_LoadWithTestCommand(t *testing.T) {
	tmpDir := t.TempDir()
	gitcourerDir := filepath.Join(tmpDir, domain.MetadataDir)
	require.NoError(t, os.MkdirAll(gitcourerDir, 0755))

	existing := map[string]interface{}{
		"description":  "test project",
		"test_command": "make test-ci",
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(gitcourerDir, "config.json"), data, 0644))

	cfg, err := LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "make test-ci", cfg.TestCommand)
}

// TestProjectConfig_SaveNew creates a config.json from scratch and saves it.
func TestProjectConfig_SaveNew(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &ProjectConfig{
		Description: "test project",
		TestCommand: "make test",
	}

	require.NoError(t, SaveProjectConfig(tmpDir, cfg))

	// Reload and verify
	loaded, err := LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "test project", loaded.Description)
	assert.Equal(t, "make test", loaded.TestCommand)
}

// TestProjectConfig_SavePreservesExistingFields loads a config with description,
// adds test_command, saves, then reloads to verify all fields are preserved.
// Legacy "areas" field in file is ignored on load and not written on save.
func TestProjectConfig_SavePreservesExistingFields(t *testing.T) {
	tmpDir := t.TempDir()
	gitcourerDir := filepath.Join(tmpDir, domain.MetadataDir)
	require.NoError(t, os.MkdirAll(gitcourerDir, 0755))

	existing := map[string]interface{}{
		"description": "existing project",
		"areas": map[string]interface{}{
			"core": []string{"internal/"},
			"docs": []string{"docs/"},
		},
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(gitcourerDir, "config.json"), data, 0644))

	// Load and modify
	cfg, err := LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	cfg.TestCommand = "go test ./..."

	// Save
	require.NoError(t, SaveProjectConfig(tmpDir, cfg))

	// Reload and verify fields
	loaded, err := LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "existing project", loaded.Description)
	assert.Equal(t, "go test ./...", loaded.TestCommand)
	// Legacy "areas" is not loaded into struct
}

// TestProjectConfig_LoadNonExistent returns an error when .git/git-courer/config.json doesn't exist.
func TestProjectConfig_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir() // no .git/git-courer directory

	_, err := LoadProjectConfig(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no project config found")
}

// TestProjectConfig_LoadLegacyAreas verifies that legacy configs with "areas" field
// load without error — the field is silently ignored.
func TestProjectConfig_LoadLegacyAreas(t *testing.T) {
	tmpDir := t.TempDir()
	gitcourerDir := filepath.Join(tmpDir, domain.MetadataDir)
	require.NoError(t, os.MkdirAll(gitcourerDir, 0755))

	existing := map[string]interface{}{
		"description": "legacy config",
		"areas": map[string]interface{}{
			"core": []string{"internal/"},
		},
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(gitcourerDir, "config.json"), data, 0644))

	cfg, err := LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "legacy config", cfg.Description)
	// areas field is ignored — no error, no data
}

// TestProjectConfig_SaveRoundTripUnknownFields verifies that unknown JSON fields
// are preserved across a load-save cycle.
func TestProjectConfig_SaveRoundTripUnknownFields(t *testing.T) {
	tmpDir := t.TempDir()
	gitcourerDir := filepath.Join(tmpDir, domain.MetadataDir)
	require.NoError(t, os.MkdirAll(gitcourerDir, 0755))

	// Write config with an unknown field
	content := `{
  "description": "project with extras",
  "areas": {},
  "test_command": "",
  "custom_field": "preserve_me"
}`
	require.NoError(t, os.WriteFile(filepath.Join(gitcourerDir, "config.json"), []byte(content), 0644))

	cfg, err := LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	cfg.TestCommand = "make test"

	require.NoError(t, SaveProjectConfig(tmpDir, cfg))

	// Read raw file and verify custom_field is preserved
	raw, err := os.ReadFile(filepath.Join(gitcourerDir, "config.json"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), "preserve_me")
	assert.Contains(t, string(raw), "make test")
}

// --- Excluded field tests ---

func TestProjectConfig_LoadExcluded(t *testing.T) {
	tmpDir := t.TempDir()
	gitcourerDir := filepath.Join(tmpDir, domain.MetadataDir)
	require.NoError(t, os.MkdirAll(gitcourerDir, 0755))

	existing := map[string]interface{}{
		"description": "test project",
		"areas":       map[string]interface{}{},
		"excluded":    []string{"docs", "test"},
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(gitcourerDir, "config.json"), data, 0644))

	cfg, err := LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, []string{"docs", "test"}, cfg.Excluded)
}

func TestProjectConfig_LoadExcludedNil(t *testing.T) {
	tmpDir := t.TempDir()
	gitcourerDir := filepath.Join(tmpDir, domain.MetadataDir)
	require.NoError(t, os.MkdirAll(gitcourerDir, 0755))

	existing := map[string]interface{}{
		"description": "no excluded key",
		"areas":       map[string]interface{}{},
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(gitcourerDir, "config.json"), data, 0644))

	cfg, err := LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Empty(t, cfg.Excluded, "Excluded should be empty slice when not present in config")
	assert.NotNil(t, cfg.Excluded, "Excluded should not be nil")
}

func TestProjectConfig_SaveExcluded(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &ProjectConfig{
		Description: "test project",
		TestCommand: "go test ./...",
		Excluded:    []string{"docs", "scripts", ".github"},
	}

	require.NoError(t, SaveProjectConfig(tmpDir, cfg))

	loaded, err := LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, []string{"docs", "scripts", ".github"}, loaded.Excluded)
}

func TestProjectConfig_SaveExcludedPreservesUnknownFields(t *testing.T) {
	tmpDir := t.TempDir()
	gitcourerDir := filepath.Join(tmpDir, domain.MetadataDir)
	require.NoError(t, os.MkdirAll(gitcourerDir, 0755))

	content := `{
  "description": "project with extras",
  "areas": {},
  "test_command": "",
  "custom_field": "preserve_me"
}`
	require.NoError(t, os.WriteFile(filepath.Join(gitcourerDir, "config.json"), []byte(content), 0644))

	cfg, err := LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	cfg.Excluded = []string{"docs", "test"}

	require.NoError(t, SaveProjectConfig(tmpDir, cfg))

	raw, err := os.ReadFile(filepath.Join(gitcourerDir, "config.json"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), "preserve_me")
	assert.Contains(t, string(raw), "docs")
}
