package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestProjectConfig_LoadExisting reads an existing config.json with description and areas,
// verifies test_command is empty string (default) if not present.
func TestProjectConfig_LoadExisting(t *testing.T) {
	tmpDir := t.TempDir()
	gitcourerDir := filepath.Join(tmpDir, ".git-courer")
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
	assert.Contains(t, cfg.Areas, "core")
	assert.Equal(t, []string{"internal/"}, cfg.Areas["core"])
}

// TestProjectConfig_LoadWithTestCommand verifies loading config with test_command set.
func TestProjectConfig_LoadWithTestCommand(t *testing.T) {
	tmpDir := t.TempDir()
	gitcourerDir := filepath.Join(tmpDir, ".git-courer")
	require.NoError(t, os.MkdirAll(gitcourerDir, 0755))

	existing := map[string]interface{}{
		"description":  "test project",
		"areas":        map[string]interface{}{},
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
		Areas: map[string][]string{
			"core": {"internal/"},
		},
		TestCommand: "make test",
	}

	require.NoError(t, SaveProjectConfig(tmpDir, cfg))

	// Reload and verify
	loaded, err := LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "test project", loaded.Description)
	assert.Equal(t, "make test", loaded.TestCommand)
	assert.Equal(t, []string{"internal/"}, loaded.Areas["core"])
}

// TestProjectConfig_SavePreservesExistingFields loads a config with description and areas,
// adds test_command, saves, then reloads to verify all fields are preserved.
func TestProjectConfig_SavePreservesExistingFields(t *testing.T) {
	tmpDir := t.TempDir()
	gitcourerDir := filepath.Join(tmpDir, ".git-courer")
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

	// Reload and verify all fields
	loaded, err := LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "existing project", loaded.Description)
	assert.Equal(t, "go test ./...", loaded.TestCommand)
	assert.Equal(t, []string{"internal/"}, loaded.Areas["core"])
	assert.Equal(t, []string{"docs/"}, loaded.Areas["docs"])
}

// TestProjectConfig_LoadNonExistent returns an error when .git-courer/config.json doesn't exist.
func TestProjectConfig_LoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir() // no .git-courer directory

	_, err := LoadProjectConfig(tmpDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no project config found")
}

// TestProjectConfig_LoadEmptyAreas handles areas as nil when JSON has no areas key.
func TestProjectConfig_LoadEmptyAreas(t *testing.T) {
	tmpDir := t.TempDir()
	gitcourerDir := filepath.Join(tmpDir, ".git-courer")
	require.NoError(t, os.MkdirAll(gitcourerDir, 0755))

	existing := map[string]interface{}{
		"description": "minimal project",
		// No areas key
	}
	data, err := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(gitcourerDir, "config.json"), data, 0644))

	cfg, err := LoadProjectConfig(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "minimal project", cfg.Description)
	assert.Empty(t, cfg.Areas)
	assert.Equal(t, "", cfg.TestCommand)
}

// TestProjectConfig_SaveRoundTripUnknownFields verifies that unknown JSON fields
// are preserved across a load-save cycle.
func TestProjectConfig_SaveRoundTripUnknownFields(t *testing.T) {
	tmpDir := t.TempDir()
	gitcourerDir := filepath.Join(tmpDir, ".git-courer")
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
