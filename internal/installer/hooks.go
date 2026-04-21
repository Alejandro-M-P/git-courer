// Package installer provides installation and management for git-courer.
package installer

import (
	"fmt"
	"os"
	"path/filepath"
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
		defaultConfig := `ollama:
  host: http://localhost:11434
  model: gemma4:26b
git:
  workdir: .
secrets:
  detection_mode: regex+ai
preview:
  enabled: true
  operations:
    commit: true
    branch_create: true
    release: true
`
		if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
			return fmt.Errorf("failed to write config: %w", err)
		}
	}

	// Setup git hooks
	if err := SetupHooks(projectDir); err != nil {
		return fmt.Errorf("failed to setup hooks: %w", err)
	}

	return nil
}

// SetupHooks creates git hooks for the project.
func SetupHooks(projectDir string) error {
	hooksDir := filepath.Join(projectDir, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return fmt.Errorf("failed to create hooks dir: %w", err)
	}

	preCommitPath := filepath.Join(hooksDir, "pre-commit")
	gitCourerPath, err := FindBinaryPath()
	if err != nil {
		gitCourerPath = "git-courer"
	}

	hookContent := fmt.Sprintf(`#!/bin/sh
# git-courer pre-commit hook
# Run git-courer to check for secrets before commit
command -v %s >/dev/null 2>&1 && %s check-secrets "$@"
`, gitCourerPath, gitCourerPath)

	if err := os.WriteFile(preCommitPath, []byte(hookContent), 0755); err != nil {
		return fmt.Errorf("failed to write pre-commit hook: %w", err)
	}

	return nil
}

// RemoveProject removes git-courer from a project directory.
func RemoveProject(projectDir string) error {
	// Remove .gcourer directory
	gcourerDir := filepath.Join(projectDir, ".gcourer")
	if err := os.RemoveAll(gcourerDir); err != nil {
		return fmt.Errorf("failed to remove .gcourer dir: %w", err)
	}

	// Remove git hooks
	if err := RemoveHooks(projectDir); err != nil {
		return fmt.Errorf("failed to remove hooks: %w", err)
	}

	return nil
}

// RemoveHooks removes git hooks created by git-courer.
func RemoveHooks(projectDir string) error {
	hooksDir := filepath.Join(projectDir, ".git", "hooks")
	preCommitPath := filepath.Join(hooksDir, "pre-commit")

	// Check if file contains git-courer reference
	if data, err := os.ReadFile(preCommitPath); err == nil {
		content := string(data)
		if len(content) > 0 && len(content) > 30 && content[:30] == "#!/bin/sh\n# git-courer pre-commit" {
			os.Remove(preCommitPath)
		}
	}

	return nil
}

// FindBinaryPath tries to find the git-courer binary path.
func FindBinaryPath() (string, error) {
	home := os.Getenv("HOME")
	paths := []string{
		filepath.Join(home, ".local/bin/git-courer"),
		"/usr/local/bin/git-courer",
		"/usr/bin/git-courer",
		filepath.Join(home, "go/bin/git-courer"),
		filepath.Join(home, ".config/git-courer/git-courer"),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("git-courer binary not found — run: git-courer install")
}
