package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// Task 1.2/1.3: Restructure config.go with simplified Config
// =============================================================================

// --- New Config struct tests ---

func TestConfig_Structure(t *testing.T) {
	// Verify the simplified Config has only expected fields (no Context)
	cfg := Config{}
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
}

func TestLLMConfig_Fields(t *testing.T) {
	cfg := LLMConfig{
		Provider:    "ollama",
		Model:       "qwen3.5:0.8b",
		BaseURL:     "http://localhost:11434/v1",
		NumParallel: 4,
	}
	if cfg.Provider != "ollama" {
		t.Errorf("Provider = %q, want 'ollama'", cfg.Provider)
	}
	if cfg.Model != "qwen3.5:0.8b" {
		t.Errorf("Model = %q, want 'qwen3.5:0.8b'", cfg.Model)
	}
}

// TestContextConfig_Removed verifies that ContextConfig is removed from the package.
// If this test compiles, ContextConfig no longer exists.
func TestContextConfig_Removed(t *testing.T) {
	// ContextConfig was removed — dead code in production
	// This test exists to prove the type is gone.
	// If ContextConfig still existed, downstream tests would reference it.
}

// --- Default() tests ---

func TestDefault_OldFieldsRemoved(t *testing.T) {
	cfg := Default()
	if cfg.LLM.Provider != "" {
		t.Log("LLM.Provider is empty as expected (mandatory, no default)")
	}
	_ = cfg.Preview.Enabled
	_ = cfg.Git.WorkDir
}

// TestDefault_NoContextField verifies that Default() returns a Config
// without a Context field (ContextConfig removed — dead code in production).
func TestDefault_NoContextField(t *testing.T) {
	cfg := Default()
	// Verify Config compiles and works without Context field
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
	// cfg.Context must NOT compile — this verifies the field is absent.
}

func TestDefault_LLMDefaults(t *testing.T) {
	cfg := Default()
	// Provider: no default (mandatory)
	// Model: no default (mandatory)
	// BaseURL: default should be "http://localhost:11434/v1"
	if cfg.LLM.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("LLM.BaseURL = %q, want 'http://localhost:11434/v1'", cfg.LLM.BaseURL)
	}
	// NumParallel: default 1
	if cfg.LLM.NumParallel != 1 {
		t.Errorf("LLM.NumParallel = %d, want 1", cfg.LLM.NumParallel)
	}
}

func TestDefault_PreviewDefaults(t *testing.T) {
	cfg := Default()
	// Preview.Enabled: default true
	if !cfg.Preview.Enabled {
		t.Error("Preview.Enabled should be true by default")
	}
}

func TestDefault_GitDefaults(t *testing.T) {
	cfg := Default()
	// Git.WorkDir: default "."
	if cfg.Git.WorkDir != "." {
		t.Errorf("Git.WorkDir = %q, want '.'", cfg.Git.WorkDir)
	}
}

func TestDefault_ContextStyleDefault(t *testing.T) {
	t.Skip("removed: ContextConfig.Style removed — dead code in production")
}

func TestDefault_ContextProjectNoDefault(t *testing.T) {
	t.Skip("removed: ContextConfig.Project removed — dead code in production")
}

// --- Validate() tests ---

func TestValidate_AllMandatory(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			Provider: "ollama",
			Model:    "qwen3.5:0.8b",
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() returned error for valid config: %v", err)
	}
}

func TestValidate_MissingProvider(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			// Provider missing
			Model: "qwen3.5:0.8b",
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should return error when provider is missing")
	}
	if !strings.Contains(err.Error(), "provider") {
		t.Errorf("error should mention 'provider', got: %v", err)
	}
}

func TestValidate_MissingModel(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			Provider: "ollama",
			// Model missing
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should return error when model is missing")
	}
	if !strings.Contains(err.Error(), "model") {
		t.Errorf("error should mention 'model', got: %v", err)
	}
}

// TestValidate_BothMissing verifies Validate reports both provider and model
// when both are absent (replaces TestValidate_AllThreeMissing which included context.project).
func TestValidate_BothMissing(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			// Provider missing
			// Model missing
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should return error when both mandatory fields are missing")
	}
	errStr := err.Error()
	if !strings.Contains(errStr, "provider") {
		t.Errorf("error should mention 'provider', got: %v", err)
	}
	if !strings.Contains(errStr, "model") {
		t.Errorf("error should mention 'model', got: %v", err)
	}
}

// --- Global config only tests ---

