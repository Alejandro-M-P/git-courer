package config

import (
	"os"
	"path/filepath"
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
	if cfg.Ollama.Model == "" {
		t.Error("Ollama.Model should not be empty")
	}
}

func TestDefault_CommitPaths(t *testing.T) {
	cfg := Default()
	if cfg.Commit.LogPath == "" {
		t.Error("Commit.LogPath should not be empty")
	}
	if cfg.Commit.PlanFile == "" {
		t.Error("Commit.PlanFile should not be empty")
	}
	if cfg.Commit.BlockerFile == "" {
		t.Error("Commit.BlockerFile should not be empty")
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
	if cfg.Release.ChangelogPath == "" {
		t.Error("Release.ChangelogPath should not be empty")
	}
	if cfg.Release.IntentPath == "" {
		t.Error("Release.IntentPath should not be empty")
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

func TestDefault_MCPVersion(t *testing.T) {
	cfg := Default()
	if cfg.MCP.Version == "" {
		t.Error("MCP.Version should not be empty")
	}
	if cfg.MCP.Name == "" {
		t.Error("MCP.Name should not be empty")
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
	def := Default()
	if cfg.MCP.Version != def.MCP.Version {
		t.Errorf("MCP.Version = %q, want %q", cfg.MCP.Version, def.MCP.Version)
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
