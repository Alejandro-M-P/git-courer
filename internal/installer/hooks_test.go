// Package installer_test verifies hook installation, removal, and status.
package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestInstallHook_CreatesHooksJSON verifies installHook creates hooks.json
// with PreToolUse, SessionStart, SubagentStart, and PreInvocation entries.
func TestInstallHook_CreatesHooksJSON(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	if err := installHook(hooksPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("installHook: %v", err)
	}

	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("hooks.json not created: %v", err)
	}

	var hooks struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &hooks); err != nil {
		t.Fatalf("hooks.json is not valid JSON: %v\ncontent: %s", err, data)
	}

	// Check PreToolUse entry
	preToolUse, ok := hooks.Hooks["PreToolUse"]
	if !ok {
		t.Fatal("missing PreToolUse entry")
	}
	if len(preToolUse) != 1 {
		t.Fatalf("PreToolUse: got %d entries, want 1", len(preToolUse))
	}
	if preToolUse[0].Matcher != "Bash" {
		t.Errorf("PreToolUse matcher: got %q, want %q", preToolUse[0].Matcher, "Bash")
	}
	if len(preToolUse[0].Hooks) != 1 {
		t.Fatalf("PreToolUse hooks: got %d, want 1", len(preToolUse[0].Hooks))
	}
	if !strings.Contains(preToolUse[0].Hooks[0].Command, "hook-check") {
		t.Errorf("PreToolUse command missing hook-check: %q", preToolUse[0].Hooks[0].Command)
	}

	// Check SessionStart entry
	sessionStart, ok := hooks.Hooks["SessionStart"]
	if !ok {
		t.Fatal("missing SessionStart entry")
	}
	if len(sessionStart) != 1 {
		t.Fatalf("SessionStart: got %d entries, want 1", len(sessionStart))
	}
	if sessionStart[0].Matcher != "startup|resume" {
		t.Errorf("SessionStart matcher: got %q, want %q", sessionStart[0].Matcher, "startup|resume")
	}
	if !strings.Contains(sessionStart[0].Hooks[0].Command, "session-start-hook") {
		t.Errorf("SessionStart command missing session-start-hook: %q", sessionStart[0].Hooks[0].Command)
	}

	// Check SubagentStart entry
	subagentStart, ok := hooks.Hooks["SubagentStart"]
	if !ok {
		t.Fatal("missing SubagentStart entry")
	}
	if len(subagentStart) != 1 {
		t.Fatalf("SubagentStart: got %d entries, want 1", len(subagentStart))
	}
	if subagentStart[0].Matcher != "general-purpose|Explore|Plan" {
		t.Errorf("SubagentStart matcher: got %q, want %q", subagentStart[0].Matcher, "general-purpose|Explore|Plan")
	}
	if !strings.Contains(subagentStart[0].Hooks[0].Command, "subagent-start-hook") {
		t.Errorf("SubagentStart command missing subagent-start-hook: %q", subagentStart[0].Hooks[0].Command)
	}

	// Check PreInvocation entry (added in Phase 2).
	preInvocation, ok := hooks.Hooks["PreInvocation"]
	if !ok {
		t.Fatal("missing PreInvocation entry")
	}
	if len(preInvocation) != 1 {
		t.Fatalf("PreInvocation: got %d entries, want 1", len(preInvocation))
	}
	if !strings.Contains(preInvocation[0].Hooks[0].Command, "pre-invocation-hook") {
		t.Errorf("PreInvocation command missing pre-invocation-hook: %q", preInvocation[0].Hooks[0].Command)
	}
}

// TestInstallHook_Idempotent verifies installHook does not duplicate entries
// when called a second time.
func TestInstallHook_Idempotent(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	if err := installHook(hooksPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("first installHook: %v", err)
	}

	if err := installHook(hooksPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("second installHook: %v", err)
	}

	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("hooks.json not found: %v", err)
	}

	var hooks struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &hooks); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// Each hook type should have exactly 1 entry (no duplicates).
	for _, hookType := range []string{"PreToolUse", "SessionStart", "SubagentStart"} {
		entries := hooks.Hooks[hookType]
		if len(entries) != 1 {
			t.Errorf("%s: got %d entries, want 1 (duplicate detected)", hookType, len(entries))
		}
	}
}

