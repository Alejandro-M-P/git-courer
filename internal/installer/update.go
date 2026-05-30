// Package installer provides installation and management for git-courer.
package installer

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/config"
)

// githubRelease matches the structure of the GitHub API response for releases.
type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// CheckForUpdates checks if a new version is available using GitHub API.
func CheckForUpdates() (bool, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	latest, err := fetchLatestRelease(ctx)
	if err != nil {
		return false, "", err
	}

	currentVersion := strings.TrimPrefix(config.ServerVersion, "v")
	newVersion := strings.TrimPrefix(latest.TagName, "v")

	if newVersion != currentVersion {
		return true, newVersion, nil
	}

	return false, currentVersion, nil
}

// DownloadUpdate downloads and installs the latest version.
func DownloadUpdate() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	latest, err := fetchLatestRelease(ctx)
	if err != nil {
		return err
	}

	assetURL := ""
	pattern := platformToAssetPattern(&Platform{OS: OS(runtime.GOOS), Arch: runtime.GOARCH})
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid asset pattern: %w", err)
	}

	for _, asset := range latest.Assets {
		if re.MatchString(asset.Name) {
			assetURL = asset.BrowserDownloadURL
			break
		}
	}

	if assetURL == "" {
		return fmt.Errorf("no compatible asset found for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Create temp file for download
	tmpFile, err := os.CreateTemp("", "git-courer-update-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if err := downloadFile(ctx, assetURL, tmpFile); err != nil {
		return err
	}

	// Extract binary from tar.gz
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return err
	}
	
	binData, err := extractBinaryFromTarGz(tmpFile)
	if err != nil {
		return err
	}

	// Atomic replacement
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	// On Linux/Unix, we can't overwrite a running binary directly, 
	// so we move it to a temp path first.
	oldPath := currentPath + ".old"
	if err := os.Rename(currentPath, oldPath); err != nil {
		return fmt.Errorf("failed to move old binary: %w", err)
	}
	defer os.Remove(oldPath)

	if err := os.WriteFile(currentPath, binData, 0755); err != nil {
		// Try to restore if failed
		_ = os.Rename(oldPath, currentPath)
		return fmt.Errorf("failed to write new binary: %w", err)
	}

	// Post-update: Reconfigure MCP
	_, _ = ConfigureAllMCP(currentPath)

	return nil
}

func fetchLatestRelease(ctx context.Context) (*githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/repos/Alejandro-M-P/git-courer/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api error: %s", resp.Status)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	return &release, nil
}

func downloadFile(ctx context.Context, url string, w io.Writer) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download: %s", resp.Status)
	}

	_, err = io.Copy(w, resp.Body)
	return err
}

func extractBinaryFromTarGz(r io.Reader) ([]byte, error) {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if header.Typeflag == tar.TypeReg && (header.Name == "git-courer" || strings.HasSuffix(header.Name, "/git-courer")) {
			return io.ReadAll(tr)
		}
	}

	return nil, fmt.Errorf("binary not found in archive")
}


func platformToAssetPattern(platform *Platform) string {
	if platform == nil {
		return ""
	}
	osPattern := regexp.QuoteMeta(string(platform.OS))
	archPattern := regexp.QuoteMeta(platform.Arch)
	return fmt.Sprintf("git-courer_.*_%s_%s\\.tar\\.gz", osPattern, archPattern)
}



