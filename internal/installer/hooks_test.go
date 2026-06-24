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
// with PreToolUse, SessionStart, and SubagentStart entries.
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
