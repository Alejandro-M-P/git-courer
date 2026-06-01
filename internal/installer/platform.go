// Package installer provides installation and management for git-courer.
package installer

import (
	"fmt"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
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

// BinaryName returns the binary name for this platform.
func (p *Platform) BinaryName() string {
	ext := ""
	if p.OS == OSWindows {
		ext = ".exe"
	}
	return fmt.Sprintf("git-courer-%s-%s%s", p.OS, p.Arch, ext)
}

// GitHubAsset returns the partial GitHub asset name (without version).
// Assets are named like: git-courer_1.0.1_Darwin_arm64.tar.gz
// OS names use Title-case to match Goreleaser v2 output.
func (p *Platform) GitHubAsset() string {
	osTitle := cases.Title(language.English).String(string(p.OS))
	return fmt.Sprintf("git-courer_%s_%s", osTitle, p.Arch)
}

// ArchiveExt returns the archive extension for this platform.
// Windows uses .zip, all other platforms use .tar.gz.
func (p *Platform) ArchiveExt() string {
	if p.OS == OSWindows {
		return ".zip"
	}
	return ".tar.gz"
}

// String returns a string representation of the platform.
func (p *Platform) String() string {
	return fmt.Sprintf("%s/%s", p.OS, p.Arch)
}
