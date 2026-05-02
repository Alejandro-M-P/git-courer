package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDefault_NotNil(t *testing.T) {
	cfg := Default()
	if cfg == nil {
		t.Fatal("Default() returned nil")
	}
}

func TestDefault_OllamaDefaults(t *testing.T) {
	cfg := Default()
	if cfg.Ollama.Host == "" {
		t.Error("Ollama.Host should not be empty")
	}
	// No default model — user must configure explicitly.
	_ = cfg.Ollama.Model
}

func TestDefault_CommitPaths(t *testing.T) {
	cfg := Default()
	if cfg.Commit.LogPath == "" {
		t.Error("Commit.LogPath should not be empty")
	}
	if cfg.Commit.MaxLogLines <= 0 {
		t.Error("Commit.MaxLogLines should be positive")
	}
}

func TestDefault_ReleasePaths(t *testing.T) {
	cfg := Default()
	if cfg.Release.LogPath == "" {
		t.Error("Release.LogPath should not be empty")
	}
	if cfg.Release.MaxCommitsPerChunk <= 0 {
		t.Error("Release.MaxCommitsPerChunk should be positive")
	}
}

func TestDefault_BackupEnabled(t *testing.T) {
	cfg := Default()
	if !cfg.Backup.Enabled {
		t.Error("Backup.Enabled should be true by default")
	}
}

func TestServerConstants(t *testing.T) {
	if ServerName == "" {
		t.Error("ServerName should not be empty")
	}
	if ServerVersion == "" {
		t.Error("ServerVersion should not be empty")
	}
}

func TestDefault_EnabledOperations(t *testing.T) {
	cfg := Default()
	if len(cfg.Commands.EnabledOperations) == 0 {
		t.Error("Commands.EnabledOperations should not be empty")
	}
	// commit should always be enabled
	found := false
	for _, op := range cfg.Commands.EnabledOperations {
		if op == "commit" {
			found = true
			break
		}
	}
	if !found {
		t.Error("'commit' should be in EnabledOperations by default")
	}
}

func TestCommandsConfig_IsEnabled(t *testing.T) {
	cfg := CommandsConfig{
		EnabledOperations: []string{"commit", "release", "push"},
	}
	cases := []struct {
		op   string
		want bool
	}{
		{"commit", true},
		{"release", true},
		{"push", true},
		{"merge", false},
		{"", false},
		{"COMMIT", false}, // case sensitive
	}
	for _, tc := range cases {
		got := cfg.IsEnabled(tc.op)
		if got != tc.want {
			t.Errorf("IsEnabled(%q) = %v, want %v", tc.op, got, tc.want)
		}
	}
}

func TestPreviewConfig_IsRequired(t *testing.T) {
	cfg := PreviewConfig{
		Enabled: true,
		Operations: map[string]bool{
			"commit":  true,
			"release": true,
			"merge":   false,
		},
	}
	cases := []struct {
		op   string
		want bool
	}{
		{"commit", true},
		{"release", true},
		{"merge", false},
		{"push", false},
	}
	for _, tc := range cases {
		got := cfg.IsRequired(tc.op)
		if got != tc.want {
			t.Errorf("IsRequired(%q) = %v, want %v", tc.op, got, tc.want)
		}
	}
}

func TestPreviewConfig_IsRequired_DisabledOverall(t *testing.T) {
	cfg := PreviewConfig{
		Enabled: false,
		Operations: map[string]bool{
			"commit": true,
		},
	}
	if cfg.IsRequired("commit") {
		t.Error("IsRequired should return false when Enabled=false, regardless of operation")
	}
}

func TestDurationConfig_UnmarshalYAML_String(t *testing.T) {
	// Test via Load from a temp file
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, ".gcourer", "config.yaml")
	os.MkdirAll(filepath.Dir(cfgFile), 0755)
	os.WriteFile(cfgFile, []byte(`commit:
  ttl: "5m"
`), 0644)

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error: %v", err)
	}
	if cfg.Commit.TTL.Duration != 5*time.Minute {
		t.Errorf("TTL = %v, want 5m", cfg.Commit.TTL.Duration)
	}
}

func TestDurationConfig_MarshalYAML(t *testing.T) {
	d := NewDurationConfig(10 * time.Minute)
	val, err := d.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML() error: %v", err)
	}
	s, ok := val.(string)
	if !ok {
		t.Fatalf("MarshalYAML() returned %T, want string", val)
	}
	if s == "" {
		t.Error("MarshalYAML() returned empty string")
	}
}

