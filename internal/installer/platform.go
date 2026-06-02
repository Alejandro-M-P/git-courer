// Package installer provides installation and management for git-courer.
package installer

import (
	"fmt"
	"regexp"
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

// assetMatcher pairs a compiled regex for GitHub asset name matching
// with a flag indicating whether the matched asset is an archive.
type assetMatcher struct {
	Pattern   *regexp.Regexp // compiled regex for asset name matching
	IsArchive bool           // true = extract from archive, false = raw binary
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
// Assets are named like: git-courer_1.0.1_linux_amd64.tar.gz
// OS names use lowercase to match Goreleaser v2 output.
func (p *Platform) GitHubAsset() string {
	return fmt.Sprintf("git-courer_%s_%s", p.OS, p.Arch)
}

// ArchiveExt returns the archive extension for this platform.
// Windows uses .zip, all other platforms use .tar.gz.
func (p *Platform) ArchiveExt() string {
	if p.OS == OSWindows {
		return ".zip"
	}
	return ".tar.gz"
}

// GoreleaserArchivePattern returns a matcher for goreleaser archive assets.
// Pattern: git-courer_{version}_{os}_{arch}.{tar.gz|zip}
// OS names are lowercase (matching goreleaser v2 naming).
func (p *Platform) GoreleaserArchivePattern() *assetMatcher {
	if p == nil {
		return nil
	}
	osLower := string(p.OS)
	archPattern := regexp.QuoteMeta(p.Arch)
	ext := p.ArchiveExt()
	// goreleaser v2 format: git-courer_{version}_{os}_{arch}.{ext}
	pattern := fmt.Sprintf(`git-courer_\d+\.\d+\.\d+_%s_%s%s`, osLower, archPattern, regexp.QuoteMeta(ext))
	re := regexp.MustCompile(pattern)
	return &assetMatcher{Pattern: re, IsArchive: true}
}

// RawBinaryPattern returns a matcher for manually uploaded raw binary assets.
// Pattern: git-courer-{os}-{arch}[.exe] (optional .exe for Windows)
// OS names are lowercase, hyphen-separated.
func (p *Platform) RawBinaryPattern() *assetMatcher {
	if p == nil {
		return nil
	}
	osLower := string(p.OS)
	archPattern := regexp.QuoteMeta(p.Arch)
	// .exe is optional for Windows raw binaries (some are uploaded without it)
	exeSuffix := ""
	if p.OS == OSWindows {
		exeSuffix = `(\.exe)?`
	}
	// Manual upload format: git-courer-{os}-{arch}[.exe]
	pattern := fmt.Sprintf(`^git-courer-%s-%s%s$`, osLower, archPattern, exeSuffix)
	re := regexp.MustCompile(pattern)
	return &assetMatcher{Pattern: re, IsArchive: false}
}

// String returns a string representation of the platform.
func (p *Platform) String() string {
	return fmt.Sprintf("%s/%s", p.OS, p.Arch)
}
