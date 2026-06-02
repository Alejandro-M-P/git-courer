package installer

import (
	"testing"
)

func TestGoreleaserArchivePattern(t *testing.T) {
	tests := []struct {
		name        string
		os          OS
		arch        string
		assetNames  []string
		shouldMatch bool
	}{
		{
			name:        "linux amd64 matches goreleaser archive",
			os:          OSLinux,
			arch:        "amd64",
			assetNames:  []string{"git-courer_2.1.0_linux_amd64.tar.gz"},
			shouldMatch: true,
		},
		{
			name:        "darwin arm64 matches goreleaser archive",
			os:          OSMacOS,
			arch:        "arm64",
			assetNames:  []string{"git-courer_2.0.0_darwin_arm64.tar.gz"},
			shouldMatch: true,
		},
		{
			name:        "windows amd64 matches goreleaser zip",
			os:          OSWindows,
			arch:        "amd64",
			assetNames:  []string{"git-courer_1.5.0_windows_amd64.zip"},
			shouldMatch: true,
		},
		{
			name:        "does not match raw binary",
			os:          OSLinux,
			arch:        "amd64",
			assetNames:  []string{"git-courer-linux-amd64"},
			shouldMatch: false,
		},
		{
			name:        "does not match wrong OS",
			os:          OSLinux,
			arch:        "amd64",
			assetNames:  []string{"git-courer_2.1.0_darwin_amd64.tar.gz"},
			shouldMatch: false,
		},
		{
			name:        "does not match wrong arch",
			os:          OSLinux,
			arch:        "amd64",
			assetNames:  []string{"git-courer_2.1.0_linux_arm64.tar.gz"},
			shouldMatch: false,
		},
		{
			name:        "does not match Title-case OS",
			os:          OSLinux,
			arch:        "amd64",
			assetNames:  []string{"git-courer_2.1.0_Linux_amd64.tar.gz"},
			shouldMatch: false,
		},
		{
			name:        "windows does not match tar.gz archive",
			os:          OSWindows,
			arch:        "amd64",
			assetNames:  []string{"git-courer_1.5.0_windows_amd64.tar.gz"},
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			platform := &Platform{OS: tt.os, Arch: tt.arch}
			matcher := platform.GoreleaserArchivePattern()
			if matcher == nil {
				t.Fatal("GoreleaserArchivePattern() returned nil")
			}
			if matcher.IsArchive != true {
				t.Errorf("GoreleaserArchivePattern().IsArchive = %v, want true", matcher.IsArchive)
			}
			re := matcher.Pattern
			for _, assetName := range tt.assetNames {
				matches := re.MatchString(assetName)
				if matches != tt.shouldMatch {
					t.Errorf("pattern %q matching asset %q = %v, want %v", re.String(), assetName, matches, tt.shouldMatch)
				}
			}
		})
	}
}

func TestRawBinaryPattern(t *testing.T) {
	tests := []struct {
		name        string
		os          OS
		arch        string
		assetNames  []string
		shouldMatch bool
	}{
		{
			name:        "linux amd64 matches raw binary",
			os:          OSLinux,
			arch:        "amd64",
			assetNames:  []string{"git-courer-linux-amd64"},
			shouldMatch: true,
		},
		{
			name:        "darwin arm64 matches raw binary",
			os:          OSMacOS,
			arch:        "arm64",
			assetNames:  []string{"git-courer-darwin-arm64"},
			shouldMatch: true,
		},
		{
			name:        "windows amd64 matches raw binary with .exe",
			os:          OSWindows,
			arch:        "amd64",
			assetNames:  []string{"git-courer-windows-amd64.exe"},
			shouldMatch: true,
		},
		{
			name:        "windows amd64 matches raw binary without .exe",
			os:          OSWindows,
			arch:        "amd64",
			assetNames:  []string{"git-courer-windows-amd64"},
			shouldMatch: true,
		},
		{
			name:        "does not match goreleaser archive",
			os:          OSLinux,
			arch:        "amd64",
			assetNames:  []string{"git-courer_2.1.0_linux_amd64.tar.gz"},
			shouldMatch: false,
		},
		{
			name:        "does not match wrong OS",
			os:          OSLinux,
			arch:        "amd64",
			assetNames:  []string{"git-courer-darwin-amd64"},
			shouldMatch: false,
		},
		{
			name:        "does not match wrong arch",
			os:          OSLinux,
			arch:        "amd64",
			assetNames:  []string{"git-courer-linux-arm64"},
			shouldMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			platform := &Platform{OS: tt.os, Arch: tt.arch}
			matcher := platform.RawBinaryPattern()
			if matcher == nil {
				t.Fatal("RawBinaryPattern() returned nil")
			}
			if matcher.IsArchive != false {
				t.Errorf("RawBinaryPattern().IsArchive = %v, want false", matcher.IsArchive)
			}
			re := matcher.Pattern
			for _, assetName := range tt.assetNames {
				matches := re.MatchString(assetName)
				if matches != tt.shouldMatch {
					t.Errorf("pattern %q matching asset %q = %v, want %v", re.String(), assetName, matches, tt.shouldMatch)
				}
			}
		})
	}
}

func TestGitHubAsset_UsesLowercaseOS(t *testing.T) {
	tests := []struct {
		os       OS
		arch     string
		expected string
	}{
		{OSLinux, "amd64", "git-courer_linux_amd64"},
		{OSMacOS, "arm64", "git-courer_darwin_arm64"},
		{OSWindows, "amd64", "git-courer_windows_amd64"},
	}

	for _, tt := range tests {
		t.Run(string(tt.os)+"_"+tt.arch, func(t *testing.T) {
			p := &Platform{OS: tt.os, Arch: tt.arch}
			got := p.GitHubAsset()
			if got != tt.expected {
				t.Errorf("GitHubAsset() = %q, want %q", got, tt.expected)
			}
		})
	}
}