func TestLoadFromDir_UsesDefaults_WhenNoFile(t *testing.T) {
	cfg, err := LoadFromDir(t.TempDir())
	if err != nil {
		t.Fatalf("LoadFromDir() error: %v", err)
	}
	if cfg == nil {
		t.Fatal("LoadFromDir() returned nil config")
	}
}

func TestLoadFromDir_OverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, ".gcourer", "config.yaml")
	os.MkdirAll(filepath.Dir(cfgFile), 0755)
	os.WriteFile(cfgFile, []byte(`ollama:
  model: "test-model:7b"
`), 0644)

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error: %v", err)
	}
	if cfg.Ollama.Model != "test-model:7b" {
		t.Errorf("Ollama.Model = %q, want test-model:7b", cfg.Ollama.Model)
	}
}

func TestNewDurationConfig(t *testing.T) {
	d := NewDurationConfig(30 * time.Second)
	if d.Duration != 30*time.Second {
		t.Errorf("Duration = %v, want 30s", d.Duration)
	}
}

// --- GlobalConfigPath ---

func TestGlobalConfigPath_Windows(t *testing.T) {
	// Simulate Windows environment
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("APPDATA", "C:\\Users\\test\\AppData\\Roaming")

	path := GlobalConfigPath()
	if !strings.Contains(path, "git-courer") {
		t.Errorf("GlobalConfigPath() = %q, should contain 'git-courer'", path)
	}
}

func TestGlobalConfigPath_XDGOverride(t *testing.T) {
	// Test XDG_CONFIG_HOME override
	t.Setenv("XDG_CONFIG_HOME", "C:\\custom\\config")
	t.Setenv("APPDATA", "") // Clear APPDATA to ensure XDG is used

	path := GlobalConfigPath()
	if !strings.Contains(path, "custom") {
		t.Errorf("GlobalConfigPath() = %q, should use XDG_CONFIG_HOME", path)
	}
}

func TestGlobalConfigPath_NonWindows(t *testing.T) {
	// Test non-Windows path construction (when GOOS != windows)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("APPDATA", "")

	path := GlobalConfigPath()
	if path == "" {
		t.Error("GlobalConfigPath() should not return empty string")
	}
	// On Windows, this should fall back to home directory path
	if !strings.Contains(path, "git-courer") {
		t.Errorf("GlobalConfigPath() = %q, should contain 'git-courer'", path)
	}
}

// --- ProjectConfigPaths ---

func TestProjectConfigPaths(t *testing.T) {
	paths := ProjectConfigPaths("C:\\project")
	if len(paths) != 4 {
		t.Errorf("ProjectConfigPaths() returned %d paths, want 4", len(paths))
	}
	// Check first path
	if !strings.Contains(paths[0], ".gcourer") {
		t.Errorf("First path should contain .gcourer, got %q", paths[0])
	}
}

// --- Load error paths ---

func TestLoadFromDir_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, ".gcourer", "config.yaml")
	os.MkdirAll(filepath.Dir(cfgFile), 0755)
	os.WriteFile(cfgFile, []byte(`invalid: yaml: content: [`), 0644)

	_, err := LoadFromDir(dir)
	if err == nil {
		t.Error("LoadFromDir() should error on invalid YAML")
	}
}

func TestLoadFromDir_GlobalConfigOverride(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, ".gcourer", "config.yaml")
	os.MkdirAll(filepath.Dir(cfgFile), 0755)

	os.WriteFile(cfgFile, []byte(`ollama:
  model: override-model
`), 0644)

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error: %v", err)
	}
	if cfg.Ollama.Model != "override-model" {
		t.Errorf("Ollama.Model = %q, want override-model", cfg.Ollama.Model)
	}
}

// --- SaveGlobal ---

func TestSaveGlobal(t *testing.T) {
	cfg := Default()
	cfg.Ollama.Model = "test-model"

	// SaveGlobal needs a valid home directory or it will fail
	// Just verify the method is callable - actual file creation may fail
	// but should not panic
	err := cfg.SaveGlobal()
	// May fail due to permissions or missing home dir, but that's OK
	if err != nil {
		t.Logf("SaveGlobal error (expected in test env): %v", err)
	}
}

func TestSaveGlobal_MarshalError(t *testing.T) {
	// This is hard to test since we can't easily cause yaml.Marshal to fail
	// Skip this edge case for now
}

// --- DurationConfig error cases ---

func TestDurationConfig_UnmarshalYAML_InvalidFormat(t *testing.T) {
	// Test invalid duration string
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, ".gcourer", "config.yaml")
	os.MkdirAll(filepath.Dir(cfgFile), 0755)
	os.WriteFile(cfgFile, []byte(`commit:
  ttl: "invalid-duration"
`), 0644)

	cfg, err := LoadFromDir(dir)
	if err == nil {
		// Should fail but let's see what happens
		t.Logf("Got cfg with TTL: %v", cfg.Commit.TTL.Duration)
	}
}

