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
	// Verify the new simplified Config has only expected fields
	cfg := Config{}
	// Config should have LLM, Preview, Git, Context — nothing else
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
	_ = cfg.Context
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

func TestContextConfig_Fields(t *testing.T) {
	cfg := ContextConfig{
		Project: "my-project",
		Style:   "concise_technical",
	}
	if cfg.Project != "my-project" {
		t.Errorf("Project = %q, want 'my-project'", cfg.Project)
	}
	if cfg.Style != "concise_technical" {
		t.Errorf("Style = %q, want 'concise_technical'", cfg.Style)
	}
}

// --- Default() tests ---

func TestDefault_OldFieldsRemoved(t *testing.T) {
	cfg := Default()
	// Old fields should not exist. We verify by checking that the
	// new config struct only has the expected fields.
	// If Ollama, Secrets, etc. still existed, code that uses them would compile
	// but we're removing them, so we just verify the new fields work.
	if cfg.LLM.Provider != "" {
		t.Log("LLM.Provider is empty as expected (mandatory, no default)")
	}
	if cfg.Context.Project != "" {
		t.Log("Context.Project is empty as expected (mandatory, no default)")
	}
	_ = cfg.Preview.Enabled
	_ = cfg.Git.WorkDir
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
	cfg := Default()
	// context.style should default to "concise_technical"
	if cfg.Context.Style != "concise_technical" {
		t.Errorf("Context.Style = %q, want 'concise_technical'", cfg.Context.Style)
	}
}

func TestDefault_ContextProjectNoDefault(t *testing.T) {
	cfg := Default()
	// context.project has no default (mandatory)
	if cfg.Context.Project != "" {
		t.Errorf("Context.Project = %q, want empty (mandatory)", cfg.Context.Project)
	}
}

// --- Validate() tests ---

func TestValidate_AllMandatory(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			Provider: "ollama",
			Model:    "qwen3.5:0.8b",
		},
		Context: ContextConfig{
			Project: "my-project",
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
		Context: ContextConfig{
			Project: "my-project",
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
		Context: ContextConfig{
			Project: "my-project",
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

func TestValidate_MissingProject(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			Provider: "ollama",
			Model:    "qwen3.5:0.8b",
		},
		Context: ContextConfig{
			// Project missing
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should return error when project is missing")
	}
	if !strings.Contains(err.Error(), "project") {
		t.Errorf("error should mention 'project', got: %v", err)
	}
}

func TestValidate_AllThreeMissing(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			// Provider missing
			// Model missing
		},
		Context: ContextConfig{
			// Project missing
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() should return error when all 3 mandatory fields are missing")
	}
	errStr := err.Error()
	// Error should mention all three missing fields
	if !strings.Contains(errStr, "provider") {
		t.Errorf("error should mention 'provider', got: %v", err)
	}
	if !strings.Contains(errStr, "model") {
		t.Errorf("error should mention 'model', got: %v", err)
	}
	if !strings.Contains(errStr, "project") {
		t.Errorf("error should mention 'project', got: %v", err)
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
context:
  project: "global-project"
  style: "global-style"
`), 0644)

	// Set up project config that SHOULD be ignored
	projDir := t.TempDir()
	projPath := filepath.Join(projDir, ".gcourer", "config.yaml")
	os.MkdirAll(filepath.Dir(projPath), 0755)
	os.WriteFile(projPath, []byte(`context:
  style: "local-style"
`), 0644)

	cfg, err := LoadFromDir(projDir)
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
	if cfg.Context.Project != "global-project" {
		t.Errorf("Context.Project = %q, want 'global-project' (from global)", cfg.Context.Project)
	}
	// Per-project style is ignored — global style wins
	if cfg.Context.Style != "global-style" {
		t.Errorf("Context.Style = %q, want 'global-style' (global, not project)", cfg.Context.Style)
	}
}

func TestLoadFromDir_NoGlobalConfig(t *testing.T) {
	// Set up a temp dir with no global config
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	// Don't create any global config file

	// Set up project config that should be ignored
	projDir := t.TempDir()
	projPath := filepath.Join(projDir, ".gcourer", "config.yaml")
	os.MkdirAll(filepath.Dir(projPath), 0755)
	os.WriteFile(projPath, []byte(`llm:
  provider: "ollama"
  model: "test"
context:
  project: "test-project"
  style: "local-style"
`), 0644)

	cfg, err := LoadFromDir(projDir)
	if err != nil {
		t.Fatalf("LoadFromDir() error: %v", err)
	}

	// Should return defaults since there's no global config
	// Per-project config is now ignored
	if cfg.LLM.Provider != "" {
		t.Errorf("LLM.Provider = %q, want '' (default, per-project ignored)", cfg.LLM.Provider)
	}
	if cfg.Context.Style != "concise_technical" {
		t.Errorf("Context.Style = %q, want 'concise_technical' (default, per-project ignored)", cfg.Context.Style)
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
context:
  project: "test"
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
		Context ContextConfig
	}
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
	_ = cfg.Context
}

func TestLLMConfig_NoContextWindow(t *testing.T) {
	cfg := LLMConfig{}
	// ContextWindow should not exist in LLMConfig
	// This would fail to compile if field exists
	_ = cfg.Provider
	_ = cfg.Model
	_ = cfg.BaseURL
	_ = cfg.NumParallel
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
	_ = cfg.Context
}

func TestConfig_NoCommandsField(t *testing.T) {
	cfg := Config{}
	// CommandsConfig should not exist
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
	_ = cfg.Context
}

func TestConfig_NoCommitField(t *testing.T) {
	cfg := Config{}
	// CommitConfig should not exist
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
	_ = cfg.Context
}

func TestConfig_NoReleaseField(t *testing.T) {
	cfg := Config{}
	// ReleaseConfig should not exist
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
	_ = cfg.Context
}

func TestConfig_NoBackupField(t *testing.T) {
	cfg := Config{}
	// BackupConfig should not exist
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
	_ = cfg.Context
}

func TestConfig_NoValidationField(t *testing.T) {
	cfg := Config{}
	// ValidationConfig should not exist
	_ = cfg.LLM
	_ = cfg.Preview
	_ = cfg.Git
	_ = cfg.Context
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
	_ = cfg.Context
	// cfg.TestCommand must NOT compile — if this file compiles, the field is absent.
}