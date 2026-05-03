// Package installer provides installation and management for git-courer.
package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// SetupProject sets up git-courer in a project directory.
func SetupProject(projectDir string) error {
	// Ensure .gcourer directory
	gcourerDir := filepath.Join(projectDir, ".gcourer")
	if err := os.MkdirAll(gcourerDir, 0755); err != nil {
		return fmt.Errorf("failed to create .gcourer dir: %w", err)
	}

	// Create config.yaml if not exists
	configPath := filepath.Join(gcourerDir, "config.yaml")
	if _, err := os.Stat(configPath); err != nil {
		defaultConfig := `# git-courer configuration — see https://github.com/Alejandro-M-P/git-courer

# ---------------------------------------------------------------------------
# Unified LLM config
# Provider and model are MANDATORY.
# ---------------------------------------------------------------------------
llm:
  provider: ""        # mandatory — ollama | openai-compatible | lmstudio | vllm | localai
  model: ""           # mandatory — model name/identifier
  base_url: http://localhost:11434/v1
  num_parallel: 1

# ---------------------------------------------------------------------------
# Preview / Confirmation settings
# ---------------------------------------------------------------------------
preview:
  enabled: true

# ---------------------------------------------------------------------------
# Git behavior settings
# ---------------------------------------------------------------------------
git:
  workdir: "."

# ---------------------------------------------------------------------------
# Project context (for commit messages and changelogs)
# ---------------------------------------------------------------------------
context:
  project: ""         # mandatory — short project description
  style: concise_technical
`
		if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
	}

	return nil
}

// RemoveProject removes git-courer from a project directory.
func RemoveProject(projectDir string) error {
	gcourerDir := filepath.Join(projectDir, ".gcourer")
	if err := os.RemoveAll(gcourerDir); err != nil {
		return fmt.Errorf("failed to remove .gcourer dir: %w", err)
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
		"/opt/homebrew/bin/git-courer",               // macOS Homebrew (Apple Silicon)
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
