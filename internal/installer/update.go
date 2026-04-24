// Package installer provides installation and management for git-courer.
package installer

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	selfupdate "github.com/creativeprojects/go-selfupdate"
)

// CheckForUpdates checks if a new version is available.
func CheckForUpdates() (bool, string, error) {
	ctx := context.Background()
	
	src, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return false, "", fmt.Errorf("failed to create GitHub source: %w", err)
	}
	
	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source: src,
	})
	if err != nil {
		return false, "", fmt.Errorf("failed to create updater: %w", err)
	}
	
	release, found, err := updater.DetectLatest(ctx, selfupdate.NewRepositorySlug("Alejandro-M-P", "git-courer"))
	if err != nil {
		return false, "", fmt.Errorf("failed to detect latest version: %w", err)
	}
	if !found {
		return false, "", fmt.Errorf("no release found for Alejandro-M-P/git-courer")
	}
	
	currentVersion := getCurrentVersion()
	if release.Version() != currentVersion {
		return true, release.Version(), nil
	}
	
	return false, currentVersion, nil
}

// DownloadUpdate downloads and installs the latest version.
func DownloadUpdate() error {
	ctx := context.Background()
	
	src, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return fmt.Errorf("failed to create GitHub source: %w", err)
	}
	
	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source: src,
	})
	if err != nil {
		return fmt.Errorf("failed to create updater: %w", err)
	}
	
	release, found, err := updater.DetectLatest(ctx, selfupdate.NewRepositorySlug("Alejandro-M-P", "git-courer"))
	if err != nil {
		return fmt.Errorf("failed to fetch release: %w", err)
	}
	if !found {
		return fmt.Errorf("no release found for Alejandro-M-P/git-courer")
	}
	
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}
	
	// go-selfupdate handles: download, extract, verify checksum, atomic replace, rollback on failure
	if err := updater.UpdateTo(ctx, release, currentPath); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	
	// Make executable (Windows ignores this, Unix needs it)
	if runtime.GOOS != "windows" {
		os.Chmod(currentPath, 0755)
	}
	
	// Reconfigure MCP with updated binary
	binPath, _ := FindBinaryPath()
	if configured, err := ConfigureAllMCP(binPath); err != nil {
		fmt.Fprintf(os.Stderr, "  MCP setup: %v\n", err)
	} else if configured > 0 {
		fmt.Printf("  %d MCP client(s) reconfigured\n", configured)
	}
	
	return nil
}



func getCurrentVersion() string {
	return config.ServerVersion
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
