// Package installer provides installation and management for git-courer.
package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// goldenRulesAdditionalContext is the markdown content returned by
// session-start-hook and subagent-start-hook as additionalContext.
const GoldenRulesAdditionalContext = `## git-courer Golden Rules

0. On session start (MANDATORY) → ALWAYS run ` + "`git-courer session start`" + ` first for any change, even the smallest one. This creates an isolated worktree + branch so you never work directly on main.
1. After start (MANDATORY) → ALWAYS run ` + "`git-courer session select session_id=<id>`" + ` to point all subsequent git commands at the session's worktree. Without this, commands run on the main repo instead of the isolated worktree.
2. Workspace Isolation (MANDATORY) → ALWAYS perform all code modifications, terminal commands, and tests inside the designated workspace directory (worktree) created by ` + "`git-courer session start`" + `, NEVER directly in the main repository root.
3. Before Commit → ALWAYS check ` + "`git-courer status`" + ` to know active files and ` + "`git-courer diff`" + ` to verify your changes.
4. Committing → Stage your changes using ` + "`git-courer stage`" + ` and commit using ` + "`git-courer commit`" + ` (or ` + "`git-courer integrate`" + ` for automated checks).
5. Pre-merge Verification → ALWAYS run ` + "`git-courer pr-review`" + ` to run all validation checks in the workspace before closing.
6. Session Closure → Run ` + "`git-courer session finish`" + ` to perform final verification, merge the branch into main, and clean up the workspace.`

// hooksJSON represents the structure of a Codex hooks.json file.
type hooksJSON struct {
	Hooks map[string][]hookEntry `json:"hooks"`
}

type hookEntry struct {
	Matcher string    `json:"matcher"`
	Hooks   []hookCmd `json:"hooks"`
}

type hookCmd struct {
	Type    string `json:"type"`
	Command string `json:"command"`
}

// claudeSettings represents the structure of a Claude Code settings.json
// file. Claude Code stores hooks inline in the "hooks" object (keyed by event
// name) rather than in a separate hooks file like Codex.
type claudeSettings struct {
	Hooks map[string][]claudeHookEntry `json:"hooks,omitempty"`
	// Other top-level settings keys are preserved via the generic decoder used
	// by installClaudeHooks/removeClaudeHooks in mcp_config.go; this struct is
	// only used by claudeHooksStatus for a lightweight read.
}

// claudeHookEntry is one matcher group inside a Claude Code hooks event.
type claudeHookEntry struct {
	Matcher string          `json:"matcher"`
	Hooks   []claudeHookCmd `json:"hooks"`
}

// claudeHookCmd is a single hook command inside a Claude Code hook entry.
// Claude Code supports the exec form (type "command" with separate args) and
// an optional timeout in seconds.
type claudeHookCmd struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Timeout int      `json:"timeout,omitempty"`
}

// claudeGitCourerHookEvents is the set of hook events git-courer installs
// into Claude Code settings.json. Used by mergeClaudeHooks and
// claudeHooksStatus to know what a complete install looks like.
var claudeGitCourerHookEvents = []string{
	"PreToolUse",
	"SessionStart",
	"SubagentStart",
	"PreInvocation",
}