// TestInstallHook_BackupsExisting verifies installHook backs up an existing
// hooks.json before mutation.
func TestInstallHook_BackupsExisting(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	original := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo hello"}]}]}}`
	if err := os.WriteFile(hooksPath, []byte(original), 0644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	if err := installHook(hooksPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("installHook: %v", err)
	}

	// Verify backup exists with original content.
	bakData, err := os.ReadFile(hooksPath + ".bak")
	if err != nil {
		t.Fatalf("backup not created: %v", err)
	}
	if string(bakData) != original {
		t.Errorf("backup content mismatch:\ngot:  %s\nwant: %s", bakData, original)
	}
}

// TestRemoveHook_RemovesHooksJSON verifies removeHook deletes hooks.json.
func TestRemoveHook_RemovesHooksJSON(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	if err := installHook(hooksPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("installHook: %v", err)
	}

	if err := RemoveHook(hooksPath); err != nil {
		t.Fatalf("RemoveHook: %v", err)
	}

	if _, err := os.Stat(hooksPath); err == nil {
		t.Error("hooks.json still exists after RemoveHook")
	}
}

// TestRemoveHook_RestoresBackup verifies RemoveHook restores .bak when it exists.
func TestRemoveHook_RestoresBackup(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	original := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo hello"}]}]}}`
	if err := os.WriteFile(hooksPath, []byte(original), 0644); err != nil {
		t.Fatalf("write original: %v", err)
	}
	if err := os.WriteFile(hooksPath+".bak", []byte(original), 0644); err != nil {
		t.Fatalf("write backup: %v", err)
	}

	if err := RemoveHook(hooksPath); err != nil {
		t.Fatalf("removeHook: %v", err)
	}

	// hooks.json should be restored from .bak.
	restored, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("hooks.json not restored: %v", err)
	}
	if string(restored) != original {
		t.Errorf("restored content mismatch:\ngot:  %s\nwant: %s", restored, original)
	}

	// .bak should be removed.
	if _, err := os.Stat(hooksPath + ".bak"); err == nil {
		t.Error("backup file should have been removed after restore")
	}
}

// TestHooksStatus_Installed verifies hooksStatus returns "installed" when
// hooks.json exists with a git-courer PreToolUse entry.
func TestHooksStatus_Installed(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	if err := installHook(hooksPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("installHook: %v", err)
	}

	status := hooksStatus(hooksPath)
	if status != "installed" {
		t.Errorf("hooksStatus: got %q, want %q", status, "installed")
	}
}

// TestHooksStatus_NotInstalled verifies hooksStatus returns "not_installed"
// when hooks.json does not exist.
func TestHooksStatus_NotInstalled(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	status := hooksStatus(hooksPath)
	if status != "not_installed" {
		t.Errorf("hooksStatus: got %q, want %q", status, "not_installed")
	}
}

// TestHooksStatus_NotInstalledNoGitCourer verifies hooksStatus returns
// "not_installed" when hooks.json exists but lacks a git-courer entry.
func TestHooksStatus_NotInstalledNoGitCourer(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	other := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo hello"}]}]}}`
	if err := os.WriteFile(hooksPath, []byte(other), 0644); err != nil {
		t.Fatalf("write hooks.json: %v", err)
	}

	status := hooksStatus(hooksPath)
	if status != "not_installed" {
		t.Errorf("hooksStatus: got %q, want %q", status, "not_installed")
	}
}

// --- mergeClaudeHooks tests (T10) ---

