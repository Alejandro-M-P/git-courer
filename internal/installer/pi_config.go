package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// piPermissionsPath returns the path to the Pi agent settings.json that hosts
// the @pi-lab/permissions rules. Pi stores agent settings under
// ~/.pi/agent/settings.json (the same file PostInstallNotice already reads
// for the pi-mcp-adapter package list).
func piPermissionsPath() string {
	return filepath.Join(homeDirFn(), ".pi/agent/settings.json")
}

// piPermissionRule is one entry of @pi-lab/permissions "permissions.rules"
// array. Match identifies the tool and command to gate; Action is "deny" or
// "ask"; Reason is the human-readable explanation shown to the agent.
type piPermissionRule struct {
	Match  piPermissionMatch `json:"match"`
	Action string            `json:"action"`
	Reason string            `json:"reason"`
}

// piPermissionMatch identifies the tool and command a rule applies to.
type piPermissionMatch struct {
	Tool   string             `json:"tool"`
	Params piPermissionParams `json:"params"`
}

// piPermissionParams carries the command string a Bash rule matches on.
type piPermissionParams struct {
	Command string `json:"command"`
}

// piDenySubcommands is the ordered list of the 23 git subcommands git-courer
// covers with an MCP tool. Each becomes a deny rule for "git {sub}". The list
// mirrors openCodeDenySubcommands so every client enforces the same coverage.
var piDenySubcommands = []string{
	"status", "diff", "commit", "log", "branch", "merge", "rebase",
	"cherry-pick", "revert", "reset", "stash", "push", "pull", "fetch",
	"blame", "add", "restore", "clean", "rm", "switch", "checkout",
	"worktree", "reflog",
}

// piMCPToolForSubcommand maps a git subcommand to the git-courer MCP tool name
// used in the deny reason. It mirrors the classifier mcpTools map so the
// reason text is consistent across clients.
var piMCPToolForSubcommand = map[string]string{
	"status":      "status",
	"diff":        "diff",
	"commit":      "commit",
	"log":         "history",
	"branch":      "branch",
	"merge":       "integrate",
	"rebase":      "integrate",
	"cherry-pick": "integrate",
	"revert":      "rewrite",
	"reset":       "rewrite",
	"stash":       "stash",
	"push":        "sync",
	"pull":        "sync",
	"fetch":       "sync",
	"blame":       "history",
	"add":         "stage",
	"restore":     "stage",
	"clean":       "stage",
	"rm":          "stage",
	"switch":      "branch",
	"checkout":    "branch",
	"worktree":    "branch",
	"reflog":      "history",
}

// piGitCourerRuleReasons is the set of reason strings git-courer writes into
// @pi-lab/permissions rules. Used to identify and strip git-courer-owned
// rules on uninstall without clobbering user-defined rules.
var piGitCourerRuleReasons = func() map[string]bool {
	m := make(map[string]bool, len(piDenySubcommands)+1)
	for _, sub := range piDenySubcommands {
		m["Use git-courer/"+piMCPToolForSubcommand[sub]+" instead"] = true
	}
	m["Use git-courer tools instead of raw git"] = true
	return m
}()

// installPiPermissions writes @pi-lab/permissions rules into the Pi agent
// settings.json at settingsPath. It adds one deny rule per covered git
// subcommand (23 rules) plus a single "ask" fallback for any other "git"
// command. Existing non-git-courer rules and every other top-level settings
// key are preserved.
//
// Behavior:
//   - If settings.json does not exist, a fresh file with the git-courer
//     rules is created (no backup).
//   - If settings.json exists, it is backed up to settingsPath + ".gc.bak"
//     before the first mutation. The backup is NOT overwritten on re-run.
//   - If settings.json exists but is unparseable JSON, it is backed up and a
//     fresh file with the git-courer rules is written.
//   - Idempotent: running twice produces byte-identical settings.json.
func installPiPermissions(settingsPath, binPath string) error {
	_ = binPath // reserved for future rule customization; not used today

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return fmt.Errorf("failed to create pi settings dir: %w", err)
	}

	// Read existing settings.json into a generic map to preserve every
	// top-level key we do not own (packages, model, etc.).
	settings := make(map[string]interface{})
	fileExisted := false
	parsedOK := false
	if data, err := os.ReadFile(settingsPath); err == nil {
		fileExisted = true
		if jsonErr := json.Unmarshal(data, &settings); jsonErr != nil {
			// Unparseable — back up the original bytes and start fresh so we
			// can still apply the rules without clobbering user config.
			if backupErr := backupPiSettings(settingsPath, data); backupErr != nil {
				return fmt.Errorf("failed to backup unparseable pi settings: %w", backupErr)
			}
			settings = make(map[string]interface{})
		} else {
			parsedOK = true
		}
	}

	// Backup existing valid settings before mutation (only if it parsed and
	// no backup already exists from a prior install — preserves idempotency).
	if fileExisted && parsedOK {
		bakPath := settingsPath + ".gc.bak"
		if _, err := os.Stat(bakPath); os.IsNotExist(err) {
			if backupErr := backupPiSettings(settingsPath, nil); backupErr != nil {
				return fmt.Errorf("failed to backup pi settings: %w", backupErr)
			}
		}
	}

	// permissions.rules — merge git-courer rules.
	perm, _ := settings["permissions"].(map[string]interface{})
	if perm == nil {
		perm = make(map[string]interface{})
		settings["permissions"] = perm
	}
	rules, _ := perm["rules"].([]interface{})

	// Build the git-courer ruleset: 23 deny rules + 1 ask fallback.
	gcRules := piBuildGitCourerRules()

	// Merge: drop any existing git-courer-owned rules first (so re-runs are
	// idempotent), then append the fresh git-courer rules, preserving
	// user-defined rules in their original order.
	merged := piMergeRules(rules, gcRules)
	if len(merged) == 0 {
		delete(perm, "rules")
	} else {
		perm["rules"] = merged
	}
	// Drop the permissions object entirely if it is now empty.
	if len(perm) == 0 {
		delete(settings, "permissions")
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal pi settings.json: %w", err)
	}
	return writeAtomic(settingsPath, data, 0644)
}

