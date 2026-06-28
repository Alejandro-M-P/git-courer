// Package installer_test verifies OpenCode policy configuration
// (permission.bash "git *": "ask" rule and GIT_COURER.md instructions entry)
// added in SDD 4 — Global Agent Policy for OpenCode.
package installer

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// openCodeShell is the decode target for opencode.json assertions. Only the
// fields owned by git-courer policy are typed; everything else is preserved
// through the generic map but not asserted here.
type openCodeShell struct {
	Permission struct {
		Bash        map[string]string `json:"bash"`
		OtherKeys   map[string]interface{} `json:"-"`
	} `json:"permission"`
	Instructions []string `json:"instructions"`
}

// readOpenCodeConfig reads and parses opencode.json at configPath into a
// generic map (preserving all unknown keys) and returns it so tests can
// assert on the full structure without losing user content.
func readOpenCodeConfig(t *testing.T, configPath string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read opencode.json: %v", err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("opencode.json is not valid JSON: %v\ncontent: %s", err, data)
	}
	return cfg
}

// assertBashRule asserts that permission.bash contains key with the expected
// value.
func assertBashRule(t *testing.T, cfg map[string]interface{}, key, want string) {
	t.Helper()
	perm, ok := cfg["permission"].(map[string]interface{})
	if !ok {
		t.Fatal("permission key missing or not an object")
	}
	bash, ok := perm["bash"].(map[string]interface{})
	if !ok {
		t.Fatal("permission.bash missing or not an object")
	}
	got, ok := bash[key].(string)
	if !ok {
		t.Errorf("permission.bash[%q] missing", key)
		return
	}
	if got != want {
		t.Errorf("permission.bash[%q]: got %q, want %q", key, got, want)
	}
}

// assertBashRuleAbsent asserts that permission.bash does NOT contain key.
func assertBashRuleAbsent(t *testing.T, cfg map[string]interface{}, key string) {
	t.Helper()
	perm, ok := cfg["permission"].(map[string]interface{})
	if !ok {
		return // no permission — definitely absent
	}
	bash, ok := perm["bash"].(map[string]interface{})
	if !ok {
		return
	}
	if _, exists := bash[key]; exists {
		t.Errorf("permission.bash[%q] should be absent, but is present", key)
	}
}

// bashRuleOrder returns the serialized order of permission.bash keys as they
// appear in the raw JSON file. Go's json.MarshalIndent sorts map keys
// alphabetically, so this reflects last-match-wins ordering.
func bashRuleOrder(t *testing.T, configPath string) []string {
	t.Helper()
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read opencode.json: %v", err)
	}
	// Re-decode into a structure that preserves order via json.Decoder.
	// Use a raw token walk to find the bash object keys in file order.
	// Simplest robust approach: decode into the typed shell and rely on
	// alphabetical marshalling, but we want FILE order. Re-read raw and
	// parse the bash object substring.
	return bashKeyOrderFromRaw(t, data)
}

// bashKeyOrderFromRaw extracts the permission.bash object keys in the order
// they appear in the raw JSON bytes. This proves last-match-wins ordering
// without relying on Go's map (which sorts alphabetically on marshal).
func bashKeyOrderFromRaw(t *testing.T, data []byte) []string {
	t.Helper()
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("parse root: %v", err)
	}
	permRaw, ok := root["permission"]
	if !ok {
		return nil
	}
	var perm map[string]json.RawMessage
	if err := json.Unmarshal(permRaw, &perm); err != nil {
		t.Fatalf("parse permission: %v", err)
	}
	bashRaw, ok := perm["bash"]
	if !ok {
		return nil
	}
	var bash map[string]json.RawMessage
	if err := json.Unmarshal(bashRaw, &bash); err != nil {
		t.Fatalf("parse bash: %v", err)
	}
	// To preserve order we must re-parse the raw bash object with a token
	// decoder. json.RawMessage into a map loses order, so parse the raw
	// bytes directly with json.Decoder tokens.
	return keysInOrder(t, bashRaw)
}

