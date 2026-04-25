// Package config provides build-time version detection.
package config

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

// ServerVersion is the current server version.
// Can be overridden at build time:
//   - goreleaser: -X github.com/Alejandro-M-P/git-courer/internal/config.ServerVersion={{.Version}}
//   - manual: go build -ldflags "-X github.com/Alejandro-M-P/git-courer/internal/config.ServerVersion=1.2.3"
//
// Priority order:
//   1. Set via ldflags at build time (takes precedence)
//   2. GitHub latest release (requires network, most accurate)
//   3. git describe --tags (fallback for development)
//   4. git rev-parse --short HEAD (last resort)
var ServerVersion = "dev"

// init automatically detects version from GitHub if not overridden at build time.
func init() {
	if ServerVersion == "dev" {
		// Try GitHub API first (most accurate - gets actual latest release)
		if v := detectVersionFromGitHub(); v != "" {
			ServerVersion = v
			return
		}
		// Fallback: git describe
		if v := detectVersionFromGit(); v != "" {
			ServerVersion = v
			return
		}
	}
}

// detectVersionFromGitHub gets the latest release version from GitHub API.
func detectVersionFromGitHub() string {
	url := "https://api.github.com/repos/Alejandro-M-P/git-courer/releases/latest"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "git-courer")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return ""
	}

	return strings.TrimPrefix(release.TagName, "v")
}

// detectVersionFromGit extracts version from git tags (fallback).
func detectVersionFromGit() string {
	// Try git describe --tags (gets most recent tag)
	if output, err := exec.Command("git", "describe", "--tags", "--abbrev=0", "--dirty").Output(); err == nil {
		version := strings.TrimSpace(string(output))
		return strings.TrimPrefix(version, "v")
	}

	// Fallback: get short commit hash
	if output, err := exec.Command("git", "rev-parse", "--short", "HEAD").Output(); err == nil {
		return strings.TrimSpace(string(output))
	}

	return ""
}

// GetLatestVersion retrieves the latest version from GitHub (for external use).
func GetLatestVersion() (string, error) {
	// First try ldflags version
	if ServerVersion != "dev" {
		return ServerVersion, nil
	}

	// Then GitHub API
	if v := detectVersionFromGitHub(); v != "" {
		return v, nil
	}

	return "", fmt.Errorf("could not detect latest version")
}