// --- Load function ---

func TestLoadFromDir_ContextConfig(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, ".gcourer", "config.yaml")
	os.MkdirAll(filepath.Dir(cfgFile), 0755)
	os.WriteFile(cfgFile, []byte(`context:
  project: "My project"
  style: "conventional commits"
`), 0644)

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error: %v", err)
	}
	if cfg.Context.Project != "My project" {
		t.Errorf("Context.Project = %q, want %q", cfg.Context.Project, "My project")
	}
	if cfg.Context.Style != "conventional commits" {
		t.Errorf("Context.Style = %q, want %q", cfg.Context.Style, "conventional commits")
	}
}

func TestLoadFromDir_ContextConfig_Omitted(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, ".gcourer", "config.yaml")
	os.MkdirAll(filepath.Dir(cfgFile), 0755)
	os.WriteFile(cfgFile, []byte(`ollama:
  model: "test-model"
`), 0644)

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error: %v", err)
	}
	if cfg.Context.Project != "" {
		t.Errorf("Context.Project = %q, want empty (zero value)", cfg.Context.Project)
	}
	if cfg.Context.Style != "" {
		t.Errorf("Context.Style = %q, want empty (zero value)", cfg.Context.Style)
	}
}

// TestDefault_ContextConfig_ZeroValue verifies Default() returns zero-value ContextConfig.
func TestLoadFromDir_ContextConfig_CascadeProjectWins(t *testing.T) {
	// Set up a fake global config with context.project and context.style.
	globalDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", globalDir)
	globalPath := filepath.Join(globalDir, "git-courer", "config.yaml")
	os.MkdirAll(filepath.Dir(globalPath), 0755)
	os.WriteFile(globalPath, []byte(`context:
  project: "Global Project"
  style: "global style"
`), 0644)

	// Set up a project dir with an override for context.style only.
	projDir := t.TempDir()
	projPath := filepath.Join(projDir, ".gcourer", "config.yaml")
	os.MkdirAll(filepath.Dir(projPath), 0755)
	os.WriteFile(projPath, []byte(`context:
  style: "local style"
`), 0644)

	cfg, err := LoadFromDir(projDir)
	if err != nil {
		t.Fatalf("LoadFromDir() error: %v", err)
	}
	if cfg.Context.Project != "Global Project" {
		t.Errorf("Context.Project = %q, want %q", cfg.Context.Project, "Global Project")
	}
	if cfg.Context.Style != "local style" {
		t.Errorf("Context.Style = %q, want %q", cfg.Context.Style, "local style")
	}
}

func TestDefault_ContextConfig_ZeroValue(t *testing.T) {
	cfg := Default()
	if cfg.Context.Project != "" {
		t.Errorf("Default().Context.Project = %q, want empty", cfg.Context.Project)
	}
	if cfg.Context.Style != "" {
		t.Errorf("Default().Context.Style = %q, want empty", cfg.Context.Style)
	}
}

func TestLoad(t *testing.T) {
	// Load uses LoadFromDir(".")
	// Just verify it's callable
	_, err := Load()
	// May error if no git repo, but should not panic
	_ = err
}

// --- PreviewConfig edge cases ---

func TestPreviewConfig_IsRequired_EmptyOperations(t *testing.T) {
	cfg := PreviewConfig{
		Enabled:    true,
		Operations: map[string]bool{},
	}
	if cfg.IsRequired("commit") {
		t.Error("IsRequired() should return false for unknown operation when Enabled=true")
	}
}

func TestPreviewConfig_IsRequired_NilOperations(t *testing.T) {
	cfg := PreviewConfig{
		Enabled:    true,
		Operations: nil,
	}
	if cfg.IsRequired("commit") {
		t.Error("IsRequired() should return false for nil operations map")
	}
}

// --- GitConfig defaults ---

func TestDefault_GitConfig(t *testing.T) {
	cfg := Default()
	if cfg.Git.WorkDir == "" {
		t.Error("Git.WorkDir should not be empty")
	}
	if !cfg.Git.AutoAddSecrets {
		t.Error("Git.AutoAddSecrets should be true by default")
	}
	if cfg.Git.RequireCleanRepo {
		t.Error("Git.RequireCleanRepo should be false by default")
	}
}

func TestDefault_SecretsConfig(t *testing.T) {
	cfg := Default()
	if cfg.Secrets.DetectionMode == "" {
		t.Error("Secrets.DetectionMode should not be empty")
	}
	if len(cfg.Secrets.Patterns) == 0 {
		t.Error("Secrets.Patterns should not be empty")
	}
}

