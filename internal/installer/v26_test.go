// Package installer_test verifies GIT_COURER.md creation, config backup, and
// the doctor diagnostics added in v2.6.0.
package installer

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConfigureMCP_InjectsPromptBlock verifies ConfigureMCP injects the prompt block
// into the instructions file alongside the MCP config, and deletes old physical GIT_COURER.md.
func TestConfigureMCP_InjectsPromptBlock(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	client := &MCPClient{
		Name:     "claude-code",
		Filename: "settings.json",
		RootKey:  "mcpServers",
		ConfigFn: func(binPath string) map[string]interface{} {
			return map[string]interface{}{"command": binPath, "args": []string{"mcp"}}
		},
		Paths:  []string{configPath},
		Detect: func() bool { return true },
	}

	// Pre-create old physical GIT_COURER.md to test cleanup.
	oldRulesPath := filepath.Join(dir, gitCourerMdFilename)
	_ = os.WriteFile(oldRulesPath, []byte("old rules"), 0644)

	if err := ConfigureMCP(client, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("ConfigureMCP: %v", err)
	}

	// Old physical rules file must be deleted.
	if _, err := os.Stat(oldRulesPath); !os.IsNotExist(err) {
		t.Error("old physical GIT_COURER.md was not deleted")
	}

	// Prompt block must be injected in ~/.claude/CLAUDE.md.
	instrPath := client.GetInstructionsPath()
	data, err := os.ReadFile(instrPath)
	if err != nil {
		t.Fatalf("instructions file not created: %v", err)
	}
	content := string(data)
	if !contains(content, promptBlockStartDelimiter) || !contains(content, promptBlockEndDelimiter) {
		t.Fatal("prompt block delimiters missing")
	}
	// Golden rules must be present inside block.
	for _, want := range []string{"status", "diff", "pr-review"} {
		if !contains(content, want) {
			t.Errorf("prompt block missing %q in:\n%s", want, content)
		}
	}
}

