// Package installer_test verifies ConfigureMCP installs PreToolUse hooks
// for clients that support them (SDD 2, Phase 3).
package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// codexTestClient builds a Codex-like MCPClient whose config and hooks paths
// live inside the provided temp dir, so tests never touch the real home.
func codexTestClient(t *testing.T, dir string) *MCPClient {
	t.Helper()
	configPath := filepath.Join(dir, "config.toml")
	hooksPath := filepath.Join(dir, "hooks.json")
	return &MCPClient{
		Name:     "codex",
		Filename: "config.toml",
		RootKey:  "mcpServers",
		ConfigFn: func(binPath string) map[string]interface{} {
			return map[string]interface{}{"command": binPath, "args": []string{"mcp"}}
		},
		Paths:  []string{configPath},
		Detect: func() bool { return true },
		HooksConfig: &HooksConfig{
			Path:   hooksPath,
			Format: "json",
		},
	}
}

// noHooksTestClient builds a client WITHOUT HooksConfig (e.g. opencode) so
// we can assert ConfigureMCP does not create any hooks.json for it.
func noHooksTestClient(t *testing.T, dir string) *MCPClient {
	t.Helper()
	configPath := filepath.Join(dir, "opencode.json")
	return &MCPClient{
		Name:     "opencode",
		Filename: "opencode.json",
		RootKey:  "mcp",
		ConfigFn: func(binPath string) map[string]interface{} {
			return map[string]interface{}{"command": []string{binPath, "mcp"}}
		},
		Paths:  []string{configPath},
		Detect: func() bool { return true },
		// HooksConfig intentionally nil
	}
}

// assertHooksJsonHasGitCourer reads hooksPath and verifies it contains
// exactly one git-courer PreToolUse entry.
func assertHooksJsonHasGitCourer(t *testing.T, hooksPath string, wantCount int) {
	t.Helper()
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("hooks.json not created at %s: %v", hooksPath, err)
	}
	content := string(data)
	for _, want := range []string{"PreToolUse", "Bash", "git-courer", "hook-check"} {
		if !strings.Contains(content, want) {
			t.Errorf("hooks.json missing %q in:\n%s", want, content)
		}
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("hooks.json not valid JSON: %v", err)
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
	if count != wantCount {
		t.Errorf("git-courer hook entries: got %d, want %d\ncontent:\n%s", count, wantCount, content)
	}
}

// TestConfigureMCP_InstallsHook verifies ConfigureMCP creates hooks.json
// with the git-courer PreToolUse entry for a client that has HooksConfig.
func TestConfigureMCP_InstallsHook(t *testing.T) {
	dir := t.TempDir()
	client := codexTestClient(t, dir)

	if err := ConfigureMCP(client, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("ConfigureMCP: %v", err)
	}

	assertHooksJsonHasGitCourer(t, client.HooksConfig.Path, 1)
}

// TestConfigureMCP_NoHooksConfig verifies ConfigureMCP does NOT create any
// hooks.json for a client without HooksConfig (e.g. opencode).
func TestConfigureMCP_NoHooksConfig(t *testing.T) {
	dir := t.TempDir()
	client := noHooksTestClient(t, dir)

	if err := ConfigureMCP(client, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("ConfigureMCP: %v", err)
	}

	// No hooks file should exist anywhere in the temp dir.
	matches, _ := filepath.Glob(filepath.Join(dir, "*hooks*"))
	if len(matches) != 0 {
		t.Errorf("no hooks.json expected for client without HooksConfig, found: %v", matches)
	}
}

// TestConfigureMCP_HooksIdempotent verifies calling ConfigureMCP twice
// produces exactly one git-courer entry (not duplicated).
func TestConfigureMCP_HooksIdempotent(t *testing.T) {
	dir := t.TempDir()
	client := codexTestClient(t, dir)

	if err := ConfigureMCP(client, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("ConfigureMCP first call: %v", err)
	}
	if err := ConfigureMCP(client, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("ConfigureMCP second call: %v", err)
	}

	assertHooksJsonHasGitCourer(t, client.HooksConfig.Path, 1)
}

// TestConfigureMCP_InstallsHookOnEarlyReturn verifies ConfigureMCP installs
// the hook even on the early-return path (when the config already contains
// git-courer). This guards the "already configured" branch.
func TestConfigureMCP_InstallsHookOnEarlyReturn(t *testing.T) {
	dir := t.TempDir()
	client := codexTestClient(t, dir)

	// First call: writes config + hooks.
	if err := ConfigureMCP(client, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("ConfigureMCP first call: %v", err)
	}
	// Remove hooks.json to prove the second (early-return) call reinstalls it.
	if err := os.Remove(client.HooksConfig.Path); err != nil {
		t.Fatalf("remove hooks.json: %v", err)
	}

	// Second call hits the containsGitCourer early-return branch.
	if err := ConfigureMCP(client, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("ConfigureMCP second call: %v", err)
	}

	assertHooksJsonHasGitCourer(t, client.HooksConfig.Path, 1)
}