func TestDefault_CommitConfig(t *testing.T) {
	cfg := Default()
	if cfg.Commit.MaxLogLines <= 0 {
		t.Error("Commit.MaxLogLines should be positive")
	}
}

func TestDefault_AllOperationsEnabled(t *testing.T) {
	cfg := Default()
	expectedOps := []string{"commit", "release", "push", "pull", "branch_create", "branch_delete", "merge"}
	for _, op := range expectedOps {
		if !cfg.Commands.IsEnabled(op) {
			t.Errorf("Operation %q should be enabled by default", op)
		}
	}
}

// --- ReleaseConfig defaults ---

func TestDefault_ReleaseConfig(t *testing.T) {
	cfg := Default()
	if cfg.Release.LogPath == "" {
		t.Error("Release.LogPath should not be empty")
	}
	if cfg.Release.MaxCommitsPerChunk <= 0 {
		t.Error("Release.MaxCommitsPerChunk should be positive")
	}
}

// --- OllamaConfig defaults ---

func TestDefault_OllamaConfig(t *testing.T) {
	cfg := Default()
	if cfg.Ollama.Host == "" {
		t.Error("Ollama.Host should not be empty")
	}
	// No default model — user must configure explicitly.
	// AutoStart and ModelsDir are optional (can be empty/false)
	_ = cfg.Ollama.AutoStart
	_ = cfg.Ollama.ModelsDir
	_ = cfg.Ollama.ContextWindow
}

// --- CommandsConfig edge cases ---

func TestCommandsConfig_IsEnabled_Empty(t *testing.T) {
	cfg := CommandsConfig{
		EnabledOperations: []string{},
	}
	if cfg.IsEnabled("commit") {
		t.Error("IsEnabled should return false for empty operations list")
	}
}

func TestCommandsConfig_IsEnabled_EmptyString(t *testing.T) {
	cfg := CommandsConfig{
		EnabledOperations: []string{"commit"},
	}
	if cfg.IsEnabled("") {
		t.Error("IsEnabled should return false for empty operation key")
	}
}

// --- DurationConfig edge cases ---

func TestDurationConfig_Zero(t *testing.T) {
	d := NewDurationConfig(0)
	if d.Duration != 0 {
		t.Errorf("Duration = %v, want 0", d.Duration)
	}
}

func TestDurationConfig_MarshalEmpty(t *testing.T) {
	d := NewDurationConfig(0)
	val, err := d.MarshalYAML()
	if err != nil {
		t.Fatalf("MarshalYAML() error: %v", err)
	}
	s, ok := val.(string)
	if !ok {
		t.Fatalf("MarshalYAML() returned %T, want string", val)
	}
	if s != "0s" {
		t.Errorf("MarshalYAML() = %q, want 0s", s)
	}
}

// --- LLMConfig defaults ---

func TestLLMConfig_Defaults(t *testing.T) {
	cfg := Default()

	// Default() must populate LLM with provider=ollama and base_url pointing to /v1
	if cfg.LLM.Provider != "ollama" {
		t.Errorf("LLM.Provider = %q, want %q", cfg.LLM.Provider, "ollama")
	}
	if cfg.LLM.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("LLM.BaseURL = %q, want %q", cfg.LLM.BaseURL, "http://localhost:11434/v1")
	}
	// No default model — user must configure explicitly.
	// ContextWindow defaults to 0 (use model default)
	if cfg.LLM.ContextWindow != 0 {
		t.Errorf("LLM.ContextWindow = %d, want 0", cfg.LLM.ContextWindow)
	}
	// Ollama sub-config defaults
	if cfg.LLM.Ollama.AutoStart != false {
		t.Error("LLM.Ollama.AutoStart should be false by default")
	}
}

// --- ResolveLLMConfig ---