// mergeClaudeHooks merges the git-courer hook entries in gitcourer into the
// existing hooks map, preserving every non-git-courer hook. Matching is by
// (matcher, command containing "git-courer"):
//   - If an existing entry for the same event has the same matcher AND a hook
//     command containing "git-courer", the git-courer command is updated in
//     place (handles binary path changes) instead of appended.
//   - If an existing entry for the same event has the same matcher but NO
//     git-courer command, the git-courer hook is appended to that entry's
//     hooks list (so git-courer runs alongside any other hook for that
//     matcher).
//   - If no existing entry for the same event has the same matcher, a new
//     entry is appended.
//
// existing may be nil — a fresh map is returned with only the git-courer
// entries. The returned map is always non-nil when gitcourer is non-empty.
func mergeClaudeHooks(existing, gitcourer map[string][]claudeHookEntry) map[string][]claudeHookEntry {
	if existing == nil {
		existing = make(map[string][]claudeHookEntry)
	}
	for event, newEntries := range gitcourer {
		for _, newEntry := range newEntries {
			merged := false
			for i, oldEntry := range existing[event] {
				if oldEntry.Matcher != newEntry.Matcher {
					continue
				}
				// Same matcher — update or append the git-courer hook command.
				updated := false
				for j, cmd := range existing[event][i].Hooks {
					if strings.Contains(cmd.Command, "git-courer") {
						existing[event][i].Hooks[j] = newEntry.Hooks[0]
						updated = true
						break
					}
				}
				if !updated {
					existing[event][i].Hooks = append(existing[event][i].Hooks, newEntry.Hooks[0])
				}
				merged = true
				break
			}
			if !merged {
				existing[event] = append(existing[event], newEntry)
			}
		}
	}
	return existing
}

// claudeHooksStatus returns the installation status of git-courer hooks in
// the Claude Code settings.json at settingsPath:
//   - "installed"     — all three git-courer hook events are present with a
//                       git-courer command for the expected matcher.
//   - "not_installed" — none of the git-courer hook events are present.
//   - "partial"       — some but not all git-courer hook events are present.
//
// A read or parse error is reported as "not_installed" (same convention as
// hooksStatus for Codex).
func claudeHooksStatus(settingsPath string) string {
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return "not_installed"
	}

	var settings claudeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return "not_installed"
	}
	if settings.Hooks == nil {
		return "not_installed"
	}

	// Expected (event, matcher) pairs installed by git-courer.
	expected := map[string]string{
		"PreToolUse":    "Bash",
		"SessionStart":  "startup|resume",
		"SubagentStart": "general-purpose|Explore|Plan",
		"PreInvocation": "",
	}

	found := 0
	for event, matcher := range expected {
		for _, entry := range settings.Hooks[event] {
			if entry.Matcher != matcher {
				continue
			}
			for _, cmd := range entry.Hooks {
				if strings.Contains(cmd.Command, "git-courer") {
					found++
					break
				}
			}
		}
	}

	switch found {
	case 0:
		return "not_installed"
	case len(expected):
		return "installed"
	default:
		return "partial"
	}
}

// installHook creates or updates hooks.json at hooksPath with PreToolUse,
// SessionStart, and SubagentStart entries pointing to binPath.
// It backs up existing hooks.json before mutation.
// Idempotent: skips entries that already have the same matcher+command.
func installHook(hooksPath, binPath string) error {
	// Backup existing hooks.json before mutation.
	if _, err := os.Stat(hooksPath); err == nil {
		data, err := os.ReadFile(hooksPath)
		if err != nil {
			return fmt.Errorf("failed to read existing hooks.json: %w", err)
		}
		if err := os.WriteFile(hooksPath+".bak", data, 0644); err != nil {
			return fmt.Errorf("failed to backup hooks.json: %w", err)
		}
	}

	// Read existing hooks or start fresh.
	hooks := hooksJSON{Hooks: make(map[string][]hookEntry)}
	if data, err := os.ReadFile(hooksPath); err == nil {
		_ = json.Unmarshal(data, &hooks)
		if hooks.Hooks == nil {
			hooks.Hooks = make(map[string][]hookEntry)
		}
	}

	// Define the hooks to install.
	entries := []struct {
		event   string
		matcher string
		command string
	}{
		{event: "PreToolUse", matcher: "Bash", command: binPath + " hook-check"},
		{event: "SessionStart", matcher: "startup|resume", command: binPath + " session-start-hook"},
		{event: "SubagentStart", matcher: "general-purpose|Explore|Plan", command: binPath + " subagent-start-hook"},
		{event: "PreInvocation", matcher: "", command: binPath + " pre-invocation-hook"},
	}

	for _, e := range entries {
		// Check if this matcher+command already exists (idempotent).
		existing := hooks.Hooks[e.event]
		found := false
		for _, entry := range existing {
			if entry.Matcher == e.matcher && len(entry.Hooks) > 0 && entry.Hooks[0].Command == e.command {
				found = true
				break
			}
		}
		if found {
			continue
		}

		// Add the hook entry.
		hooks.Hooks[e.event] = append(hooks.Hooks[e.event], hookEntry{
			Matcher: e.matcher,
			Hooks: []hookCmd{
				{Type: "command", Command: e.command},
			},
		})
	}

	data, err := json.MarshalIndent(hooks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hooks.json: %w", err)
	}
	return os.WriteFile(hooksPath, data, 0644)
}

