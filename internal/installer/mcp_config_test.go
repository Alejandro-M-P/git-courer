// Package installer_test verifies MCP config operations including TOML format
// support and hook installation.
package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestConfigureTomlFormat_WritesValidTOML verifies configureTomlFormat writes
// a valid TOML file with the correct [mcp_servers."git-courer"] section.
func TestConfigureTomlFormat_WritesValidTOML(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	entry := map[string]interface{}{
		"command": "/usr/local/bin/git-courer",
		"args":    []string{"mcp"},
	}

	if err := configureTomlFormat(configPath, "mcp_servers", entry); err != nil {
		t.Fatalf("configureTomlFormat: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, `[mcp_servers.git-courer]`) && !strings.Contains(content, `[mcp_servers."git-courer"]`) {
		t.Errorf("TOML missing section header [mcp_servers.git-courer]\ncontent:\n%s", content)
	}
	if !strings.Contains(content, `command = "/usr/local/bin/git-courer"`) {
		t.Errorf("TOML missing command field\ncontent:\n%s", content)
	}
	if !strings.Contains(content, `args = ["mcp"]`) {
		t.Errorf("TOML missing args field\ncontent:\n%s", content)
	}
}

// TestConfigureTomlFormat_PreservesExistingConfig verifies configureTomlFormat
// preserves existing content in the config file when adding git-courer.
func TestConfigureTomlFormat_PreservesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	existing := `[mcp_servers."other-tool"]
command = "other"
`
	if err := os.WriteFile(configPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	entry := map[string]interface{}{
		"command": "/usr/local/bin/git-courer",
		"args":    []string{"mcp"},
	}

	if err := configureTomlFormat(configPath, "mcp_servers", entry); err != nil {
		t.Fatalf("configureTomlFormat: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, `other-tool`) {
		t.Errorf("existing section was removed\ncontent:\n%s", content)
	}
	if !strings.Contains(content, `git-courer`) {
		t.Errorf("git-courer section missing\ncontent:\n%s", content)
	}
}

// TestConfigureObjectFormat_DelegatesToToml verifies configureObjectFormat
// delegates to configureTomlFormat when ConfigFormat is "toml".
func TestConfigureObjectFormat_DelegatesToToml(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")

	entry := map[string]interface{}{
		"command": "/usr/local/bin/git-courer",
		"args":    []string{"mcp"},
	}

	if err := configureObjectFormatWithFormat(configPath, "mcp_servers", entry, "toml"); err != nil {
		t.Fatalf("configureObjectFormatWithFormat: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, `[mcp_servers.git-courer]`) && !strings.Contains(content, `[mcp_servers."git-courer"]`) {
		t.Errorf("expected TOML output, got:\n%s", content)
	}
}

// TestConfigureObjectFormat_WritesJSON verifies configureObjectFormat writes
// JSON when ConfigFormat is "json" (default).
func TestConfigureObjectFormat_WritesJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	entry := map[string]interface{}{
		"command": "/usr/local/bin/git-courer",
		"args":    []string{"mcp"},
	}

	if err := configureObjectFormatWithFormat(configPath, "mcpServers", entry, "json"); err != nil {
		t.Fatalf("configureObjectFormatWithFormat: %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, `"mcpServers"`) {
		t.Errorf("expected JSON output with mcpServers key, got:\n%s", content)
	}
	if !strings.Contains(content, `"git-courer"`) {
		t.Errorf("expected JSON output with git-courer entry, got:\n%s", content)
	}
}

// --- installClaudeHooks tests (T8) ---

// claudeSettingsShell is the decode target for assertions on settings.json.
// Only the fields the tests care about are typed; everything else is ignored.
type claudeSettingsShell struct {
	Hooks       map[string][]claudeHookEntry `json:"hooks"`
	Permissions interface{}                  `json:"permissions,omitempty"`
	Model       interface{}                  `json:"model,omitempty"`
	Theme       interface{}                  `json:"theme,omitempty"`
}

// readClaudeSettings reads and parses settings.json into claudeSettingsShell.
func readClaudeSettings(t *testing.T, path string) claudeSettingsShell {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	var s claudeSettingsShell
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("settings.json is not valid JSON: %v\ncontent: %s", err, data)
	}
	return s
}

// assertGitCourerHookPresent checks that event has an entry with matcher whose
// hook command contains "git-courer".
func assertGitCourerHookPresent(t *testing.T, s claudeSettingsShell, event, matcher string) {
	t.Helper()
	entries, ok := s.Hooks[event]
	if !ok {
		t.Errorf("event %q missing from hooks", event)
		return
	}
	for _, e := range entries {
		if e.Matcher != matcher {
			continue
		}
		for _, cmd := range e.Hooks {
			if strings.Contains(cmd.Command, "git-courer") {
				return
			}
		}
	}
	t.Errorf("no git-courer hook found for event %q matcher %q", event, matcher)
}

// TestInstallClaudeHooks_CreatesHooksInEmptyFile verifies installClaudeHooks
// creates all three git-courer hook events in a non-existent settings.json
// with the expected matchers and a command containing "git-courer".
func TestInstallClaudeHooks_CreatesHooksInEmptyFile(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	if err := installClaudeHooks(settingsPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("installClaudeHooks: %v", err)
	}

	s := readClaudeSettings(t, settingsPath)
	assertGitCourerHookPresent(t, s, "PreToolUse", "Bash")
	assertGitCourerHookPresent(t, s, "SessionStart", "startup|resume")
	assertGitCourerHookPresent(t, s, "SubagentStart", "general-purpose|Explore|Plan")
}

// TestInstallClaudeHooks_MergesWithExistingHooks verifies installClaudeHooks
// preserves existing non-git-courer hooks (tokensave-style) while adding the
// git-courer hooks for every event.
func TestInstallClaudeHooks_MergesWithExistingHooks(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// Pre-existing tokensave-style hooks for two of the three events.
	existing := `{
		"hooks": {
			"PreToolUse": [
				{"matcher": "Agent", "hooks": [{"type": "command", "command": "tokensave scan"}]}
			],
			"SessionStart": [
				{"matcher": "*", "hooks": [{"type": "command", "command": "tokensave on-session"}]}
			]
		}
	}`
	if err := os.WriteFile(settingsPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	if err := installClaudeHooks(settingsPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("installClaudeHooks: %v", err)
	}

	s := readClaudeSettings(t, settingsPath)

	// git-courer hooks present.
	assertGitCourerHookPresent(t, s, "PreToolUse", "Bash")
	assertGitCourerHookPresent(t, s, "SessionStart", "startup|resume")
	assertGitCourerHookPresent(t, s, "SubagentStart", "general-purpose|Explore|Plan")

	// tokensave hooks preserved.
	tokensavePre := false
	for _, e := range s.Hooks["PreToolUse"] {
		if e.Matcher == "Agent" && len(e.Hooks) > 0 && e.Hooks[0].Command == "tokensave scan" {
			tokensavePre = true
		}
	}
	if !tokensavePre {
		t.Error("tokensave PreToolUse hook was not preserved")
	}

	tokensaveSession := false
	for _, e := range s.Hooks["SessionStart"] {
		if e.Matcher == "*" && len(e.Hooks) > 0 && e.Hooks[0].Command == "tokensave on-session" {
			tokensaveSession = true
		}
	}
	if !tokensaveSession {
		t.Error("tokensave SessionStart hook was not preserved")
	}
}

// TestInstallClaudeHooks_Idempotent verifies that running installClaudeHooks
// twice produces byte-identical settings.json output.
func TestInstallClaudeHooks_Idempotent(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	if err := installClaudeHooks(settingsPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("first install: %v", err)
	}
	first, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read after first install: %v", err)
	}

	if err := installClaudeHooks(settingsPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("second install: %v", err)
	}
	second, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read after second install: %v", err)
	}

	if string(first) != string(second) {
		t.Errorf("not idempotent — outputs differ\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestInstallClaudeHooks_UpdatesCommandOnBinPathChange verifies that
// re-installing with a different binPath updates the command in place rather
// than creating a duplicate entry.
func TestInstallClaudeHooks_UpdatesCommandOnBinPathChange(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	if err := installClaudeHooks(settingsPath, "/old/path/git-courer"); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if err := installClaudeHooks(settingsPath, "/new/path/git-courer"); err != nil {
		t.Fatalf("second install: %v", err)
	}

	s := readClaudeSettings(t, settingsPath)
	// PreToolUse/Bash must have exactly one git-courer command, and it must
	// reference the new path.
	entry := findClaudeEntry(t, s.Hooks, "PreToolUse", "Bash")
	gitCourerCount := 0
	for _, cmd := range entry.Hooks {
		if strings.Contains(cmd.Command, "git-courer") {
			gitCourerCount++
			if !strings.Contains(cmd.Command, "/new/path/git-courer") {
				t.Errorf("command not updated to new path: got %q", cmd.Command)
			}
		}
	}
	if gitCourerCount != 1 {
		t.Errorf("expected exactly 1 git-courer command, got %d (duplicate detected)", gitCourerCount)
	}
}

// TestInstallClaudeHooks_PreservesUnknownSettingsKeys verifies that
// top-level settings keys we do not own (permissions, model, theme) survive
// the merge.
func TestInstallClaudeHooks_PreservesUnknownSettingsKeys(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	existing := `{
		"permissions": {"allow": ["Bash(go:*)"]},
		"model": "claude-sonnet-4",
		"theme": "dark"
	}`
	if err := os.WriteFile(settingsPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	if err := installClaudeHooks(settingsPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("installClaudeHooks: %v", err)
	}

	s := readClaudeSettings(t, settingsPath)
	if s.Permissions == nil {
		t.Error("permissions key was dropped")
	}
	if s.Model == nil {
		t.Error("model key was dropped")
	}
	if s.Theme == nil {
		t.Error("theme key was dropped")
	}
}

// TestInstallClaudeHooks_CreatesBackup verifies installClaudeHooks backs up an
// existing settings.json to settings.json.bak before mutating it.
func TestInstallClaudeHooks_CreatesBackup(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	original := `{"permissions": {"allow": []}}`
	if err := os.WriteFile(settingsPath, []byte(original), 0644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	if err := installClaudeHooks(settingsPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("installClaudeHooks: %v", err)
	}

	bakPath := settingsPath + ".bak"
	if _, err := os.Stat(bakPath); err != nil {
		t.Fatalf("backup file not created: %v", err)
	}
	bakData, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(bakData) != original {
		t.Errorf("backup content mismatch:\ngot:  %s\nwant: %s", bakData, original)
	}
}

// --- removeClaudeHooks tests (T9) ---

// TestRemoveClaudeHooks_StripsOnlyGitCourer verifies that removeClaudeHooks
// removes only the git-courer hooks while leaving non-git-courer hooks (e.g.
// tokensave) intact.
func TestRemoveClaudeHooks_StripsOnlyGitCourer(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	existing := `{
		"hooks": {
			"PreToolUse": [
				{"matcher": "Agent", "hooks": [{"type": "command", "command": "tokensave scan"}]},
				{"matcher": "Bash", "hooks": [{"type": "command", "command": "/usr/local/bin/git-courer hook-check"}]}
			],
			"SessionStart": [
				{"matcher": "*", "hooks": [{"type": "command", "command": "tokensave on-session"}]},
				{"matcher": "startup|resume", "hooks": [{"type": "command", "command": "/usr/local/bin/git-courer session-start-hook"}]}
			]
		}
	}`
	if err := os.WriteFile(settingsPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	if err := removeClaudeHooks(settingsPath); err != nil {
		t.Fatalf("removeClaudeHooks: %v", err)
	}

	s := readClaudeSettings(t, settingsPath)

	// tokensave hooks must remain.
	tokensavePreFound := false
	for _, e := range s.Hooks["PreToolUse"] {
		if e.Matcher == "Agent" && len(e.Hooks) > 0 && e.Hooks[0].Command == "tokensave scan" {
			tokensavePreFound = true
		}
	}
	if !tokensavePreFound {
		t.Error("tokensave PreToolUse hook was removed")
	}

	tokensaveSessionFound := false
	for _, e := range s.Hooks["SessionStart"] {
		if e.Matcher == "*" && len(e.Hooks) > 0 && e.Hooks[0].Command == "tokensave on-session" {
			tokensaveSessionFound = true
		}
	}
	if !tokensaveSessionFound {
		t.Error("tokensave SessionStart hook was removed")
	}

	// git-courer hooks must be gone: no entry for git-courer matchers.
	for _, e := range s.Hooks["PreToolUse"] {
		if e.Matcher == "Bash" {
			for _, cmd := range e.Hooks {
				if strings.Contains(cmd.Command, "git-courer") {
					t.Error("git-courer PreToolUse hook was not removed")
				}
			}
		}
	}
	for _, e := range s.Hooks["SessionStart"] {
		if e.Matcher == "startup|resume" {
			for _, cmd := range e.Hooks {
				if strings.Contains(cmd.Command, "git-courer") {
					t.Error("git-courer SessionStart hook was not removed")
				}
			}
		}
	}
}

// TestRemoveClaudeHooks_RestoresBackup verifies that when a .bak file exists,
// removeClaudeHooks restores the settings.json from the backup and removes
// the .bak file.
func TestRemoveClaudeHooks_RestoresBackup(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	current := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"/usr/local/bin/git-courer hook-check"}]}]}}`
	if err := os.WriteFile(settingsPath, []byte(current), 0644); err != nil {
		t.Fatalf("write current: %v", err)
	}

	backupContent := `{"permissions":{"allow":[]}}`
	if err := os.WriteFile(settingsPath+".bak", []byte(backupContent), 0644); err != nil {
		t.Fatalf("write backup: %v", err)
	}

	if err := removeClaudeHooks(settingsPath); err != nil {
		t.Fatalf("removeClaudeHooks: %v", err)
	}

	restored, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json not restored: %v", err)
	}
	if string(restored) != backupContent {
		t.Errorf("restored content mismatch:\ngot:  %s\nwant: %s", restored, backupContent)
	}

	if _, err := os.Stat(settingsPath + ".bak"); err == nil {
		t.Error("backup file should have been removed after restore")
	}
}

// TestRemoveClaudeHooks_NoOpWhenNoGitCourer verifies that removeClaudeHooks is
// a no-op (does not rewrite the file) when settings.json has only non-git-courer
// hooks.
func TestRemoveClaudeHooks_NoOpWhenNoGitCourer(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	original := `{"hooks":{"PreToolUse":[{"matcher":"Agent","hooks":[{"type":"command","command":"tokensave scan"}]}]}}`
	if err := os.WriteFile(settingsPath, []byte(original), 0644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	beforeStat, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}

	if err := removeClaudeHooks(settingsPath); err != nil {
		t.Fatalf("removeClaudeHooks: %v", err)
	}

	after, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(after) != original {
		t.Errorf("file was modified despite no git-courer hooks\ngot:  %s\nwant: %s", after, original)
	}

	afterStat, err := os.Stat(settingsPath)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if !afterStat.ModTime().Equal(beforeStat.ModTime()) {
		t.Errorf("file was rewritten — modtime changed: before=%v after=%v", beforeStat.ModTime(), afterStat.ModTime())
	}
}

// TestRemoveClaudeHooks_NoOpWhenNoFile verifies that removeClaudeHooks returns
// nil (no error) when settings.json does not exist at all.
func TestRemoveClaudeHooks_NoOpWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "does-not-exist.json")

	if err := removeClaudeHooks(settingsPath); err != nil {
		t.Errorf("removeClaudeHooks on missing file returned error: %v", err)
	}
}