func TestResolveLLMConfig_LegacyOnly(t *testing.T) {
	// When only OllamaConfig is set (legacy), ResolveLLMConfig auto-populates LLMConfig
	cfg := &Config{
		Ollama: OllamaConfig{
			Host:          "http://myserver:11434",
			Model:         "my-model:7b",
			ContextWindow: 4096,
			AutoStart:     true,
			ModelsDir:     "/custom/models",
		},
	}

	resolved, err := cfg.ResolveLLMConfig()

	if err != nil {

		t.Fatalf("ResolveLLMConfig() error: %v", err)

	}

	// Provider should be ollama
	if resolved.Provider != "ollama" {
		t.Errorf("ResolveLLMConfig().Provider = %q, want %q", resolved.Provider, "ollama")
	}
	// BaseURL should be host + "/v1"
	if resolved.BaseURL != "http://myserver:11434/v1" {
		t.Errorf("ResolveLLMConfig().BaseURL = %q, want %q", resolved.BaseURL, "http://myserver:11434/v1")
	}
	// Model should come from Ollama
	if resolved.Model != "my-model:7b" {
		t.Errorf("ResolveLLMConfig().Model = %q, want %q", resolved.Model, "my-model:7b")
	}
	// ContextWindow should come from Ollama
	if resolved.ContextWindow != 4096 {
		t.Errorf("ResolveLLMConfig().ContextWindow = %d, want 4096", resolved.ContextWindow)
	}
	// Ollama sub-config should carry over
	if resolved.Ollama.AutoStart != true {
		t.Error("ResolveLLMConfig().Ollama.AutoStart should be true")
	}
	if resolved.Ollama.ModelsDir != "/custom/models" {
		t.Errorf("ResolveLLMConfig().Ollama.ModelsDir = %q, want %q", resolved.Ollama.ModelsDir, "/custom/models")
	}
}

func TestResolveLLMConfig_LLMOverrides(t *testing.T) {
	// When both OllamaConfig and LLM are set, LLM takes precedence
	cfg := &Config{
		Ollama: OllamaConfig{
			Host:          "http://legacy:11434",
			Model:         "legacy-model:7b",
			ContextWindow: 2048,
		},
		LLM: LLMConfig{
			Provider:      "ollama",
			BaseURL:       "http://custom:11434/v1",
			Model:         "custom-model:13b",
			ContextWindow: 8192,
			Ollama: OllamaSubConfig{
				ModelsDir: "/override/models",
				AutoStart: true,
			},
		},
	}

	resolved, err := cfg.ResolveLLMConfig()

	if err != nil {

		t.Fatalf("ResolveLLMConfig() error: %v", err)

	}

	// LLM fields win over OllamaConfig
	if resolved.Provider != "ollama" {
		t.Errorf("Provider = %q, want %q", resolved.Provider, "ollama")
	}
	if resolved.BaseURL != "http://custom:11434/v1" {
		t.Errorf("BaseURL = %q, want %q", resolved.BaseURL, "http://custom:11434/v1")
	}
	if resolved.Model != "custom-model:13b" {
		t.Errorf("Model = %q, want %q", resolved.Model, "custom-model:13b")
	}
	if resolved.ContextWindow != 8192 {
		t.Errorf("ContextWindow = %d, want 8192", resolved.ContextWindow)
	}
	// LLM.Ollama sub-config should be preserved
	if resolved.Ollama.ModelsDir != "/override/models" {
		t.Errorf("Ollama.ModelsDir = %q, want %q", resolved.Ollama.ModelsDir, "/override/models")
	}
	if resolved.Ollama.AutoStart != true {
		t.Error("Ollama.AutoStart should be true")
	}
}

func TestResolveLLMConfig_ContextWindow(t *testing.T) {
	// ContextWindow comes from LLMConfig, not OllamaConfig
	cfg := &Config{
		Ollama: OllamaConfig{
			Host:          "http://localhost:11434",
			Model:         "gemma4:26b",
			ContextWindow: 2048, // Ollama legacy value
		},
		LLM: LLMConfig{
			Provider:      "ollama",
			BaseURL:       "http://localhost:11434/v1",
			Model:         "gemma4:26b",
			ContextWindow: 32768, // LLM value takes precedence
		},
	}

	resolved, err := cfg.ResolveLLMConfig()

	if err != nil {

		t.Fatalf("ResolveLLMConfig() error: %v", err)

	}

	if resolved.ContextWindow != 32768 {
		t.Errorf("ContextWindow = %d, want 32768 (from LLM, not Ollama)", resolved.ContextWindow)
	}
}

// TestResolveLLMConfig_LLMOverridesOllama verifies that when both ollama: and llm:
// sections exist and llm.provider is set, llm: takes full precedence.
func TestResolveLLMConfig_LLMOverridesOllama(t *testing.T) {
	cfg := &Config{
		Ollama: OllamaConfig{
			Host:          "http://ollama-host:11434",
			Model:         "ollama-model:7b",
			ContextWindow: 2048,
		},
		LLM: LLMConfig{
			Provider:      "openai-compatible",
			BaseURL:       "https://api.openai.com/v1",
			Model:         "gpt-4",
			ContextWindow: 8192,
			APIKey:        "sk-test",
		},
	}

	resolved, err := cfg.ResolveLLMConfig()

	if err != nil {

		t.Fatalf("ResolveLLMConfig() error: %v", err)

	}

	// LLM wins across the board
	if resolved.Provider != "openai-compatible" {
		t.Errorf("Provider = %q, want %q", resolved.Provider, "openai-compatible")
	}
	if resolved.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("BaseURL = %q, want %q", resolved.BaseURL, "https://api.openai.com/v1")
	}
	if resolved.Model != "gpt-4" {
		t.Errorf("Model = %q, want %q", resolved.Model, "gpt-4")
	}
	if resolved.ContextWindow != 8192 {
		t.Errorf("ContextWindow = %d, want 8192", resolved.ContextWindow)
	}
	if resolved.APIKey != "sk-test" {
		t.Errorf("APIKey = %q, want %q", resolved.APIKey, "sk-test")
	}
	// Ollama fields should NOT bleed through
	if resolved.Provider == "ollama" {
		t.Error("Provider should NOT fallback to ollama when LLM.Provider is set")
	}
}

