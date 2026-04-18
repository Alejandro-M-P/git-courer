// Package installer provides installation and management for git-courer.
package installer

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// CheckForUpdates checks if a new version is available.
func CheckForUpdates() (bool, string, error) {
	release, err := FetchLatestRelease("Alejandro-M-P", "git-courer")
	if err != nil {
		return false, "", err
	}

	currentVersion := getCurrentVersion()
	newVersion := strings.TrimPrefix(release.TagName, "v")

	if newVersion != currentVersion {
		return true, newVersion, nil
	}

	return false, currentVersion, nil
}

// DownloadUpdate downloads and installs the latest version.
func DownloadUpdate() error {
	release, err := FetchLatestRelease("Alejandro-M-P", "git-courer")
	if err != nil {
		return fmt.Errorf("failed to fetch release: %w", err)
	}

	platform := Detect()
	asset := release.FindAsset(platform)
	if asset == nil {
		return fmt.Errorf("no asset found for platform %s", platform)
	}

	// Download to temp
	tmpPath := "/tmp/git-courer-update"
	if runtime.GOOS == "windows" {
		tmpPath += ".exe"
	}

	if err := asset.DownloadAsset(tmpPath); err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}

	// Find current binary and replace
	currentPath, err := FindBinaryPath()
	if err != nil {
		// Try common paths
		currentPath = "/usr/local/bin/git-courer"
		if runtime.GOOS == "windows" {
			currentPath = filepath.Join(os.Getenv("LOCALAPPDATA"), "git-courer.exe")
		}
	}

	// Backup current
	backupPath := currentPath + ".backup"
	if _, err := os.Stat(currentPath); err == nil {
		if err := os.Rename(currentPath, backupPath); err != nil {
			return fmt.Errorf("failed to backup current binary: %w", err)
		}
	}

	// Move new binary to location
	if err := os.Rename(tmpPath, currentPath); err != nil {
		// Restore from backup
		os.Rename(backupPath, currentPath)
		return fmt.Errorf("failed to install update: %w", err)
	}

	// Make executable
	os.Chmod(currentPath, 0755)

	// Remove backup
	os.Remove(backupPath)

	return nil
}

func getCurrentVersion() string {
	return "0.1.0" // TODO: read from config or binary
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
