// Package installer provides installation and management for git-courer.
package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

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