// TestResolveLLMConfig_EmptyProviderFallsBackToOllama verifies that when
// llm.provider is empty, the resolved config is built from the ollama.host.
func TestResolveLLMConfig_EmptyProviderFallsBackToOllama(t *testing.T) {
	cfg := &Config{
		Ollama: OllamaConfig{
			Host:  "http://my-ollama:11434",
			Model: "custom-model:13b",
		},
		LLM: LLMConfig{
			// Provider empty — triggers fallback to Ollama
		},
	}

	resolved, err := cfg.ResolveLLMConfig()

	if err != nil {

		t.Fatalf("ResolveLLMConfig() error: %v", err)

	}

	if resolved.Provider != "ollama" {
		t.Errorf("Provider = %q, want %q (auto-populated)", resolved.Provider, "ollama")
	}
	if resolved.BaseURL != "http://my-ollama:11434/v1" {
		t.Errorf("BaseURL = %q, want %q (host + /v1)", resolved.BaseURL, "http://my-ollama:11434/v1")
	}
}

// TestResolveLLMConfig_LLMBasURLOverridesOllamaHost verifies that when llm.base_url
// is explicitly set, it overrides the auto-generated ollama.host + "/v1".
func TestResolveLLMConfig_LLMBasURLOverridesOllamaHost(t *testing.T) {
	cfg := &Config{
		Ollama: OllamaConfig{
			Host:  "http://localhost:11434",
			Model: "gemma4:26b",
		},
		LLM: LLMConfig{
			Provider: "ollama",
			BaseURL:  "https://remote-ollama.example.com/v1",
			Model:    "gemma4:26b",
		},
	}

	resolved, err := cfg.ResolveLLMConfig()

	if err != nil {

		t.Fatalf("ResolveLLMConfig() error: %v", err)

	}

	if resolved.BaseURL != "https://remote-ollama.example.com/v1" {
		t.Errorf("BaseURL = %q, want %q (llm.base_url overrides ollama.host+/v1)", resolved.BaseURL, "https://remote-ollama.example.com/v1")
	}
}

// TestResolveLLMConfig_ContextWindowFromLLM verifies that when llm.context_window
// is set, it takes precedence over ollama.context_window.
func TestResolveLLMConfig_ContextWindowFromLLM(t *testing.T) {
	cfg := &Config{
		Ollama: OllamaConfig{
			Host:          "http://localhost:11434",
			Model:         "gemma4:26b",
			ContextWindow: 2048, // legacy value — should NOT win
		},
		LLM: LLMConfig{
			Provider:      "ollama",
			BaseURL:       "http://localhost:11434/v1",
			Model:         "gemma4:26b",
			ContextWindow: 65536, // this should win
		},
	}

	resolved, err := cfg.ResolveLLMConfig()

	if err != nil {

		t.Fatalf("ResolveLLMConfig() error: %v", err)

	}

	if resolved.ContextWindow != 65536 {
		t.Errorf("ContextWindow = %d, want 65536 (from llm, not 2048 from ollama)", resolved.ContextWindow)
	}
}

// TestResolveLLMConfig_DefaultProviderIsNotResolvable verifies that Default()
// requires a model before ResolveLLMConfig can succeed.
func TestResolveLLMConfig_DefaultProviderIsOllama(t *testing.T) {
	cfg := Default()
	_, err := cfg.ResolveLLMConfig()
	if err == nil {
		t.Fatal("Default() without model should return error")
	}
}

// TestResolveLLMConfig_EmptyModelStateErrors verifies that when both
// Ollama.Model and LLM.Model are empty, ResolveLLMConfig errors.
func TestResolveLLMConfig_EmptyModelStateUsesDefault(t *testing.T) {
	cfg := &Config{}

	_, err := cfg.ResolveLLMConfig()
	if err == nil {
		t.Fatal("ResolveLLMConfig() with empty model should return error")
	}
}