// TestConfigureMCP_OverwritesPromptBlock verifies prompt block is ALWAYS
// overwritten/updated with the current golden rules on every setup run,
// while preserving other surrounding custom rules.
func TestConfigureMCP_OverwritesPromptBlock(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	client := &MCPClient{
		Name:     "claude-code",
		Filename: "settings.json",
		RootKey:  "mcpServers",
		ConfigFn: func(binPath string) map[string]interface{} {
			return map[string]interface{}{"command": binPath}
		},
		Paths:  []string{configPath},
		Detect: func() bool { return true },
	}

	// Pre-create custom instructions file with user content and old block.
	instrPath := client.GetInstructionsPath()
	if err := os.MkdirAll(filepath.Dir(instrPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	originalContent := "Before content\n" + promptBlockStartDelimiter + "\nOld rules\n" + promptBlockEndDelimiter + "\nAfter content"
	if err := os.WriteFile(instrPath, []byte(originalContent), 0644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	if err := ConfigureMCP(client, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("ConfigureMCP: %v", err)
	}

	data, _ := os.ReadFile(instrPath)
	content := string(data)
	if !contains(content, "Before content") {
		t.Error("Before content was not preserved")
	}
	if !contains(content, "After content") {
		t.Error("After content was not preserved")
	}
	expectedBlock := promptBlockStartDelimiter + "\n" + gitCourerMdContent + "\n" + promptBlockEndDelimiter
	if !contains(content, expectedBlock) {
		t.Errorf("prompt block was not updated correctly:\ngot: %q\nwant: %q", content, expectedBlock)
	}
}

// TestConfigureMCP_CreatesBackup verifies a .bak backup of the existing config
// is created before the config is modified.
func TestConfigureMCP_CreatesBackup(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	original := `{"mcpServers":{"other":{"command":"other"}}}`
	if err := os.WriteFile(configPath, []byte(original), 0644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	client := &MCPClient{
		Name:     "claude-code",
		Filename: "settings.json",
		RootKey:  "mcpServers",
		ConfigFn: func(binPath string) map[string]interface{} {
			return map[string]interface{}{"command": binPath}
		},
		Paths:  []string{configPath},
		Detect: func() bool { return true },
	}

	if err := ConfigureMCP(client, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("ConfigureMCP: %v", err)
	}

	backup, err := os.ReadFile(configPath + ".bak")
	if err != nil {
		t.Fatalf("backup file not created: %v", err)
	}
	if string(backup) != original {
		t.Errorf("backup content does not match original:\ngot:  %q\nwant: %q", backup, original)
	}
}

// TestConfigureMCP_NoBackupWhenNoExistingConfig verifies no backup is created
// when the config file does not exist yet (nothing to back up).
func TestConfigureMCP_NoBackupWhenNoExistingConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	client := &MCPClient{
		Name:     "claude-code",
		Filename: "settings.json",
		RootKey:  "mcpServers",
		ConfigFn: func(binPath string) map[string]interface{} {
			return map[string]interface{}{"command": binPath}
		},
		Paths:  []string{configPath},
		Detect: func() bool { return true },
	}

	if err := ConfigureMCP(client, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("ConfigureMCP: %v", err)
	}

	if _, err := os.Stat(configPath + ".bak"); err == nil {
		t.Error("backup file should NOT exist when there was no original config")
	}
}

// TestRunDoctor_ReportsDiagnostics verifies RunDoctor returns diagnostics for
// each detected client.
func TestRunDoctor_ReportsDiagnostics(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	// Mock getMCPClients to return a single detected client.
	oldGetClients := getMCPClients
	defer func() { getMCPClients = oldGetClients }()
	getMCPClients = func() []*MCPClient {
		return []*MCPClient{
			{
				Name:     "test-client",
				Filename: "settings.json",
				RootKey:  "mcpServers",
				ConfigFn: func(binPath string) map[string]interface{} {
					return map[string]interface{}{"command": binPath}
				},
				Paths:  []string{configPath},
				Detect: func() bool { return true },
			},
		}
	}

	// Run ConfigureMCP first so the config and GIT_COURER.md exist.
	if err := ConfigureMCP(getMCPClients()[0], "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("ConfigureMCP: %v", err)
	}

	diagnostics := RunDoctor()
	if len(diagnostics) == 0 {
		t.Fatal("RunDoctor returned no diagnostics")
	}

	d := diagnostics[0]
	if d.ClientName != "test-client" {
		t.Errorf("ClientName: got %q, want %q", d.ClientName, "test-client")
	}
	if !d.MCPConfigured {
		t.Error("MCPConfigured: got false, want true (config was written)")
	}
	if !d.PromptBlockInjected {
		t.Error("PromptBlockInjected: got false, want true (prompt block was written)")
	}
	if d.HooksStatus != "not_installed" {
		t.Errorf("HooksStatus: got %q, want %q", d.HooksStatus, "not_installed")
	}
}

// TestRunDoctor_NoClients verifies RunDoctor returns empty diagnostics when
// no clients are detected.
func TestRunDoctor_NoClients(t *testing.T) {
	oldGetClients := getMCPClients
	defer func() { getMCPClients = oldGetClients }()
	getMCPClients = func() []*MCPClient { return nil }

	diagnostics := RunDoctor()
	if len(diagnostics) != 0 {
		t.Errorf("RunDoctor returned %d diagnostics, want 0", len(diagnostics))
	}
}

// TestRestoreBackup_RestoresConfig verifies restoreBackup copies the .bak
// file over the config and removes the .bak. This is the unit test for the
// backup-restore logic added in RunUninstall; RunUninstall itself is tested
// as an integration in sdd-verify because it touches the real binary path
// and global config.
func TestRestoreBackup_RestoresConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	original := `{"mcpServers":{"other":{"command":"other"}}}`
	if err := os.WriteFile(configPath, []byte(original), 0644); err != nil {
		t.Fatalf("write original: %v", err)
	}
	if err := os.WriteFile(configPath+".bak", []byte(original), 0644); err != nil {
		t.Fatalf("write backup: %v", err)
	}
	modified := `{"mcpServers":{"git-courer":{"command":"git-courer"}}}`
	if err := os.WriteFile(configPath, []byte(modified), 0644); err != nil {
		t.Fatalf("write modified: %v", err)
	}

	restoreBackup(configPath)

	restored, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config file missing after restore: %v", err)
	}
	if string(restored) != original {
		t.Errorf("config was not restored from backup:\ngot:  %q\nwant: %q", restored, original)
	}

	if _, err := os.Stat(configPath + ".bak"); err == nil {
		t.Error("backup file should have been removed after restore")
	}
}

// TestRestoreBackup_NoBackupIsNoop verifies restoreBackup is a no-op when no
// .bak file exists.
func TestRestoreBackup_NoBackupIsNoop(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")
	current := `{"mcpServers":{"git-courer":{"command":"x"}}}`
	if err := os.WriteFile(configPath, []byte(current), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	restoreBackup(configPath) // no .bak — should do nothing

	data, _ := os.ReadFile(configPath)
	if string(data) != current {
		t.Errorf("config changed when no backup existed:\ngot:  %q\nwant: %q", data, current)
	}
}

// TestConfigureMCP_OpenCodePolicyEarlyReturnPath verifies that when
// opencode.json already contains git-courer (early-return path in
// ConfigureMCP), the policy entries (permission.bash "git *": "ask" and
// GIT_COURER.md in instructions) are still applied. This proves the wiring
// calls configureOpenCodePolicy in the already-configured branch.
func TestConfigureMCP_OpenCodePolicyEarlyReturnPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	// Pre-configure with git-courer MCP entry so containsGitCourer is true.
	existing := `{"mcp":{"git-courer":{"command":"git-courer","args":["mcp"]}}}`
	if err := os.WriteFile(configPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	client := &MCPClient{
		Name:     "opencode",
		Filename: "opencode.json",
		RootKey:  "mcp",
		ConfigFn: func(binPath string) map[string]interface{} {
			return map[string]interface{}{"command": binPath, "args": []string{"mcp"}}
		},
		Paths:  []string{configPath},
		Detect: func() bool { return true },
	}

	if err := ConfigureMCP(client, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("ConfigureMCP: %v", err)
	}

	cfg := readOpenCodeConfig(t, configPath)
	assertBashRule(t, cfg, "git *", "ask")
	assertInstructionsContains(t, cfg, filepath.Join(filepath.Dir(configPath), "AGENTS.md"))
}

// TestConfigureMCP_OpenCodePolicyNormalPath verifies that when opencode.json
// is fresh (no git-courer entry), ConfigureMCP writes the MCP entry AND
// applies the policy entries in the normal path.
func TestConfigureMCP_OpenCodePolicyNormalPath(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")

	client := &MCPClient{
		Name:     "opencode",
		Filename: "opencode.json",
		RootKey:  "mcp",
		ConfigFn: func(binPath string) map[string]interface{} {
			return map[string]interface{}{"command": binPath, "args": []string{"mcp"}}
		},
		Paths:  []string{configPath},
		Detect: func() bool { return true },
	}

	if err := ConfigureMCP(client, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("ConfigureMCP: %v", err)
	}

	cfg := readOpenCodeConfig(t, configPath)
	// MCP entry present.
	mcp, ok := cfg["mcp"].(map[string]interface{})
	if !ok {
		t.Fatal("mcp key missing or not an object")
	}
	if _, ok := mcp["git-courer"]; !ok {
		t.Error("git-courer MCP entry missing in normal path")
	}
	// Policy present.
	assertBashRule(t, cfg, "git *", "ask")
	assertInstructionsContains(t, cfg, filepath.Join(filepath.Dir(configPath), "AGENTS.md"))
}

// TestConfigureMCP_NonOpenCodeClientNoPolicy verifies that a non-opencode
// client (e.g. claude-code) does NOT receive the opencode policy entries —
// permission.bash and instructions must be absent.
func TestConfigureMCP_NonOpenCodeClientNoPolicy(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	client := &MCPClient{
		Name:     "claude-code",
		Filename: "settings.json",
		RootKey:  "mcpServers",
		ConfigFn: func(binPath string) map[string]interface{} {
			return map[string]interface{}{"command": binPath, "args": []string{"mcp"}}
		},
		Paths:  []string{configPath},
		Detect: func() bool { return true },
	}

	if err := ConfigureMCP(client, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("ConfigureMCP: %v", err)
	}

	cfg := readOpenCodeConfig(t, configPath)
	// permission.bash must NOT contain "git *" for non-opencode clients.
	if perm, ok := cfg["permission"].(map[string]interface{}); ok {
		if bash, ok := perm["bash"].(map[string]interface{}); ok {
			if _, exists := bash["git *"]; exists {
				t.Error("non-opencode client received opencode policy permission.bash[\"git *\"]")
			}
		}
	}
	// instructions must NOT contain the AGENTS.md path (for this config dir).
	assertInstructionsAbsent(t, cfg, filepath.Join(filepath.Dir(configPath), "AGENTS.md"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && indexOf(s, substr) >= 0
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}