// removePiPermissions strips the git-courer permission rules from the Pi
// agent settings.json at settingsPath. Behavior:
//   - If settingsPath + ".gc.bak" exists, it is restored over settingsPath and
//     the .bak file is removed.
//   - Otherwise the git-courer rules are removed in place from
//     permissions.rules, preserving user-defined rules and every other
//     top-level settings key.
//   - Idempotent: running twice does not error.
func removePiPermissions(settingsPath string) error {
	bakPath := settingsPath + ".gc.bak"
	if _, err := os.Stat(bakPath); err == nil {
		bakData, err := os.ReadFile(bakPath)
		if err != nil {
			return fmt.Errorf("failed to read pi settings backup: %w", err)
		}
		if err := writeAtomic(settingsPath, bakData, 0644); err != nil {
			return fmt.Errorf("failed to restore pi settings backup: %w", err)
		}
		_ = os.Remove(bakPath)
		return nil
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		// No file — nothing to remove.
		return nil
	}

	settings := make(map[string]interface{})
	if err := json.Unmarshal(data, &settings); err != nil {
		// Unparseable — leave it alone rather than risk clobbering user config.
		return nil
	}

	perm, ok := settings["permissions"].(map[string]interface{})
	if !ok {
		return nil // no permissions section — nothing to strip
	}

	rules, ok := perm["rules"].([]interface{})
	if !ok {
		return nil // no rules — nothing to strip
	}

	changed := false
	var kept []interface{}
	for _, v := range rules {
		rm, ok := v.(map[string]interface{})
		if !ok {
			kept = append(kept, v)
			continue
		}
		if reason, ok := rm["reason"].(string); ok && piGitCourerRuleReasons[reason] {
			changed = true
			continue // drop git-courer rule
		}
		kept = append(kept, v)
	}

	if !changed {
		return nil // no git-courer rules — leave the file untouched
	}

	if len(kept) == 0 {
		delete(perm, "rules")
	} else {
		perm["rules"] = kept
	}
	if len(perm) == 0 {
		delete(settings, "permissions")
	}

	// If the settings map is now empty, delete the file entirely.
	if len(settings) == 0 {
		_ = os.Remove(settingsPath)
		return nil
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cleaned pi settings.json: %w", err)
	}
	if string(out) == "{}" {
		_ = os.Remove(settingsPath)
		return nil
	}
	return writeAtomic(settingsPath, out, 0644)
}

// backupPiSettings copies settingsPath to settingsPath + ".gc.bak" so
// removePiPermissions can restore it. If extra is non-nil it is written
// instead of reading the file (used for the unparseable case).
func backupPiSettings(settingsPath string, extra []byte) error {
	var data []byte
	var err error
	if extra != nil {
		data = extra
	} else {
		data, err = os.ReadFile(settingsPath)
		if err != nil {
			return err
		}
	}
	return os.WriteFile(settingsPath+".gc.bak", data, 0644)
}

// piBuildGitCourerRules returns the ordered list of @pi-lab/permissions rules
// git-courer installs: 23 deny rules (one per covered git subcommand) followed
// by a single "ask" fallback for any other "git" command.
func piBuildGitCourerRules() []piPermissionRule {
	rules := make([]piPermissionRule, 0, len(piDenySubcommands)+1)
	for _, sub := range piDenySubcommands {
		tool := piMCPToolForSubcommand[sub]
		rules = append(rules, piPermissionRule{
			Match: piPermissionMatch{
				Tool:   "bash",
				Params: piPermissionParams{Command: "git " + sub},
			},
			Action: "deny",
			Reason: "Use git-courer/" + tool + " instead",
		})
	}
	// Ask fallback for any other raw git invocation.
	rules = append(rules, piPermissionRule{
		Match: piPermissionMatch{
			Tool:   "bash",
			Params: piPermissionParams{Command: "git"},
		},
		Action: "ask",
		Reason: "Use git-courer tools instead of raw git",
	})
	return rules
}

// piMergeRules merges git-courer rules into an existing rules slice. Existing
// git-courer-owned rules (identified by their reason string) are dropped
// first so re-runs are idempotent; user-defined rules are preserved in their
// original order, followed by the fresh git-courer rules.
func piMergeRules(existing []interface{}, gcRules []piPermissionRule) []interface{} {
	// Drop stale git-courer rules from existing.
	cleaned := make([]interface{}, 0, len(existing))
	for _, v := range existing {
		rm, ok := v.(map[string]interface{})
		if !ok {
			cleaned = append(cleaned, v)
			continue
		}
		if reason, ok := rm["reason"].(string); ok && piGitCourerRuleReasons[reason] {
			continue // drop stale git-courer rule
		}
		cleaned = append(cleaned, v)
	}
	// Append the fresh git-courer rules as generic maps.
	for _, r := range gcRules {
		// Marshal then unmarshal to convert the typed struct into a
		// map[string]interface{} consistent with the existing slice shape.
		b, err := json.Marshal(r)
		if err != nil {
			continue
		}
		var m interface{}
		if err := json.Unmarshal(b, &m); err != nil {
			continue
		}
		cleaned = append(cleaned, m)
	}
	return cleaned
}