// gitCourerMergeInput builds the git-courer hook entries used as the second
// argument to mergeClaudeHooks in tests. The matcher/command values mirror
// what installClaudeHooks produces in production so the test reflects reality.
func gitCourerMergeInput(binPath string) map[string][]claudeHookEntry {
	return map[string][]claudeHookEntry{
		"PreToolUse": {
			{
				Matcher: "Bash",
				Hooks:   []claudeHookCmd{{Type: "command", Command: binPath + " hook-check", Args: []string{}}},
			},
		},
		"SessionStart": {
			{
				Matcher: "startup|resume",
				Hooks:   []claudeHookCmd{{Type: "command", Command: binPath + " session-start-hook", Args: []string{}, Timeout: 10}},
			},
		},
		"SubagentStart": {
			{
				Matcher: "general-purpose|Explore|Plan",
				Hooks:   []claudeHookCmd{{Type: "command", Command: binPath + " subagent-start-hook", Args: []string{}, Timeout: 10}},
			},
		},
		"PreInvocation": {
			{
				Matcher: "",
				Hooks:   []claudeHookCmd{{Type: "command", Command: binPath + " pre-invocation-hook", Args: []string{}, Timeout: 10}},
			},
		},
	}
}

// findClaudeEntry returns the entry for matcher within event, or fails the test.
func findClaudeEntry(t *testing.T, hooks map[string][]claudeHookEntry, event, matcher string) claudeHookEntry {
	t.Helper()
	entries, ok := hooks[event]
	if !ok {
		t.Fatalf("event %q missing from merged hooks", event)
	}
	for _, e := range entries {
		if e.Matcher == matcher {
			return e
		}
	}
	t.Fatalf("no entry for matcher %q under event %q", matcher, event)
	return claudeHookEntry{}
}

// TestMergeClaudeHooks_EmptyExisting verifies that merging git-courer hooks
// into a nil existing map returns exactly the git-courer entries.
func TestMergeClaudeHooks_EmptyExisting(t *testing.T) {
	merged := mergeClaudeHooks(nil, gitCourerMergeInput("/usr/local/bin/git-courer"))
	if merged == nil {
		t.Fatal("merged map is nil")
	}

	for _, event := range claudeGitCourerHookEvents {
		entries, ok := merged[event]
		if !ok {
			t.Errorf("event %q missing", event)
			continue
		}
		if len(entries) != 1 {
			t.Errorf("%s: got %d entries, want 1", event, len(entries))
			continue
		}
		if len(entries[0].Hooks) != 1 {
			t.Errorf("%s: got %d hook commands, want 1", event, len(entries[0].Hooks))
			continue
		}
		if !strings.Contains(entries[0].Hooks[0].Command, "git-courer") {
			t.Errorf("%s: command %q does not contain git-courer", event, entries[0].Hooks[0].Command)
		}
	}
}

// TestMergeClaudeHooks_PreservesNonGitCourer verifies that a non-git-courer
// hook present in existing is preserved AND the git-courer hooks are added.
func TestMergeClaudeHooks_PreservesNonGitCourer(t *testing.T) {
	existing := map[string][]claudeHookEntry{
		"PreToolUse": {
			{
				Matcher: "Agent",
				Hooks:   []claudeHookCmd{{Type: "command", Command: "tokensave scan"}},
			},
		},
	}

	merged := mergeClaudeHooks(existing, gitCourerMergeInput("/usr/local/bin/git-courer"))

	// Non-git-courer hook must survive.
	preEntries := merged["PreToolUse"]
	var tokensaveFound, gitcourerFound bool
	for _, e := range preEntries {
		for _, cmd := range e.Hooks {
			if cmd.Command == "tokensave scan" && e.Matcher == "Agent" {
				tokensaveFound = true
			}
			if strings.Contains(cmd.Command, "git-courer") && e.Matcher == "Bash" {
				gitcourerFound = true
			}
		}
	}
	if !tokensaveFound {
		t.Error("tokensave hook was not preserved")
	}
	if !gitcourerFound {
		t.Error("git-courer hook was not added")
	}
}

// TestMergeClaudeHooks_UpdatesExistingGitCourer verifies that when an existing
// entry already has a git-courer command for the same matcher, the command is
// updated in place (not duplicated) — the behavior that handles a binary path
// change.
func TestMergeClaudeHooks_UpdatesExistingGitCourer(t *testing.T) {
	existing := map[string][]claudeHookEntry{
		"PreToolUse": {
			{
				Matcher: "Bash",
				Hooks: []claudeHookCmd{
					{Type: "command", Command: "/old/path/git-courer hook-check", Args: []string{}},
				},
			},
		},
	}

	merged := mergeClaudeHooks(existing, gitCourerMergeInput("/new/path/git-courer"))

	entry := findClaudeEntry(t, merged, "PreToolUse", "Bash")
	if len(entry.Hooks) != 1 {
		t.Fatalf("PreToolUse/Bash: got %d hook commands, want 1 (no duplication)", len(entry.Hooks))
	}
	if entry.Hooks[0].Command != "/new/path/git-courer hook-check" {
		t.Errorf("command not updated: got %q, want %q", entry.Hooks[0].Command, "/new/path/git-courer hook-check")
	}
}

