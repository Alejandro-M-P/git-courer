// Package installer provides installation and management for git-courer.
package installer

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
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

	// Download tar.gz to temp
	tmpTar := "/tmp/git-courer-update.tar.gz"

	if err := asset.DownloadAsset(tmpTar); err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}

	// Extract binary from tar.gz
	binaryName := "git-courer"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	tmpBinary, err := extractBinaryFromTarGz(tmpTar, binaryName)
	if err != nil {
		os.Remove(tmpTar)
		return fmt.Errorf("failed to extract binary: %w", err)
	}

	// Find current binary
	currentPath, err := os.Executable()
	if err != nil {
		os.Remove(tmpBinary)
		os.Remove(tmpTar)
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Remove existing binary first (force replace)
	if err := os.Remove(currentPath); err != nil && !os.IsNotExist(err) {
		os.Remove(tmpBinary)
		os.Remove(tmpTar)
		return fmt.Errorf("failed to remove old binary: %w", err)
	}

	// Copy new binary to location (avoids EXDEV cross-device issues)
	if err := copyFile(tmpBinary, currentPath); err != nil {
		os.Remove(tmpBinary)
		os.Remove(tmpTar)
		return fmt.Errorf("failed to install update: %w", err)
	}

	// Make executable
	os.Chmod(currentPath, 0755)

	// Cleanup
	os.Remove(tmpBinary)
	os.Remove(tmpTar)

	// Reconfigure MCP with updated binary
	binPath, _ := FindBinaryPath()
	if configured, err := ConfigureAllMCP(binPath); err != nil {
		fmt.Fprintf(os.Stderr, "  MCP setup: %v\n", err)
	} else if configured > 0 {
		fmt.Printf("  %d MCP client(s) reconfigured\n", configured)
	}

	return nil
}

// extractBinaryFromTarGz extracts a binary from a tar.gz archive.
func extractBinaryFromTarGz(tarPath, binaryName string) (string, error) {
	file, err := os.Open(tarPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return "", err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}

		if header.Name == binaryName || filepath.Base(header.Name) == binaryName {
			tmpPath := "/tmp/git-courer-binary"
			if runtime.GOOS == "windows" {
				tmpPath += ".exe"
			}

			out, err := os.Create(tmpPath)
			if err != nil {
				return "", err
			}
			defer out.Close()

			if _, err := io.Copy(out, tr); err != nil {
				return "", err
			}

			return tmpPath, nil
		}
	}

	return "", fmt.Errorf("binary %s not found in archive", binaryName)
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	from, err := os.Open(src)
	if err != nil {
		return err
	}
	defer from.Close()

	to, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer to.Close()

	_, err = io.Copy(to, from)
	if err != nil {
		return err
	}

	return to.Sync()
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
