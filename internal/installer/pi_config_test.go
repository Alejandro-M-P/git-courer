// Package installer_test verifies Pi @pi-lab/permissions configuration.
package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// piSettingsShell is the decode target for assertions on the Pi agent
// settings.json. Only the fields the tests care about are typed.
type piSettingsShell struct {
	Permissions struct {
		Rules []piPermissionRule `json:"rules"`
	} `json:"permissions"`
	Packages []string `json:"packages,omitempty"`
}

// readPiSettings reads and parses the Pi settings.json.
func readPiSettings(t *testing.T, path string) piSettingsShell {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read pi settings.json: %v", err)
	}
	var s piSettingsShell
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatalf("pi settings.json is not valid JSON: %v\ncontent: %s", err, data)
	}
	return s
}

// piRuleCount returns the number of git-courer rules (by reason) in a slice.
func piRuleCount(rules []piPermissionRule) int {
	count := 0
	for _, r := range rules {
		if piGitCourerRuleReasons[r.Reason] {
			count++
		}
	}
	return count
}

// TestPiBuildGitCourerRules_Returns23DenyPlus1Ask verifies the rule builder
// returns exactly 23 deny rules + 1 ask fallback, in the correct order.
func TestPiBuildGitCourerRules_Returns23DenyPlus1Ask(t *testing.T) {
	rules := piBuildGitCourerRules()
	if len(rules) != 24 {
		t.Fatalf("expected 24 rules (23 deny + 1 ask), got %d", len(rules))
	}

	// First 23 must be deny.
	for i, r := range rules[:23] {
		if r.Action != "deny" {
			t.Errorf("rule %d: expected action deny, got %q", i, r.Action)
		}
		if r.Match.Tool != "bash" {
			t.Errorf("rule %d: expected tool bash, got %q", i, r.Match.Tool)
		}
	}

	// Last rule must be ask fallback.
	last := rules[23]
	if last.Action != "ask" {
		t.Errorf("last rule: expected action ask, got %q", last.Action)
	}
	if last.Match.Params.Command != "git" {
		t.Errorf("last rule: expected command 'git', got %q", last.Match.Params.Command)
	}
	if last.Reason != "Use git-courer tools instead of raw git" {
		t.Errorf("last rule: unexpected reason %q", last.Reason)
	}
}

// TestPiBuildGitCourerRules_AllReasonsInSet verifies every generated reason
// string is present in piGitCourerRuleReasons (for idempotent merge/remove).
func TestPiBuildGitCourerRules_AllReasonsInSet(t *testing.T) {
	rules := piBuildGitCourerRules()
	for _, r := range rules {
		if !piGitCourerRuleReasons[r.Reason] {
			t.Errorf("reason %q not found in piGitCourerRuleReasons", r.Reason)
		}
	}
}

// TestPiMergeRules_DropsStaleGitCourerRules verifies that existing git-courer
// rules are dropped before appending fresh ones (idempotent re-run).
func TestPiMergeRules_DropsStaleGitCourerRules(t *testing.T) {
	gcRules := piBuildGitCourerRules()

	// Simulate existing rules: 2 user rules + 24 stale git-courer rules.
	existing := []interface{}{
		map[string]interface{}{
			"match":  map[string]interface{}{"tool": "bash", "params": map[string]interface{}{"command": "npm publish"}},
			"action": "deny",
			"reason": "User-defined npm publish block",
		},
	}
	for _, r := range gcRules {
		b, _ := json.Marshal(r)
		var m interface{}
		json.Unmarshal(b, &m)
		existing = append(existing, m)
	}
	existing = append(existing, map[string]interface{}{
		"match":  map[string]interface{}{"tool": "bash", "params": map[string]interface{}{"command": "rm -rf"}},
		"action": "deny",
		"reason": "User-defined rm block",
	})

	merged := piMergeRules(existing, gcRules)

	// Should have: 2 user rules + 24 fresh git-courer rules = 26.
	if len(merged) != 26 {
		t.Errorf("expected 26 merged rules (2 user + 24 gc), got %d", len(merged))
	}

	// User rules must be first and preserved.
	first, ok := merged[0].(map[string]interface{})
	if !ok || first["reason"] != "User-defined npm publish block" {
		t.Error("first user rule was not preserved at the front")
	}
	second, ok := merged[1].(map[string]interface{})
	if !ok || second["reason"] != "User-defined rm block" {
		t.Error("second user rule was not preserved at index 1")
	}
}