// keysInOrder decodes a JSON object's raw bytes and returns top-level keys in
// the order they appear.
func keysInOrder(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	tok, err := dec.Token()
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '{' {
		t.Fatalf("expected object start, got %v", tok)
	}
	var keys []string
	for dec.More() {
		kTok, err := dec.Token()
		if err != nil {
			t.Fatalf("key token: %v", err)
		}
		k, ok := kTok.(string)
		if !ok {
			t.Fatalf("expected string key, got %v", kTok)
		}
		keys = append(keys, k)
		// Skip the value.
		var v interface{}
		if err := dec.Decode(&v); err != nil {
			t.Fatalf("decode value: %v", err)
		}
	}
	return keys
}

// assertInstructionsContains asserts that the instructions array contains
// exactly one entry equal to path.
func assertInstructionsContains(t *testing.T, cfg map[string]interface{}, path string) {
	t.Helper()
	raw, ok := cfg["instructions"]
	if !ok {
		t.Fatal("instructions key missing")
	}
	arr, ok := raw.([]interface{})
	if !ok {
		t.Fatalf("instructions is not an array, got %T", raw)
	}
	count := 0
	for _, v := range arr {
		if s, ok := v.(string); ok && s == path {
			count++
		}
	}
	if count != 1 {
		t.Errorf("instructions contains %d copies of %q, want exactly 1", count, path)
	}
}

// assertInstructionsAbsent asserts that the instructions array does NOT contain
// path.
func assertInstructionsAbsent(t *testing.T, cfg map[string]interface{}, path string) {
	t.Helper()
	raw, ok := cfg["instructions"]
	if !ok {
		return
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return
	}
	for _, v := range arr {
		if s, ok := v.(string); ok && s == path {
			t.Errorf("instructions should not contain %q, but it does", path)
			return
		}
	}
}

// gitCourerMdPath returns the expected AGENTS.md instructions path for a
// config directory (retaining name to minimize other changes).
func gitCourerMdPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), "AGENTS.md")
}

// --- configureOpenCodePolicy tests (T1) ---