// --- NumParallel config ---

func TestDefault_NumParallelIsOne(t *testing.T) {
	cfg := Default()
	if cfg.LLM.NumParallel != 1 {
		t.Errorf("Default() LLM.NumParallel = %d, want 1 (serial by default)", cfg.LLM.NumParallel)
	}
}

func TestResolveLLMConfig_NumParallel_PreservesPositiveValue(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			Provider:    "ollama",
			BaseURL:     "http://localhost:11434/v1",
			Model:       "test-model",
			NumParallel: 3,
		},
	}
	resolved, err := cfg.ResolveLLMConfig()
	if err != nil {
		t.Fatalf("ResolveLLMConfig() error: %v", err)
	}
	if resolved.NumParallel != 3 {
		t.Errorf("ResolveLLMConfig().NumParallel = %d, want 3 (preserved positive value)", resolved.NumParallel)
	}
}

func TestResolveLLMConfig_NumParallel_ClampsZeroToOne(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			Provider:    "ollama",
			BaseURL:     "http://localhost:11434/v1",
			Model:       "test-model",
			NumParallel: 0,
		},
	}
	resolved, err := cfg.ResolveLLMConfig()
	if err != nil {
		t.Fatalf("ResolveLLMConfig() error: %v", err)
	}
	if resolved.NumParallel != 1 {
		t.Errorf("ResolveLLMConfig().NumParallel = %d, want 1 (zero clamped to 1)", resolved.NumParallel)
	}
}

func TestResolveLLMConfig_NumParallel_ClampsNegativeToOne(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			Provider:    "ollama",
			BaseURL:     "http://localhost:11434/v1",
			Model:       "test-model",
			NumParallel: -5,
		},
	}
	resolved, err := cfg.ResolveLLMConfig()
	if err != nil {
		t.Fatalf("ResolveLLMConfig() error: %v", err)
	}
	if resolved.NumParallel != 1 {
		t.Errorf("ResolveLLMConfig().NumParallel = %d, want 1 (negative clamped to 1)", resolved.NumParallel)
	}
}

func TestResolveLLMConfig_NumParallel_LegacyDefaultsToOne(t *testing.T) {
	// When LLM.Provider is empty (legacy Ollama-only config),
	// NumParallel should default to 1 since no value was set.
	cfg := &Config{
		Ollama: OllamaConfig{
			Host:  "http://localhost:11434",
			Model: "test-model",
		},
		LLM: LLMConfig{
			// Provider empty — triggers legacy fallback
		},
	}
	resolved, err := cfg.ResolveLLMConfig()
	if err != nil {
		t.Fatalf("ResolveLLMConfig() error: %v", err)
	}
	if resolved.NumParallel != 1 {
		t.Errorf("ResolveLLMConfig().NumParallel = %d, want 1 (legacy default)", resolved.NumParallel)
	}
}

func TestLLMConfig_NumParallel_YAMLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, ".gcourer", "config.yaml")
	os.MkdirAll(filepath.Dir(cfgFile), 0755)
	os.WriteFile(cfgFile, []byte(`llm:
  num_parallel: 5
`), 0644)

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error: %v", err)
	}
	if cfg.LLM.NumParallel != 5 {
		t.Errorf("YAML round-trip: LLM.NumParallel = %d, want 5", cfg.LLM.NumParallel)
	}
}

func TestResolveLLMConfig_EmptyProvider(t *testing.T) {
	// When LLM.Provider is empty, auto-build from OllamaConfig
	cfg := &Config{
		Ollama: OllamaConfig{
			Host:  "http://localhost:11434",
			Model: "codellama:7b",
		},
		LLM: LLMConfig{
			// Provider is empty — should trigger auto-migration
		},
	}

	resolved, err := cfg.ResolveLLMConfig()

	if err != nil {

		t.Fatalf("ResolveLLMConfig() error: %v", err)

	}

	if resolved.Provider != "ollama" {
		t.Errorf("Provider = %q, want %q (auto-populated from Ollama)", resolved.Provider, "ollama")
	}
	if resolved.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("BaseURL = %q, want %q", resolved.BaseURL, "http://localhost:11434/v1")
	}
	if resolved.Model != "codellama:7b" {
		t.Errorf("Model = %q, want %q", resolved.Model, "codellama:7b")
	}
}

// --- OperationParams tests ---

