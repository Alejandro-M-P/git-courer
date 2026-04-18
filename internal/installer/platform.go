// Package installer provides installation and management for git-courer.
package installer

import (
	"fmt"
	"runtime"
)

// OS represents a supported operating system.
type OS string

const (
	OSLinux   OS = "linux"
	OSMacOS   OS = "darwin"
	OSWindows OS = "windows"
)

// Platform represents the current platform.
type Platform struct {
	OS      OS
	Arch    string
	Version string
}

// Detect returns the current platform.
func Detect() *Platform {
	os := runtime.GOOS
	switch os {
	case "linux":
		os = "linux"
	case "darwin":
		os = "darwin"
	case "windows":
		os = "windows"
	}

	return &Platform{
		OS:      OS(os),
		Arch:    runtime.GOARCH,
		Version: "latest",
	}
}

// BinaryName returns the binary name for this platform.
func (p *Platform) BinaryName() string {
	ext := ""
	if p.OS == OSWindows {
		ext = ".exe"
	}
	return fmt.Sprintf("git-courer-%s-%s%s", p.OS, p.Arch, ext)
}

// GitHubAsset returns the GitHub asset name.
func (p *Platform) GitHubAsset() string {
	// Map runtime arch to GitHub release arch names
	arch := p.Arch
	if arch == "arm64" {
		arch = "arm64"
	} else if arch == "amd64" {
		arch = "amd64"
	}
	return fmt.Sprintf("git-courer-%s-%s", p.OS, arch)
}

// String returns a string representation of the platform.
func (p *Platform) String() string {
	return fmt.Sprintf("%s/%s", p.OS, p.Arch)
}