// TestPiMergeRules_NoExistingRules verifies merge works with an empty existing
// slice (fresh install).
func TestPiMergeRules_NoExistingRules(t *testing.T) {
	gcRules := piBuildGitCourerRules()
	merged := piMergeRules(nil, gcRules)
	if len(merged) != 24 {
		t.Errorf("expected 24 merged rules, got %d", len(merged))
	}
}

// TestInstallPiPermissions_CreatesFreshFile verifies installPiPermissions
// writes 24 rules (23 deny + 1 ask) in a fresh settings.json.
func TestInstallPiPermissions_CreatesFreshFile(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	if err := installPiPermissions(settingsPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("installPiPermissions: %v", err)
	}

	s := readPiSettings(t, settingsPath)
	gcCount := piRuleCount(s.Permissions.Rules)
	if gcCount != 24 {
		t.Errorf("expected 24 git-courer rules, got %d", gcCount)
	}

	// No backup should be created when the file did not exist.
	if _, err := os.Stat(settingsPath + ".gc.bak"); err == nil {
		t.Error("backup should not be created when settings.json did not exist")
	}
}

// TestInstallPiPermissions_MergesPreservingExisting verifies existing
// non-git-courer rules and top-level keys are preserved.
func TestInstallPiPermissions_MergesPreservingExisting(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	original := `{
		"permissions": {
			"rules": [
				{"match":{"tool":"bash","params":{"command":"npm publish"}},"action":"deny","reason":"block npm publish"}
			]
		},
		"packages": ["npm:pi-mcp-adapter"]
	}`
	if err := os.WriteFile(settingsPath, []byte(original), 0644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	if err := installPiPermissions(settingsPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("installPiPermissions: %v", err)
	}

	s := readPiSettings(t, settingsPath)

	// User rule preserved.
	foundUserRule := false
	for _, r := range s.Permissions.Rules {
		if r.Reason == "block npm publish" {
			foundUserRule = true
		}
	}
	if !foundUserRule {
		t.Error("user-defined rule was not preserved")
	}

	// git-courer rules present.
	gcCount := piRuleCount(s.Permissions.Rules)
	if gcCount != 24 {
		t.Errorf("expected 24 git-courer rules, got %d", gcCount)
	}

	// Non-permission key preserved.
	if len(s.Packages) == 0 || s.Packages[0] != "npm:pi-mcp-adapter" {
		t.Errorf("packages key was not preserved: %v", s.Packages)
	}

	// Backup created with original content.
	bakData, err := os.ReadFile(settingsPath + ".gc.bak")
	if err != nil {
		t.Fatalf("backup not created: %v", err)
	}
	if string(bakData) != original {
		t.Errorf("backup content mismatch:\ngot:  %s\nwant: %s", bakData, original)
	}
}

// TestInstallPiPermissions_Idempotent verifies two runs produce byte-identical
// output and do not overwrite the backup.
func TestInstallPiPermissions_Idempotent(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	// Pre-populate so a backup is created.
	original := `{"packages":["npm:pi-mcp-adapter"]}`
	if err := os.WriteFile(settingsPath, []byte(original), 0644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	if err := installPiPermissions(settingsPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("first install: %v", err)
	}
	first, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read after first: %v", err)
	}
	firstBakStat, err := os.Stat(settingsPath + ".gc.bak")
	if err != nil {
		t.Fatalf("backup not created: %v", err)
	}

	if err := installPiPermissions(settingsPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("second install: %v", err)
	}
	second, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read after second: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("not idempotent — outputs differ\nfirst:\n%s\nsecond:\n%s", first, second)
	}

	// Backup should not be overwritten on re-run.
	secondBakStat, err := os.Stat(settingsPath + ".gc.bak")
	if err != nil {
		t.Fatalf("backup missing after second run: %v", err)
	}
	if !secondBakStat.ModTime().Equal(firstBakStat.ModTime()) {
		t.Errorf("backup was overwritten on second run: before=%v after=%v", firstBakStat.ModTime(), secondBakStat.ModTime())
	}
}