// RemoveHook deletes hooks.json and restores .bak if it exists.
func RemoveHook(hooksPath string) error {
	// Try to restore .bak first.
	bakPath := hooksPath + ".bak"
	if _, err := os.Stat(bakPath); err == nil {
		data, err := os.ReadFile(bakPath)
		if err != nil {
			return fmt.Errorf("failed to read backup: %w", err)
		}
		if err := os.WriteFile(hooksPath, data, 0644); err != nil {
			return fmt.Errorf("failed to restore backup: %w", err)
		}
		_ = os.Remove(bakPath)
		return nil
	}

	// No backup — just remove hooks.json.
	if err := os.Remove(hooksPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove hooks.json: %w", err)
	}
	return nil
}

// hooksStatus returns "installed" if hooks.json exists with a git-courer
// PreToolUse entry, or "not_installed" otherwise.
func hooksStatus(hooksPath string) string {
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		return "not_installed"
	}

	var hooks hooksJSON
	if err := json.Unmarshal(data, &hooks); err != nil {
		return "not_installed"
	}

	entries, ok := hooks.Hooks["PreToolUse"]
	if !ok {
		return "not_installed"
	}

	for _, entry := range entries {
		if entry.Matcher == "Bash" && len(entry.Hooks) > 0 && strings.Contains(entry.Hooks[0].Command, "git-courer") {
			return "installed"
		}
	}

	return "not_installed"
}

// installAntigravityHooks creates or updates hooks.json at hooksPath with
// PreToolUse (matcher run_command) and PreInvocation entries pointing to
// binPath. It backs up an existing hooks.json to hooksPath + ".bak" before
// the first mutation and merges git-courer entries into any existing hooks
// while preserving non-git-courer entries.
//
// Behavior:
//   - If hooks.json does not exist, a fresh file is created (no backup).
//   - If hooks.json exists, it is backed up to hooksPath + ".bak" before any
//     mutation. The backup is only written when the file is about to change
//     for the first time.
//   - Idempotent: running twice produces byte-identical hooks.json. The
//     backup is not overwritten on re-run.
func installAntigravityHooks(hooksPath, binPath string) error {
	if err := os.MkdirAll(filepath.Dir(hooksPath), 0755); err != nil {
		return fmt.Errorf("failed to create hooks dir: %w", err)
	}

	// Read existing hooks.json (if any) and back it up before the first mutation.
	fileExisted := false
	hooks := hooksJSON{Hooks: make(map[string][]hookEntry)}
	if data, err := os.ReadFile(hooksPath); err == nil {
		fileExisted = true
		_ = json.Unmarshal(data, &hooks)
		if hooks.Hooks == nil {
			hooks.Hooks = make(map[string][]hookEntry)
		}
		// Backup before mutation.
		if backupErr := os.WriteFile(hooksPath+".bak", data, 0644); backupErr != nil {
			return fmt.Errorf("failed to backup hooks.json: %w", backupErr)
		}
	}

	// Define the Antigravity hook entries: PreToolUse with run_command matcher
	// (Antigravity uses run_command, not Bash) and PreInvocation.
	entries := []struct {
		event   string
		matcher string
		command string
	}{
		{event: "PreToolUse", matcher: "run_command", command: binPath + " hook-check"},
		{event: "PreInvocation", matcher: "", command: binPath + " pre-invocation-hook"},
	}

	for _, e := range entries {
		existing := hooks.Hooks[e.event]
		found := false
		for _, entry := range existing {
			if entry.Matcher == e.matcher && len(entry.Hooks) > 0 && entry.Hooks[0].Command == e.command {
				found = true
				break
			}
		}
		if found {
			continue
		}
		hooks.Hooks[e.event] = append(hooks.Hooks[e.event], hookEntry{
			Matcher: e.matcher,
			Hooks: []hookCmd{
				{Type: "command", Command: e.command},
			},
		})
	}

	data, err := json.MarshalIndent(hooks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal hooks.json: %w", err)
	}

	// Write atomically so a partial write never corrupts the real hooks file.
	if fileExisted {
		return writeAtomic(hooksPath, data, 0644)
	}
	return os.WriteFile(hooksPath, data, 0644)
}