// --- installAntigravityHooks tests (Phase 2) ---

// TestInstallAntigravityHooks_CreatesHooksJSON verifies installAntigravityHooks
// creates hooks.json with PreToolUse (matcher run_command) and PreInvocation
// entries pointing at the git-courer binary.
func TestInstallAntigravityHooks_CreatesHooksJSON(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	if err := installAntigravityHooks(hooksPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("installAntigravityHooks: %v", err)
	}

	data, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("hooks.json not created: %v", err)
	}

	var hooks struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &hooks); err != nil {
		t.Fatalf("hooks.json is not valid JSON: %v\ncontent: %s", err, data)
	}

	// PreToolUse with run_command matcher.
	preToolUse, ok := hooks.Hooks["PreToolUse"]
	if !ok {
		t.Fatal("missing PreToolUse entry")
	}
	if len(preToolUse) != 1 {
		t.Fatalf("PreToolUse: got %d entries, want 1", len(preToolUse))
	}
	if preToolUse[0].Matcher != "run_command" {
		t.Errorf("PreToolUse matcher: got %q, want %q", preToolUse[0].Matcher, "run_command")
	}
	if len(preToolUse[0].Hooks) != 1 {
		t.Fatalf("PreToolUse hooks: got %d, want 1", len(preToolUse[0].Hooks))
	}
	if !strings.Contains(preToolUse[0].Hooks[0].Command, "hook-check") {
		t.Errorf("PreToolUse command missing hook-check: %q", preToolUse[0].Hooks[0].Command)
	}

	// PreInvocation entry.
	preInvocation, ok := hooks.Hooks["PreInvocation"]
	if !ok {
		t.Fatal("missing PreInvocation entry")
	}
	if len(preInvocation) != 1 {
		t.Fatalf("PreInvocation: got %d entries, want 1", len(preInvocation))
	}
	if !strings.Contains(preInvocation[0].Hooks[0].Command, "pre-invocation-hook") {
		t.Errorf("PreInvocation command missing pre-invocation-hook: %q", preInvocation[0].Hooks[0].Command)
	}
}