// TestConfigureOpenCodePolicy_AddsGitStarRule verifies
// configureOpenCodePolicy adds "git *": "ask" to permission.bash on a fresh
// opencode.json with no permission key.
func TestConfigureOpenCodePolicy_AddsGitStarRule(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	// Start from a config with no permission key.
	if err := os.WriteFile(configPath, []byte(`{"mcp":{"git-courer":{"command":"x"}}}`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := configureOpenCodePolicy(configPath); err != nil {
		t.Fatalf("configureOpenCodePolicy: %v", err)
	}

	cfg := readOpenCodeConfig(t, configPath)
	assertBashRule(t, cfg, "git *", "ask")
}

// TestConfigureOpenCodePolicy_PreservesExistingBashKeys verifies that an
// existing "*" rule is preserved alongside the new "git *" rule, and that
// "git *" appears AFTER "*" in the serialized JSON (last-match-wins).
func TestConfigureOpenCodePolicy_PreservesExistingBashKeys(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	existing := `{
		"permission": {"bash": {"*": "allow"}}
	}`
	if err := os.WriteFile(configPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := configureOpenCodePolicy(configPath); err != nil {
		t.Fatalf("configureOpenCodePolicy: %v", err)
	}

	cfg := readOpenCodeConfig(t, configPath)
	assertBashRule(t, cfg, "*", "allow")
	assertBashRule(t, cfg, "git *", "ask")

	// Last-match-wins: "git *" must serialize after "*".
	order := bashRuleOrder(t, configPath)
	starIdx, gitIdx := -1, -1
	for i, k := range order {
		if k == "*" {
			starIdx = i
		}
		if k == "git *" {
			gitIdx = i
		}
	}
	if starIdx == -1 || gitIdx == -1 {
		t.Fatalf("expected both * and git * in order %v", order)
	}
	if gitIdx < starIdx {
		t.Errorf("git * (idx %d) must appear AFTER * (idx %d) for last-match-wins; order=%v", gitIdx, starIdx, order)
	}
}

// TestConfigureOpenCodePolicy_AddsInstructions verifies the GIT_COURER.md path
// is added to the instructions array exactly once.
func TestConfigureOpenCodePolicy_AddsInstructions(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(configPath, []byte(`{}`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := configureOpenCodePolicy(configPath); err != nil {
		t.Fatalf("configureOpenCodePolicy: %v", err)
	}

	cfg := readOpenCodeConfig(t, configPath)
	assertInstructionsContains(t, cfg, gitCourerMdPath(configPath))
}

// TestConfigureOpenCodePolicy_PreservesExistingInstructions verifies other
// instructions entries are kept and GIT_COURER.md is deduplicated.
func TestConfigureOpenCodePolicy_PreservesExistingInstructions(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	other := filepath.Join(dir, "OTHER.md")
	existing := `{"instructions": ["` + other + `"]}`
	if err := os.WriteFile(configPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := configureOpenCodePolicy(configPath); err != nil {
		t.Fatalf("configureOpenCodePolicy: %v", err)
	}

	cfg := readOpenCodeConfig(t, configPath)
	arr, _ := cfg["instructions"].([]interface{})
	foundOther := false
	for _, v := range arr {
		if s, ok := v.(string); ok && s == other {
			foundOther = true
		}
	}
	if !foundOther {
		t.Errorf("existing instructions entry %q was dropped", other)
	}
	assertInstructionsContains(t, cfg, gitCourerMdPath(configPath))
}

// TestConfigureOpenCodePolicy_Idempotent verifies two runs produce
// byte-identical output.
func TestConfigureOpenCodePolicy_Idempotent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	if err := os.WriteFile(configPath, []byte(`{"permission":{"bash":{"*":"allow"}}}`), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := configureOpenCodePolicy(configPath); err != nil {
		t.Fatalf("first run: %v", err)
	}
	first, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read after first: %v", err)
	}

	if err := configureOpenCodePolicy(configPath); err != nil {
		t.Fatalf("second run: %v", err)
	}
	second, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read after second: %v", err)
	}

	if string(first) != string(second) {
		t.Errorf("not idempotent\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

// TestConfigureOpenCodePolicy_ConvertsStringInstructions verifies that when
// instructions is a string (not array), it is converted to an array containing
// the original string plus the GIT_COURER.md path.
func TestConfigureOpenCodePolicy_ConvertsStringInstructions(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	existing := `{"instructions": "other.md"}`
	if err := os.WriteFile(configPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := configureOpenCodePolicy(configPath); err != nil {
		t.Fatalf("configureOpenCodePolicy: %v", err)
	}

	cfg := readOpenCodeConfig(t, configPath)
	arr, ok := cfg["instructions"].([]interface{})
	if !ok {
		t.Fatalf("instructions was not converted to array, got %T", cfg["instructions"])
	}
	foundOriginal := false
	for _, v := range arr {
		if s, ok := v.(string); ok && s == "other.md" {
			foundOriginal = true
		}
	}
	if !foundOriginal {
		t.Error("original string instruction was lost during conversion")
	}
	assertInstructionsContains(t, cfg, gitCourerMdPath(configPath))
}

// TestConfigureOpenCodePolicy_CorruptJSONBackupsAndWritesFresh verifies that
// when opencode.json is unparseable, the original is backed up and a fresh
// config with the policy is written.
func TestConfigureOpenCodePolicy_CorruptJSONBackupsAndWritesFresh(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	corrupt := `{not valid json`
	if err := os.WriteFile(configPath, []byte(corrupt), 0644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}

	if err := configureOpenCodePolicy(configPath); err != nil {
		t.Fatalf("configureOpenCodePolicy: %v", err)
	}

	// Backup must exist with the corrupt content.
	bak, err := os.ReadFile(configPath + ".bak")
	if err != nil {
		t.Fatalf("backup not created: %v", err)
	}
	if string(bak) != corrupt {
		t.Errorf("backup content mismatch:\ngot:  %q\nwant: %q", bak, corrupt)
	}

	// Fresh config must be valid JSON with the policy.
	cfg := readOpenCodeConfig(t, configPath)
	assertBashRule(t, cfg, "git *", "ask")
	assertInstructionsContains(t, cfg, gitCourerMdPath(configPath))
}

// --- removeOpenCodePolicy tests (T1) ---

// TestRemoveOpenCodePolicy_StripsGitStarRule verifies the "git *" rule is
// removed while preserving other permission.bash keys.
func TestRemoveOpenCodePolicy_StripsGitStarRule(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	existing := `{
		"permission": {"bash": {"*": "allow", "git *": "ask"}}
	}`
	if err := os.WriteFile(configPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := removeOpenCodePolicy(configPath); err != nil {
		t.Fatalf("removeOpenCodePolicy: %v", err)
	}

	cfg := readOpenCodeConfig(t, configPath)
	assertBashRule(t, cfg, "*", "allow")
	assertBashRuleAbsent(t, cfg, "git *")
}

// TestRemoveOpenCodePolicy_RemovesGitCourerMdFromInstructions verifies the
// both old GIT_COURER.md path and AGENTS.md path are removed from instructions
// while other entries are preserved.
func TestRemoveOpenCodePolicy_RemovesGitCourerMdFromInstructions(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	other := filepath.Join(dir, "OTHER.md")
	oldRules := filepath.Join(dir, gitCourerMdFilename)
	agents := filepath.Join(dir, "AGENTS.md")
	existing := `{"instructions": ["` + other + `", "` + oldRules + `", "` + agents + `"]}`
	if err := os.WriteFile(configPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := removeOpenCodePolicy(configPath); err != nil {
		t.Fatalf("removeOpenCodePolicy: %v", err)
	}

	cfg := readOpenCodeConfig(t, configPath)
	assertInstructionsAbsent(t, cfg, oldRules)
	assertInstructionsAbsent(t, cfg, agents)
	// Other entry must survive.
	arr, _ := cfg["instructions"].([]interface{})
	foundOther := false
	for _, v := range arr {
		if s, ok := v.(string); ok && s == other {
			foundOther = true
		}
	}
	if !foundOther {
		t.Errorf("other instruction %q was removed", other)
	}
}

// TestRemoveOpenCodePolicy_NoOpWhenNoPolicyEntries verifies the file is not
// rewritten (modtime unchanged) when there are no git-courer policy entries.
func TestRemoveOpenCodePolicy_NoOpWhenNoPolicyEntries(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	original := `{"permission":{"bash":{"*":"allow"}},"instructions":["other.md"]}`
	if err := os.WriteFile(configPath, []byte(original), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	beforeStat, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}

	if err := removeOpenCodePolicy(configPath); err != nil {
		t.Fatalf("removeOpenCodePolicy: %v", err)
	}

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(after) != original {
		t.Errorf("file was modified despite no policy entries\ngot:  %s\nwant: %s", after, original)
	}
	afterStat, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if !afterStat.ModTime().Equal(beforeStat.ModTime()) {
		t.Errorf("file was rewritten — modtime changed: before=%v after=%v", beforeStat.ModTime(), afterStat.ModTime())
	}
}

// TestRemoveOpenCodePolicy_LeavesCorruptJSONUntouchedWhenNoBackup verifies
// that when opencode.json is unparseable AND no backup exists, the file is
// left intact rather than risk clobbering user config.
func TestRemoveOpenCodePolicy_LeavesCorruptJSONUntouchedWhenNoBackup(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	corrupt := `{not valid json`
	if err := os.WriteFile(configPath, []byte(corrupt), 0644); err != nil {
		t.Fatalf("write corrupt: %v", err)
	}

	if err := removeOpenCodePolicy(configPath); err != nil {
		t.Fatalf("removeOpenCodePolicy: %v", err)
	}

	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read after: %v", err)
	}
	if string(after) != corrupt {
		t.Errorf("corrupt file was modified despite no backup\ngot:  %q\nwant: %q", after, corrupt)
	}
}

// TestConfigureOpenCodePolicy_CleansUpCustomPathGitCourerMd verifies that when
// configureOpenCodePolicy runs on an opencode.json containing a custom path
// ending in GIT_COURER.md in the instructions array, it cleans it up and adds
// the new AGENTS.md global path.
func TestConfigureOpenCodePolicy_CleansUpCustomPathGitCourerMd(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "opencode.json")
	customOldRules := filepath.Join(dir, "custom-env", "config", "GIT_COURER.md")
	existing := `{"instructions": ["` + customOldRules + `"]}`
	if err := os.WriteFile(configPath, []byte(existing), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := configureOpenCodePolicy(configPath); err != nil {
		t.Fatalf("configureOpenCodePolicy: %v", err)
	}

	cfg := readOpenCodeConfig(t, configPath)
	assertInstructionsAbsent(t, cfg, customOldRules)
	assertInstructionsContains(t, cfg, gitCourerMdPath(configPath))
}