// removeAntigravityHooks removes the git-courer hook entries from hooks.json
// at hooksPath. Behavior:
//   - If hooksPath + ".bak" exists, it is restored over hooksPath and the
//     .bak file is removed (same convention as RemoveHook for Codex).
//   - Otherwise hooks.json is deleted entirely (the Antigravity hooks.json
//     is fully owned by git-courer when no pre-existing user hooks were
//     preserved by a backup).
//   - Idempotent: running twice does not error.
func removeAntigravityHooks(hooksPath string) error {
	bakPath := hooksPath + ".bak"
	if _, err := os.Stat(bakPath); err == nil {
		data, err := os.ReadFile(bakPath)
		if err != nil {
			return fmt.Errorf("failed to read backup: %w", err)
		}
		if err := writeAtomic(hooksPath, data, 0644); err != nil {
			return fmt.Errorf("failed to restore backup: %w", err)
		}
		_ = os.Remove(bakPath)
		return nil
	}

	// No backup — delete hooks.json if it exists.
	if err := os.Remove(hooksPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove hooks.json: %w", err)
	}
	return nil
}

// FindBinaryPath tries to find the git-courer binary path.
// Supports Linux, macOS, and Windows.
func FindBinaryPath() (string, error) {
	var paths []string

	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE") // Windows fallback
	}

	// Cross-platform paths
	paths = append(paths,
		filepath.Join(home, ".local", "bin", "git-courer"),
		filepath.Join(home, "go", "bin", "git-courer"),
		filepath.Join(home, ".config", "git-courer", "git-courer"),
		"/usr/local/bin/git-courer",
		"/usr/bin/git-courer",
		"/opt/homebrew/bin/git-courer",             // macOS Homebrew (Apple Silicon)
		"/usr/local/opt/git-courer/bin/git-courer", // macOS Homebrew (Intel)
	)

	// Windows-specific paths
	if runtime.GOOS == "windows" || os.Getenv("OS") == "Windows_NT" {
		paths = append(paths,
			filepath.Join(home, "AppData", "Local", "git-courer", "git-courer.exe"),
			filepath.Join(home, "scoop", "shims", "git-courer.exe"),
			filepath.Join(home, "chocolatey", "bin", "git-courer.exe"),
			`C:\Program Files\git-courer\git-courer.exe`,
		)
	}

	// Check PATH environment variable
	if pathEnv := os.Getenv("PATH"); pathEnv != "" {
		pathDirs := filepath.SplitList(pathEnv)
		for _, dir := range pathDirs {
			paths = append(paths, filepath.Join(dir, "git-courer"))
			paths = append(paths, filepath.Join(dir, "git-courer.exe"))
		}
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("git-courer binary not found — run: git-courer install or add to PATH")
}
