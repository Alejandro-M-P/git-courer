// Package installer provides installation and management for git-courer.
package installer

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// Release represents a GitHub release.
type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

// Asset represents a release asset.
type Asset struct {
	Name        string `json:"name"`
	BrowserURL  string `json:"browser_download_url"`
	DownloadURL string `json:"download_url"`
}

// FetchLatestRelease fetches the latest release from GitHub.
func FetchLatestRelease(owner, repo string) (*Release, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "git-courer-installer")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release not found (status: %d)", resp.StatusCode)
	}

	var release Release
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to decode release: %w", err)
	}

	return &release, nil
}

// FindAsset finds the asset matching the platform.
func (r *Release) FindAsset(platform *Platform) *Asset {
	targetName := platform.GitHubAsset()
	for i := range r.Assets {
		asset := &r.Assets[i]
		if len(asset.Name) >= len(targetName) && asset.Name[:len(targetName)] == targetName {
			return asset
		}
	}
	return nil
}

// DownloadAsset downloads the asset to the given path.
func (a *Asset) DownloadAsset(to string) error {
	resp, err := http.Get(a.DownloadURL)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed (status: %d)", resp.StatusCode)
	}

	out, err := os.Create(to)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// GetInstallPath returns the default install path for the platform.
func GetInstallPath() string {
	home, _ := os.UserHomeDir()

	switch Detect().OS {
	case OSWindows:
		return filepath.Join(home, "AppData", "Local", "bin")
	default:
		return "/usr/local/bin"
	}
}
