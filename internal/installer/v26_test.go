// Package installer_test verifies GIT_COURER.md creation, config backup, and
// the doctor diagnostics added in v2.6.0.
package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfigureMCP_CreatesGitCourerMd verifies ConfigureMCP creates a
// GIT_COURER.md golden-rules file in the config directory alongside the
// MCP config.
func TestConfigureMCP_CreatesGitCourerMd(t *testing.T) {
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

	rulesPath := filepath.Join(dir, gitCourerMdFilename)
	data, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("GIT_COURER.md not created: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("GIT_COURER.md is empty")
	}
	// Golden rules must be present.
	content := string(data)
	for _, want := range []string{"status", "diff", "pr-review"} {
		if !contains(content, want) {
			t.Errorf("GIT_COURER.md missing %q in:\n%s", want, content)
		}
	}
}

// TestConfigureMCP_DoesNotOverwriteGitCourerMd verifies GIT_COURER.md creation
// is idempotent — an existing file is NOT overwritten.
func TestConfigureMCP_DoesNotOverwriteGitCourerMd(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	// Pre-create a custom GIT_COURER.md with user content.
	custom := "# custom rules\n"
	rulesPath := filepath.Join(dir, gitCourerMdFilename)
	if err := os.WriteFile(rulesPath, []byte(custom), 0644); err != nil {
		t.Fatalf("write custom rules: %v", err)
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

	data, _ := os.ReadFile(rulesPath)
	if string(data) != custom {
		t.Errorf("existing GIT_COURER.md was overwritten:\ngot:  %q\nwant: %q", data, custom)
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
	if !d.GitCourerMdPresent {
		t.Error("GitCourerMdPresent: got false, want true (GIT_COURER.md was written)")
	}
	if d.HooksStatus != statusNotImplemented {
		t.Errorf("HooksStatus: got %q, want %q", d.HooksStatus, statusNotImplemented)
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

// TestHooksConfig_Fields verifies the HooksConfig struct exposes Path and
// Format fields so MCPClient can carry a hooks descriptor per client.
func TestHooksConfig_Fields(t *testing.T) {
	hc := HooksConfig{Path: "/home/u/.codex/hooks.json", Format: "json"}
	if hc.Path != "/home/u/.codex/hooks.json" {
		t.Errorf("Path: got %q", hc.Path)
	}
	if hc.Format != "json" {
		t.Errorf("Format: got %q", hc.Format)
	}
}

// TestMCPClients_CodexHasHooksConfig verifies the Codex entry in MCPClients
// carries a HooksConfig pointing at ~/.codex/hooks.json with format "json".
func TestMCPClients_CodexHasHooksConfig(t *testing.T) {
	var codex *MCPClient
	for _, c := range MCPClients() {
		if c.Name == "codex" {
			codex = c
			break
		}
	}
	if codex == nil {
		t.Fatal("codex client not found in MCPClients()")
	}
	if codex.HooksConfig == nil {
		t.Fatal("codex HooksConfig is nil — expected non-nil")
	}
	if !strings.HasSuffix(codex.HooksConfig.Path, ".codex/hooks.json") {
		t.Errorf("codex HooksConfig.Path: got %q, want suffix .codex/hooks.json", codex.HooksConfig.Path)
	}
	if codex.HooksConfig.Format != "json" {
		t.Errorf("codex HooksConfig.Format: got %q, want %q", codex.HooksConfig.Format, "json")
	}
}

// TestInstallHook_CreatesHooksJson verifies installHook writes a hooks.json
// with a PreToolUse Bash matcher pointing at git-courer hook-check when no
// hooks file exists yet.
func TestInstallHook_CreatesHooksJson(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	client := &MCPClient{
		Name: "codex",
		HooksConfig: &HooksConfig{
			Path:   hooksPath,
			Format: "json",
		},
	}

	if err := installHook(client); err != nil {
		t.Fatalf("installHook: %v", err)
	}

	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("hooks.json not created: %v", err)
	}
	content := string(data)
	for _, want := range []string{"PreToolUse", "Bash", "git-courer", "hook-check"} {
		if !strings.Contains(content, want) {
			t.Errorf("hooks.json missing %q in:\n%s", want, content)
		}
	}
}

// TestInstallHook_Idempotent verifies installHook does NOT duplicate the
// git-courer entry when hooks.json already contains it.
func TestInstallHook_Idempotent(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")
	existing := `{"hooks":{"PreToolUse":[{"matcher":"Bash","command":"git-courer hook-check"}]}}`
	if err := os.WriteFile(hooksPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	client := &MCPClient{
		Name: "codex",
		HooksConfig: &HooksConfig{Path: hooksPath, Format: "json"},
	}
	if err := installHook(client); err != nil {
		t.Fatalf("installHook: %v", err)
	}

	data, _ := os.ReadFile(hooksPath)
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("hooks.json not valid JSON after idempotent install: %v", err)
	}
	hooks, _ := parsed["hooks"].(map[string]interface{})
	pre, _ := hooks["PreToolUse"].([]interface{})
	count := 0
	for _, e := range pre {
		em, _ := e.(map[string]interface{})
		if cmd, _ := em["command"].(string); cmd == "git-courer hook-check" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("git-courer hook entries after idempotent install: got %d, want 1", count)
	}
}

// TestInstallHook_MergesPreservesExisting verifies installHook merges the
// git-courer hook into an existing hooks.json that already has other hooks,
// preserving those other entries and creating a .bak backup.
func TestInstallHook_MergesPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")
	existing := `{"hooks":{"PreToolUse":[{"matcher":"Bash","command":"some-other-hook"}]}}`
	if err := os.WriteFile(hooksPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	client := &MCPClient{
		Name: "codex",
		HooksConfig: &HooksConfig{Path: hooksPath, Format: "json"},
	}
	if err := installHook(client); err != nil {
		t.Fatalf("installHook: %v", err)
	}

	// backup created
	if _, err := os.Stat(hooksPath + ".bak"); err != nil {
		t.Errorf("backup .bak not created: %v", err)
	}

	data, _ := os.ReadFile(hooksPath)
	content := string(data)
	if !strings.Contains(content, "some-other-hook") {
		t.Errorf("existing hook was lost during merge:\n%s", content)
	}
	if !strings.Contains(content, "git-courer hook-check") {
		t.Errorf("git-courer hook was not added during merge:\n%s", content)
	}
}

// TestRemoveHook_RemovesGitCourerEntry verifies removeHook strips the
// git-courer entry and leaves other hooks intact.
func TestRemoveHook_RemovesGitCourerEntry(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")
	existing := `{"hooks":{"PreToolUse":[{"matcher":"Bash","command":"some-other-hook"},{"matcher":"Bash","command":"git-courer hook-check"}]}}`
	if err := os.WriteFile(hooksPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	client := &MCPClient{
		Name: "codex",
		HooksConfig: &HooksConfig{Path: hooksPath, Format: "json"},
	}
	if err := removeHook(client); err != nil {
		t.Fatalf("removeHook: %v", err)
	}

	data, _ := os.ReadFile(hooksPath)
	content := string(data)
	if strings.Contains(content, "git-courer hook-check") {
		t.Errorf("git-courer entry still present after remove:\n%s", content)
	}
	if !strings.Contains(content, "some-other-hook") {
		t.Errorf("other hook was removed too:\n%s", content)
	}
}

// TestRemoveHook_DeletesFileWhenEmpty verifies removeHook deletes hooks.json
// when no hooks remain after removing the git-courer entry.
func TestRemoveHook_DeletesFileWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")
	existing := `{"hooks":{"PreToolUse":[{"matcher":"Bash","command":"git-courer hook-check"}]}}`
	if err := os.WriteFile(hooksPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	client := &MCPClient{
		Name: "codex",
		HooksConfig: &HooksConfig{Path: hooksPath, Format: "json"},
	}
	if err := removeHook(client); err != nil {
		t.Fatalf("removeHook: %v", err)
	}

	if _, err := os.Stat(hooksPath); err == nil {
		t.Error("hooks.json should have been deleted when empty, still exists")
	}
}

// TestRemoveHook_NoFileIsNoop verifies removeHook is a no-op (no error) when
// hooks.json does not exist.
func TestRemoveHook_NoFileIsNoop(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	client := &MCPClient{
		Name: "codex",
		HooksConfig: &HooksConfig{Path: hooksPath, Format: "json"},
	}
	if err := removeHook(client); err != nil {
		t.Errorf("removeHook on missing file returned error: %v", err)
	}
}

// TestHooksStatus_Installed verifies hooksStatus returns "installed" when
// hooks.json exists and contains the git-courer PreToolUse entry.
func TestHooksStatus_Installed(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")
	existing := `{"hooks":{"PreToolUse":[{"matcher":"Bash","command":"git-courer hook-check"}]}}`
	if err := os.WriteFile(hooksPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	client := &MCPClient{
		Name: "codex",
		HooksConfig: &HooksConfig{Path: hooksPath, Format: "json"},
	}
	if got := hooksStatus(client); got != "installed" {
		t.Errorf("hooksStatus: got %q, want %q", got, "installed")
	}
}

// TestHooksStatus_NotInstalled_NoFile verifies hooksStatus returns
// "not_installed" when hooks.json does not exist.
func TestHooksStatus_NotInstalled_NoFile(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	client := &MCPClient{
		Name: "codex",
		HooksConfig: &HooksConfig{Path: hooksPath, Format: "json"},
	}
	if got := hooksStatus(client); got != "not_installed" {
		t.Errorf("hooksStatus: got %q, want %q", got, "not_installed")
	}
}

// TestHooksStatus_NotInstalled_NoGitCourerEntry verifies hooksStatus returns
// "not_installed" when hooks.json exists but lacks the git-courer entry.
func TestHooksStatus_NotInstalled_NoGitCourerEntry(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")
	existing := `{"hooks":{"PreToolUse":[{"matcher":"Bash","command":"some-other-hook"}]}}`
	if err := os.WriteFile(hooksPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	client := &MCPClient{
		Name: "codex",
		HooksConfig: &HooksConfig{Path: hooksPath, Format: "json"},
	}
	if got := hooksStatus(client); got != "not_installed" {
		t.Errorf("hooksStatus: got %q, want %q", got, "not_installed")
	}
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