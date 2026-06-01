// Package installer provides installation and management for git-courer.
package installer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/blak0p/git-courer/internal/config"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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

	// Extract binary from archive
	if _, err := tmpFile.Seek(0, 0); err != nil {
		return err
	}

	platform := &Platform{OS: OS(runtime.GOOS), Arch: runtime.GOARCH}
	binData, err := extractBinaryFromArchive(tmpFile, platform)
	if err != nil {
		return err
	}

	// Atomic binary replacement: write to temp file in same dir, then rename
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	// Write new binary to temp file in same directory (same filesystem guarantees atomic rename)
	currentDir := filepath.Dir(currentPath)
	newPath := filepath.Join(currentDir, filepath.Base(currentPath)+".new")
	if err := os.WriteFile(newPath, binData, 0o755); err != nil {
		return fmt.Errorf("failed to write new binary: %w", err)
	}
	defer os.Remove(newPath) // cleanup in case rename fails

	// Atomic rename: this replaces the old binary atomically on the same filesystem
	if err := os.Rename(newPath, currentPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	// Post-update: Reconfigure MCP
	_, _ = ConfigureAllMCP(currentPath)

	return nil
}

func fetchLatestRelease(ctx context.Context) (*githubRelease, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.github.com/repos/blak0p/git-courer/releases/latest", nil)
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

// extractBinaryFromArchive dispatches to the correct extraction function
// based on the platform's archive type.
func extractBinaryFromArchive(r io.Reader, platform *Platform) ([]byte, error) {
	if platform.OS == OSWindows {
		// zip requires io.ReaderAt and size, so we read all first
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("read zip archive: %w", err)
		}
		return extractBinaryFromZip(bytes.NewReader(data), int64(len(data)))
	}
	return extractBinaryFromTarGz(r)
}

// extractBinaryFromZip extracts the git-courer binary from a .zip archive.
// For Windows, the binary name is git-courer.exe; without .exe as fallback.
func extractBinaryFromZip(r io.ReaderAt, size int64) ([]byte, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("open zip archive: %w", err)
	}

	// Look for git-courer.exe first (Windows), then git-courer as fallback
	for _, f := range zr.File {
		name := f.Name
		// Strip directory prefix if present
		if idx := strings.LastIndex(name, "/"); idx >= 0 {
			name = name[idx+1:]
		}
		if name == "git-courer.exe" || name == "git-courer" {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("open file in zip: %w", err)
			}
			defer rc.Close()
			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("read file in zip: %w", err)
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("binary not found in zip archive")
}

func platformToAssetPattern(platform *Platform) string {
	if platform == nil {
		return ""
	}
	osTitle := cases.Title(language.English).String(string(platform.OS))
	archPattern := regexp.QuoteMeta(platform.Arch)
	ext := regexp.QuoteMeta(platform.ArchiveExt())
	return fmt.Sprintf("git-courer_.*_%s_%s%s", osTitle, archPattern, ext)
}
