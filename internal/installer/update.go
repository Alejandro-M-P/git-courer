// Package installer provides installation and management for git-courer.
package installer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	selfupdate "github.com/creativeprojects/go-selfupdate"
)

// CheckForUpdates checks if a new version is available using go-selfupdate.
// Uses GitHub source with platform-specific filter regex to match assets like
// git-courer_{version}_{OS}_{Arch}.tar.gz
func CheckForUpdates() (bool, string, error) {
	return checkForUpdatesWithFactory(defaultUpdaterFactory)
}

// checkForUpdatesWithFactory is the testable core function that accepts an updater factory.
func checkForUpdatesWithFactory(factory UpdaterFactory) (bool, string, error) {
	updater, err := factory()
	if err != nil {
		return false, "", fmt.Errorf("failed to create updater: %w", err)
	}

	repo := selfupdate.NewRepositorySlug("Alejandro-M-P", "git-courer")
	latest, found, err := updater.DetectLatest(context.Background(), repo)
	if err != nil {
		return false, "", fmt.Errorf("failed to detect latest release: %w", err)
	}
	if !found {
		return false, "", fmt.Errorf("no release found")
	}

	currentVersion := getCurrentVersion()
	newVersion := strings.TrimPrefix(latest.Version(), "v")

	if newVersion != currentVersion {
		return true, newVersion, nil
	}

	return false, currentVersion, nil
}

// DownloadUpdate downloads and installs the latest version using go-selfupdate.
// Relies on go-selfupdate's UpdateTo() which handles download, extraction,
// and binary replacement atomically. Post-update steps (MCP reconfiguration
// and rule file updates) only execute after successful binary replacement.
func DownloadUpdate() error {
	return downloadUpdateWithFactory(defaultUpdaterFactory)
}

// downloadUpdateWithFactory is the testable core function that accepts an updater factory.
func downloadUpdateWithFactory(factory UpdaterFactory) error {
	updater, err := factory()
	if err != nil {
		return fmt.Errorf("failed to create updater: %w", err)
	}

	repo := selfupdate.NewRepositorySlug("Alejandro-M-P", "git-courer")
	latest, found, err := updater.DetectLatest(context.Background(), repo)
	if err != nil {
		return fmt.Errorf("failed to detect latest release: %w", err)
	}
	if !found {
		return fmt.Errorf("no release found")
	}

	// Get current executable path
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Use selfupdate to download, extract, and replace binary
	// If this succeeds, post-update steps execute; if it fails, they are skipped
	if err := updater.UpdateTo(context.Background(), latest, currentPath); err != nil {
		return fmt.Errorf("failed to update binary: %w", err)
	}

	// After successful update, reconfigure MCP clients
	binPath, _ := FindBinaryPath()
	if configured, err := ConfigureAllMCP(binPath); err != nil {
		fmt.Fprintf(os.Stderr, "  MCP setup: %v\n", err)
	} else if configured > 0 {
		fmt.Printf("  %d MCP client(s) reconfigured\n", configured)
	}

	// Update rule files
	written, err := WriteRuleFiles(binPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Rule files: %v\n", err)
	} else if written > 0 {
		fmt.Printf("  %d rule file(s) updated\n", written)
	}

	return nil
}

func getCurrentVersion() string {
	return config.ServerVersion
}

// platformToAssetPattern returns a regex pattern matching GitHub assets for the given platform.
// Asset naming convention: git-courer_{version}_{OS}_{Arch}.tar.gz
func platformToAssetPattern(platform *Platform) string {
	if platform == nil {
		return ""
	}
	// Escape special regex characters in OS and arch (they shouldn't have any, but be safe)
	osPattern := regexp.QuoteMeta(string(platform.OS))
	archPattern := regexp.QuoteMeta(platform.Arch)
	return fmt.Sprintf("git-courer_.*_%s_%s\\.tar\\.gz", osPattern, archPattern)
}

// UpdaterFactory defines a factory for creating selfupdate.Updater instances.
// Used for dependency injection in tests.
type UpdaterFactory func() (*selfupdate.Updater, error)

// defaultUpdaterFactory is the production factory that creates a real updater.
var defaultUpdaterFactory UpdaterFactory = func() (*selfupdate.Updater, error) {
	platform := Detect()
	if platform == nil {
		return nil, fmt.Errorf("failed to detect platform")
	}

	filterPattern := platformToAssetPattern(platform)
	if filterPattern == "" {
		return nil, fmt.Errorf("failed to create filter pattern")
	}

	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub source: %w", err)
	}

	config := selfupdate.Config{
		Source:  source,
		Filters: []string{filterPattern},
	}

	return selfupdate.NewUpdater(config)
}

// FetchVersion fetches the current version from the binary.
func FetchVersion() (string, error) {
	cmd := exec.Command("git-courer", "version")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
