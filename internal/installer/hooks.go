// Package installer provides installation and management for git-courer.
package installer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// gitCourerHookCommand is the command written into a client's hooks.json
// PreToolUse matcher so the agent runs git-courer hook-check before every
// Bash tool call.
const gitCourerHookCommand = "git-courer hook-check"

// hooksFilePayload is the JSON shape of a Codex-style hooks.json file.
type hooksFilePayload struct {
	Hooks map[string][]map[string]interface{} `json:"hooks"`
}

// hookEntryMatchesGitCourer reports whether a single hook entry map is the
// git-courer PreToolUse hook (by command).
func hookEntryMatchesGitCourer(entry map[string]interface{}) bool {
	cmd, _ := entry["command"].(string)
	return cmd == gitCourerHookCommand
}

// installHook writes (or merges into) the client's hooks.json a PreToolUse
// hook with matcher "Bash" that runs `git-courer hook-check`.
//
// Behavior:
//   - If hooks.json does not exist, it is created with the git-courer entry.
//   - If hooks.json exists and already contains the git-courer entry, it is
//     left unchanged (idempotent).
//   - If hooks.json exists without the git-courer entry, a .bak backup is
//     created first and the git-courer entry is merged in, preserving any
//     other existing hooks.
//
// The client's HooksConfig must be non-nil; otherwise installHook returns an
// error.
func installHook(client *MCPClient) error {
	if client == nil || client.HooksConfig == nil {
		return fmt.Errorf("installHook: client has no HooksConfig")
	}
	hooksPath := client.HooksConfig.Path

	if err := os.MkdirAll(filepath.Dir(hooksPath), 0755); err != nil {
		return fmt.Errorf("installHook: create hooks dir: %w", err)
	}

	payload := hooksFilePayload{Hooks: map[string][]map[string]interface{}{}}

	// If the file already exists, load it and check for the git-courer entry.
	if data, err := os.ReadFile(hooksPath); err == nil {
		if jsonErr := json.Unmarshal(data, &payload); jsonErr == nil {
			for _, entry := range payload.Hooks["PreToolUse"] {
				if hookEntryMatchesGitCourer(entry) {
					// Already installed — idempotent no-op.
					return nil
				}
			}
		}
		// File exists but does not contain git-courer hook → backup before merge.
		if backupErr := backupConfig(hooksPath); backupErr != nil {
			return fmt.Errorf("installHook: backup hooks.json: %w", backupErr)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("installHook: read hooks.json: %w", err)
	}

	// Append the git-courer PreToolUse hook.
	payload.Hooks["PreToolUse"] = append(payload.Hooks["PreToolUse"], map[string]interface{}{
		"matcher": "Bash",
		"command": gitCourerHookCommand,
	})

	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("installHook: marshal hooks.json: %w", err)
	}
	if err := os.WriteFile(hooksPath, out, 0644); err != nil {
		return fmt.Errorf("installHook: write hooks.json: %w", err)
	}
	return nil
}

// removeHook strips the git-courer PreToolUse entry from the client's
// hooks.json. If no hooks remain after removal, the file is deleted. If the
// file does not exist or does not contain the git-courer entry, removeHook is
// a no-op. The client's HooksConfig must be non-nil.
func removeHook(client *MCPClient) error {
	if client == nil || client.HooksConfig == nil {
		return fmt.Errorf("removeHook: client has no HooksConfig")
	}
	hooksPath := client.HooksConfig.Path

	data, err := os.ReadFile(hooksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // nothing to remove
		}
		return fmt.Errorf("removeHook: read hooks.json: %w", err)
	}

	var payload hooksFilePayload
	if jsonErr := json.Unmarshal(data, &payload); jsonErr != nil {
		return fmt.Errorf("removeHook: parse hooks.json: %w", jsonErr)
	}

	pre, ok := payload.Hooks["PreToolUse"]
	if !ok {
		return nil // no PreToolUse hooks at all
	}

	filtered := pre[:0]
	removed := false
	for _, entry := range pre {
		if hookEntryMatchesGitCourer(entry) {
			removed = true
			continue
		}
		filtered = append(filtered, entry)
	}
	if !removed {
		return nil // git-courer hook was not present
	}

	// Rebuild payload without empty slices.
	cleanPayload := hooksFilePayload{Hooks: map[string][]map[string]interface{}{}}
	for event, entries := range payload.Hooks {
		if event == "PreToolUse" {
			entries = filtered
		}
		if len(entries) > 0 {
			cleanPayload.Hooks[event] = entries
		}
	}

	if len(cleanPayload.Hooks) == 0 {
		// No hooks remain → delete the file.
		if err := os.Remove(hooksPath); err != nil {
			return fmt.Errorf("removeHook: delete empty hooks.json: %w", err)
		}
		return nil
	}

	out, err := json.MarshalIndent(cleanPayload, "", "  ")
	if err != nil {
		return fmt.Errorf("removeHook: marshal hooks.json: %w", err)
	}
	if err := os.WriteFile(hooksPath, out, 0644); err != nil {
		return fmt.Errorf("removeHook: write hooks.json: %w", err)
	}
	return nil
}

// hooksStatus reports the real installation state of the git-courer hook for
// the given client.
//
// Returns:
//   - "installed"     if hooks.json exists and contains the git-courer
//                      PreToolUse entry.
//   - "not_installed" otherwise (file missing, unreadable, or lacks the
//                      git-courer entry).
func hooksStatus(client *MCPClient) string {
	if client == nil || client.HooksConfig == nil {
		return "not_installed"
	}
	hooksPath := client.HooksConfig.Path

	data, err := os.ReadFile(hooksPath)
	if err != nil {
		return "not_installed"
	}
	var payload hooksFilePayload
	if jsonErr := json.Unmarshal(data, &payload); jsonErr != nil {
		return "not_installed"
	}
	for _, entry := range payload.Hooks["PreToolUse"] {
		if hookEntryMatchesGitCourer(entry) {
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