// TestInstallAntigravityHooks_Idempotent verifies installAntigravityHooks does
// not duplicate entries when called a second time.
func TestInstallAntigravityHooks_Idempotent(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	if err := installAntigravityHooks(hooksPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("first installAntigravityHooks: %v", err)
	}
	first, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("read after first: %v", err)
	}

	if err := installAntigravityHooks(hooksPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("second installAntigravityHooks: %v", err)
	}
	second, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("read after second: %v", err)
	}

	if string(first) != string(second) {
		t.Errorf("not idempotent — outputs differ\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestInstallAntigravityHooks_BackupsExisting verifies installAntigravityHooks
// backs up an existing hooks.json to hooks.json.bak before mutation.
func TestInstallAntigravityHooks_BackupsExisting(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	original := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo hello"}]}]}}`
	if err := os.WriteFile(hooksPath, []byte(original), 0644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	if err := installAntigravityHooks(hooksPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("installAntigravityHooks: %v", err)
	}

	bakData, err := os.ReadFile(hooksPath + ".bak")
	if err != nil {
		t.Fatalf("backup not created: %v", err)
	}
	if string(bakData) != original {
		t.Errorf("backup content mismatch:\ngot:  %s\nwant: %s", bakData, original)
	}
}

// TestInstallAntigravityHooks_PreservesNonGitCourerEntries verifies that a
// non-git-courer PreToolUse entry (matcher Bash) survives the merge.
func TestInstallAntigravityHooks_PreservesNonGitCourerEntries(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	existing := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo hello"}]}]}}`
	if err := os.WriteFile(hooksPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	if err := installAntigravityHooks(hooksPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("installAntigravityHooks: %v", err)
	}

	data, _ := os.ReadFile(hooksPath)
	var hooks struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
			Hooks   []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &hooks); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	// The existing Bash matcher entry must still be present.
	bashFound := false
	for _, e := range hooks.Hooks["PreToolUse"] {
		if e.Matcher == "Bash" && len(e.Hooks) > 0 && e.Hooks[0].Command == "echo hello" {
			bashFound = true
		}
	}
	if !bashFound {
		t.Error("existing Bash matcher entry was not preserved")
	}
	// The git-courer run_command matcher entry must also be present.
	runCmdFound := false
	for _, e := range hooks.Hooks["PreToolUse"] {
		if e.Matcher == "run_command" {
			runCmdFound = true
		}
	}
	if !runCmdFound {
		t.Error("git-courer run_command matcher entry was not added")
	}
}

// --- removeAntigravityHooks tests (Phase 2) ---

// TestRemoveAntigravityHooks_RestoresBackup verifies removeAntigravityHooks
// restores hooks.json from .bak when present and removes the .bak file.
func TestRemoveAntigravityHooks_RestoresBackup(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	original := `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"echo hello"}]}]}}`
	if err := os.WriteFile(hooksPath, []byte(original), 0644); err != nil {
		t.Fatalf("write original: %v", err)
	}
	if err := os.WriteFile(hooksPath+".bak", []byte(original), 0644); err != nil {
		t.Fatalf("write backup: %v", err)
	}

	if err := removeAntigravityHooks(hooksPath); err != nil {
		t.Fatalf("removeAntigravityHooks: %v", err)
	}

	restored, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("hooks.json not restored: %v", err)
	}
	if string(restored) != original {
		t.Errorf("restored content mismatch:\ngot:  %s\nwant: %s", restored, original)
	}
	if _, err := os.Stat(hooksPath + ".bak"); err == nil {
		t.Error("backup file should have been removed after restore")
	}
}

// TestRemoveAntigravityHooks_DeletesWhenNoBackup verifies removeAntigravityHooks
// deletes hooks.json when no .bak file exists.
func TestRemoveAntigravityHooks_DeletesWhenNoBackup(t *testing.T) {
	dir := t.TempDir()
	hooksPath := filepath.Join(dir, "hooks.json")

	if err := installAntigravityHooks(hooksPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("installAntigravityHooks: %v", err)
	}

	if err := removeAntigravityHooks(hooksPath); err != nil {
		t.Fatalf("removeAntigravityHooks: %v", err)
	}

	if _, err := os.Stat(hooksPath); err == nil {
		t.Error("hooks.json still exists after removeAntigravityHooks")
	}
}

// TestMergeClaudeHooks_AppendsToSameMatcher verifies that when an existing
// entry has the same matcher as a git-courer entry but only a non-git-courer
// command, the git-courer command is appended to that entry's hooks list (so
// git-courer runs alongside the other tool).
func TestMergeClaudeHooks_AppendsToSameMatcher(t *testing.T) {
	existing := map[string][]claudeHookEntry{
		"PreToolUse": {
			{
				Matcher: "Bash",
				Hooks:   []claudeHookCmd{{Type: "command", Command: "some-other-tool pre-bash"}},
			},
		},
	}

	merged := mergeClaudeHooks(existing, gitCourerMergeInput("/usr/local/bin/git-courer"))

	entry := findClaudeEntry(t, merged, "PreToolUse", "Bash")
	if len(entry.Hooks) != 2 {
		t.Fatalf("PreToolUse/Bash: got %d hook commands, want 2 (appended alongside existing)", len(entry.Hooks))
	}
	// First command preserved, second is git-courer.
	if entry.Hooks[0].Command != "some-other-tool pre-bash" {
		t.Errorf("existing command lost: got %q", entry.Hooks[0].Command)
	}
	if !strings.Contains(entry.Hooks[1].Command, "git-courer") {
		t.Errorf("git-courer command not appended: got %q", entry.Hooks[1].Command)
	}
}
