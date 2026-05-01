package installer

import (
	"regexp"
	"testing"
)

func TestPlatformToAssetPattern(t *testing.T) {
	tests := []struct {
		name     string
		os       OS
		arch     string
		expected string
	}{
		{
			name:     "linux amd64",
			os:       OSLinux,
			arch:     "amd64",
			expected: `git-courer_.*_linux_amd64\.tar\.gz`,
		},
		{
			name:     "darwin arm64",
			os:       OSMacOS,
			arch:     "arm64",
			expected: `git-courer_.*_darwin_arm64\.tar\.gz`,
		},
		{
			name:     "windows amd64",
			os:       OSWindows,
			arch:     "amd64",
			expected: `git-courer_.*_windows_amd64\.tar\.gz`,
		},
		{
			name:     "linux 386",
			os:       OSLinux,
			arch:     "386",
			expected: `git-courer_.*_linux_386\.tar\.gz`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			platform := &Platform{
				OS:   tt.os,
				Arch: tt.arch,
			}
			pattern := platformToAssetPattern(platform)
			if pattern != tt.expected {
				t.Errorf("platformToAssetPattern() = %q, want %q", pattern, tt.expected)
			}
			// Ensure pattern is valid regex
			_, err := regexp.Compile(pattern)
			if err != nil {
				t.Errorf("failed to compile regex %q: %v", pattern, err)
			}
		})
	}
}

func TestPlatformToAssetPattern_NilPlatform(t *testing.T) {
	pattern := platformToAssetPattern(nil)
	if pattern != "" {
		t.Errorf("platformToAssetPattern(nil) = %q, want \"\"", pattern)
	}
}

func TestAssetFilterPatternMatching(t *testing.T) {
	tests := []struct {
		name        string
		os          OS
		arch        string
		assetNames  []string
		shouldMatch bool
	}{
		{
			name:        "linux amd64 matches correct asset",
			os:          OSLinux,
			arch:        "amd64",
			assetNames:  []string{"git-courer_1.4.1_linux_amd64.tar.gz", "git-courer_1.4.0_linux_amd64.tar.gz"},
			shouldMatch: true,
		},
		{
			name:        "linux amd64 does not match darwin asset",
			os:          OSLinux,
			arch:        "amd64",
			assetNames:  []string{"git-courer_1.4.1_darwin_amd64.tar.gz"},
			shouldMatch: false,
		},
		{
			name:        "darwin arm64 matches",
			os:          OSMacOS,
			arch:        "arm64",
			assetNames:  []string{"git-courer_2.0.0_darwin_arm64.tar.gz"},
			shouldMatch: true,
		},
		{
			name:        "windows amd64 matches",
			os:          OSWindows,
			arch:        "amd64",
			assetNames:  []string{"git-courer_1.5.0_windows_amd64.tar.gz"},
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			platform := &Platform{
				OS:   tt.os,
				Arch: tt.arch,
			}
			pattern := platformToAssetPattern(platform)
			if pattern == "" {
				t.Fatal("platformToAssetPattern returned empty string")
			}
			regex := regexp.MustCompile(pattern)
			for _, assetName := range tt.assetNames {
				matches := regex.MatchString(assetName)
				if matches != tt.shouldMatch {
					t.Errorf("pattern %q matching asset %q = %v, want %v", pattern, assetName, matches, tt.shouldMatch)
				}
			}
		})
	}
}