// TestInstallPiPermissions_HandlesUnparseable verifies that when settings.json
// exists but is invalid JSON, it is backed up and a fresh file is written.
func TestInstallPiPermissions_HandlesUnparseable(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	corrupt := `{not valid json`
	if err := os.WriteFile(settingsPath, []byte(corrupt), 0644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}

	if err := installPiPermissions(settingsPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("installPiPermissions on corrupt file: %v", err)
	}

	s := readPiSettings(t, settingsPath)
	gcCount := piRuleCount(s.Permissions.Rules)
	if gcCount != 24 {
		t.Errorf("expected 24 git-courer rules after recovery, got %d", gcCount)
	}

	bakData, err := os.ReadFile(settingsPath + ".gc.bak")
	if err != nil {
		t.Fatalf("backup of corrupt file not created: %v", err)
	}
	if string(bakData) != corrupt {
		t.Errorf("backup content mismatch:\ngot:  %s\nwant: %s", bakData, corrupt)
	}
}

// TestRemovePiPermissions_RestoresBackup verifies that when a .gc.bak exists,
// removePiPermissions restores it and removes the .bak.
func TestRemovePiPermissions_RestoresBackup(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	current := `{"permissions":{"rules":[{"match":{"tool":"bash","params":{"command":"git status"}},"action":"deny","reason":"Use git-courer/status instead"}]}}`
	if err := os.WriteFile(settingsPath, []byte(current), 0644); err != nil {
		t.Fatalf("write current: %v", err)
	}
	backup := `{"packages":["npm:pi-mcp-adapter"]}`
	if err := os.WriteFile(settingsPath+".gc.bak", []byte(backup), 0644); err != nil {
		t.Fatalf("write backup: %v", err)
	}

	if err := removePiPermissions(settingsPath); err != nil {
		t.Fatalf("removePiPermissions: %v", err)
	}

	restored, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings.json not restored: %v", err)
	}
	if string(restored) != backup {
		t.Errorf("restored content mismatch:\ngot:  %s\nwant: %s", restored, backup)
	}
	if _, err := os.Stat(settingsPath + ".gc.bak"); err == nil {
		t.Error("backup file should have been removed after restore")
	}
}

// TestRemovePiPermissions_StripsWithoutBackup verifies that when no .gc.bak
// exists, removePiPermissions strips git-courer rules while preserving
// user-defined rules and other top-level keys.
func TestRemovePiPermissions_StripsWithoutBackup(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	existing := `{
		"permissions": {
			"rules": [
				{"match":{"tool":"bash","params":{"command":"npm publish"}},"action":"deny","reason":"block npm publish"},
				{"match":{"tool":"bash","params":{"command":"git status"}},"action":"deny","reason":"Use git-courer/status instead"},
				{"match":{"tool":"bash","params":{"command":"git"}},"action":"ask","reason":"Use git-courer tools instead of raw git"}
			]
		},
		"packages": ["npm:pi-mcp-adapter"]
	}`
	if err := os.WriteFile(settingsPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write existing: %v", err)
	}

	if err := removePiPermissions(settingsPath); err != nil {
		t.Fatalf("removePiPermissions: %v", err)
	}

	s := readPiSettings(t, settingsPath)

	// User rule preserved.
	foundUserRule := false
	for _, r := range s.Permissions.Rules {
		if r.Reason == "block npm publish" {
			foundUserRule = true
		}
	}
	if !foundUserRule {
		t.Error("user-defined rule was removed")
	}

	// git-courer rules stripped.
	for _, r := range s.Permissions.Rules {
		if piGitCourerRuleReasons[r.Reason] {
			t.Errorf("git-courer rule with reason %q was not stripped", r.Reason)
		}
	}

	// Non-permission key preserved.
	if len(s.Packages) == 0 || s.Packages[0] != "npm:pi-mcp-adapter" {
		t.Errorf("packages key was not preserved: %v", s.Packages)
	}
}

// TestRemovePiPermissions_Idempotent verifies that running removePiPermissions
// twice does not error.
func TestRemovePiPermissions_Idempotent(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	if err := installPiPermissions(settingsPath, "/usr/local/bin/git-courer"); err != nil {
		t.Fatalf("install: %v", err)
	}

	if err := removePiPermissions(settingsPath); err != nil {
		t.Fatalf("first remove: %v", err)
	}
	if err := removePiPermissions(settingsPath); err != nil {
		t.Fatalf("second remove: %v", err)
	}
}

// TestRemovePiPermissions_NoOpWhenNoFile verifies that removePiPermissions
// returns nil when settings.json does not exist.
func TestRemovePiPermissions_NoOpWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "does-not-exist.json")

	if err := removePiPermissions(settingsPath); err != nil {
		t.Errorf("removePiPermissions on missing file returned error: %v", err)
	}
}