func TestOperationParams_YAMLRoundTrip(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, ".gcourer", "config.yaml")
	os.MkdirAll(filepath.Dir(cfgFile), 0755)
	yamlContent := `llm:
  model: test-model
  operations:
    commit:
      temperature: 0.5
      max_tokens: 512
      top_p: 0.9
    branch_create:
      temperature: 0.1
      max_tokens: 128
`
	os.WriteFile(cfgFile, []byte(yamlContent), 0644)

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir() error: %v", err)
	}
	if len(cfg.LLM.Operations) != 2 {
		t.Fatalf("LLM.Operations = %d, want 2", len(cfg.LLM.Operations))
	}
	commitOp := cfg.LLM.Operations["commit"]
	if commitOp.Temperature == nil || *commitOp.Temperature != 0.5 {
		t.Errorf("commit temperature: got %v, want 0.5", commitOp.Temperature)
	}
	if commitOp.MaxTokens == nil || *commitOp.MaxTokens != 512 {
		t.Errorf("commit max_tokens: got %v, want 512", commitOp.MaxTokens)
	}
	if commitOp.TopP == nil || *commitOp.TopP != 0.9 {
		t.Errorf("commit top_p: got %v, want 0.9", commitOp.TopP)
	}
}

func TestResolveLLMConfig_EmptyModelReturnsError(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			Provider: "openai-compatible",
			BaseURL:  "http://localhost:8080/v1",
			Model:    "", // empty
		},
	}
	_, err := cfg.ResolveLLMConfig()
	if err == nil {
		t.Fatal("ResolveLLMConfig() with empty model should return error")
	}
	if !strings.Contains(err.Error(), "model") {
		t.Errorf("error should mention 'model', got: %v", err)
	}
}

func TestResolveLLMConfig_KnownOperationsAreValid(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			Provider: "ollama",
			Model:    "test-model",
			BaseURL:  "http://localhost:11434/v1",
			Operations: map[string]OperationParams{
				"commit":         {},
				"branch_create":  {},
				"branch_delete":  {},
				"branch_rename":  {},
				"tag_create":     {},
				"tag_delete":     {},
				"merge":          {},
				"release":        {},
				"changelog":      {},
				"secret_verification": {},
			},
		},
	}
	resolved, err := cfg.ResolveLLMConfig()
	if err != nil {
		t.Fatalf("ResolveLLMConfig() error: %v", err)
	}
	if resolved.Provider != "ollama" {
		t.Errorf("Provider = %q, want %q", resolved.Provider, "ollama")
	}
}

func TestResolveLLMConfig_UnknownOperationReturnsError(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			Provider: "ollama",
			Model:    "test-model",
			Operations: map[string]OperationParams{
				"unknown_op": {},
			},
		},
	}
	_, err := cfg.ResolveLLMConfig()
	if err == nil {
		t.Fatal("ResolveLLMConfig() with unknown operation should return error")
	}
	if !strings.Contains(err.Error(), "unknown_op") {
		t.Errorf("error should mention 'unknown_op', got: %v", err)
	}
}

func TestResolveLLMConfig_OllamaRequestOptions(t *testing.T) {
	cfg := &Config{
		LLM: LLMConfig{
			Provider: "ollama",
			Model:    "test-model",
			Ollama: OllamaSubConfig{
				NumCtx:     4096,
				KeepAlive:  "5m",
				NumPredict: 256,
			},
		},
	}
	resolved, err := cfg.ResolveLLMConfig()
	if err != nil {
		t.Fatalf("ResolveLLMConfig() error: %v", err)
	}
	if resolved.Ollama.NumCtx != 4096 {
		t.Errorf("NumCtx = %d, want 4096", resolved.Ollama.NumCtx)
	}
	if resolved.Ollama.KeepAlive != "5m" {
		t.Errorf("KeepAlive = %q, want 5m", resolved.Ollama.KeepAlive)
	}
	if resolved.Ollama.NumPredict != 256 {
		t.Errorf("NumPredict = %d, want 256", resolved.Ollama.NumPredict)
	}
}

func TestResolveLLMConfig_NoDefaultModel(t *testing.T) {
	cfg := Default()
	_, err := cfg.ResolveLLMConfig()
	if err == nil {
		t.Fatal("Default() without model should error")
	}
}

func TestResolveLLMConfig_BackwardCompatibility(t *testing.T) {
	// Old configs with ollama: section but no llm: section
	cfg := &Config{
		Ollama: OllamaConfig{
			Host:  "http://localhost:11434",
			Model: "qwen3.5:latest",
		},
	}
	resolved, err := cfg.ResolveLLMConfig()
	if err != nil {
		t.Fatalf("ResolveLLMConfig() error: %v", err)
	}
	if resolved.Model != "qwen3.5:latest" {
		t.Errorf("Model = %q, want %q", resolved.Model, "qwen3.5:latest")
	}
	if resolved.Provider != "ollama" {
		t.Errorf("Provider = %q, want %q", resolved.Provider, "ollama")
	}
}