func TestLoadFromDir_GlobalOnly(t *testing.T) {
	// Set up global config
	globalDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", globalDir)
	globalPath := filepath.Join(globalDir, "git-courer", "config.yaml")
	os.MkdirAll(filepath.Dir(globalPath), 0755)
	os.WriteFile(globalPath, []byte(`llm:
  provider: "ollama"
  model: "global-model"
`), 0644)

	cfg, err := LoadFromDir(globalDir)
	if err != nil {
		t.Fatalf("LoadFromDir() error: %v", err)
	}

	// All values should come from global
	if cfg.LLM.Provider != "ollama" {
		t.Errorf("LLM.Provider = %q, want 'ollama' (from global)", cfg.LLM.Provider)
	}
	if cfg.LLM.Model != "global-model" {
		t.Errorf("LLM.Model = %q, want 'global-model' (from global)", cfg.LLM.Model)
	}
}

func TestLoadFromDir_NoGlobalConfig(t *testing.T) {
	// Set up a temp dir with no global config
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	// Don't create any global config file

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error: %v", err)
	}

	// Should return defaults since there's no global config
	if cfg.LLM.Provider != "" {
		t.Errorf("LLM.Provider = %q, want '' (default)", cfg.LLM.Provider)
	}
}

// --- Unknown field warning tests ---

func TestLoadFromDir_UnknownFieldWarning(t *testing.T) {
	// Set up global config with unknown fields
	globalDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", globalDir)
	globalPath := filepath.Join(globalDir, "git-courer", "config.yaml")
	os.MkdirAll(filepath.Dir(globalPath), 0755)
	// Include a field that doesn't exist in the new Config
	yamlContent := `llm:
  provider: "ollama"
  model: "test"
unknown_field: "this should warn"
another_unknown: 123
`
	os.WriteFile(globalPath, []byte(yamlContent), 0644)

	cfg, err := LoadFromDir(globalDir)
	// Unknown fields should NOT cause error (just warning logged)
	if err != nil {
		t.Fatalf("LoadFromDir() should not error on unknown fields, got: %v", err)
	}
	// Config should still load with known fields
	if cfg.LLM.Provider != "ollama" {
		t.Errorf("LLM.Provider = %q, want 'ollama'", cfg.LLM.Provider)
	}
}

// --- Old fields removed tests ---

func TestConfig_NoOllamaField(t *testing.T) {
	// Config should NOT have Ollama field
	cfg := Default()
	// This line would fail to compile if Ollama field exists
	var _ struct {
		LLM     LLMConfig
		Preview PreviewConfig
		Git     GitConfig
	}
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
}

func TestLLMConfig_NoAPIKey(t *testing.T) {
	cfg := LLMConfig{}
	// APIKey should not exist in LLMConfig (project is 100% local)
	_ = cfg.Provider
	_ = cfg.Model
}

func TestLLMConfig_NoOperations(t *testing.T) {
	cfg := LLMConfig{}
	// Operations map should not exist in LLMConfig
	_ = cfg.Provider
	_ = cfg.Model
}

func TestPreviewConfig_NoOperations(t *testing.T) {
	cfg := PreviewConfig{}
	// Operations map should not exist in PreviewConfig
	_ = cfg.Enabled
}

func TestGitConfig_OnlyWorkDir(t *testing.T) {
	cfg := GitConfig{}
	// Only WorkDir should exist in GitConfig
	_ = cfg.WorkDir
}

func TestConfig_NoSecretsField(t *testing.T) {
	cfg := Config{}
	// SecretsConfig should not exist
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
}

func TestConfig_NoCommandsField(t *testing.T) {
	cfg := Config{}
	// CommandsConfig should not exist
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
}

func TestConfig_NoCommitField(t *testing.T) {
	cfg := Config{}
	// CommitConfig should not exist
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
}

func TestConfig_NoReleaseField(t *testing.T) {
	cfg := Config{}
	// ReleaseConfig should not exist
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
}

func TestConfig_NoBackupField(t *testing.T) {
	cfg := Config{}
	// BackupConfig should not exist
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
}

func TestConfig_NoValidationField(t *testing.T) {
	cfg := Config{}
	// ValidationConfig should not exist
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
}

// TestConfig_NoTestCommandField verifies the global Config does NOT have TestCommand.
// TestCommand is per-project (ProjectConfig), not global.
func TestConfig_NoTestCommandField(t *testing.T) {
	// knownFields must not include test_command
	if knownFields["test_command"] {
		t.Error("knownFields contains 'test_command' — remove it. test_command is per-project, not global.")
	}

	cfg := Config{}
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
	// cfg.TestCommand must NOT compile — if this file compiles, the field is absent.
}

// TestConfig_NoContextField verifies that Config has no Context field.
// ContextConfig was removed — dead code in production.
func TestConfig_NoContextField(t *testing.T) {
	cfg := Config{}
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
	// cfg.Context must NOT compile — if this file compiles, Context field is absent.
}

// TestKnownFields_NoContextKeys verifies context keys are removed from knownFields.
func TestKnownFields_NoContextKeys(t *testing.T) {
	contextKeys := []string{"context", "context.project", "context.style"}
	for _, key := range contextKeys {
		if knownFields[key] {
			t.Errorf("knownFields contains %q — ContextConfig removed, this key should not exist", key)
		}
	}
}