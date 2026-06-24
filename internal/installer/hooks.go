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
const goldenRulesAdditionalContext = `## git-courer Golden Rules

1. BEFORE any mutation → status
2. BEFORE push → diff + review
3. BEFORE PR → pr-review

Use git-courer MCP tools instead of raw bash git.`

// hooksJSON represents the structure of a Codex hooks.json file.
type hooksJSON struct {
	Hooks map[string][]hookEntry `json:"hooks"`
}

type hookEntry struct {
	Matcher string      `json:"matcher"`
	Hooks   []hookCmd   `json:"hooks"`
}

type hookCmd struct {
	Type    string `json:"type"`
	Command string `json:"command"`
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
