package installer

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
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
			expected: `git-courer_.*_windows_amd64\.zip`,
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

func TestPlatformToAssetPattern_WindowsZip(t *testing.T) {
	platform := &Platform{OS: OSWindows, Arch: "amd64"}
	pattern := platformToAssetPattern(platform)

	// Must match real goreleaser v2 Windows asset names (lowercase OS)
	re := regexp.MustCompile(pattern)
	if !re.MatchString("git-courer_1.5.0_windows_amd64.zip") {
		t.Errorf("pattern %q should match goreleaser v2 Windows asset name", pattern)
	}
	// Should NOT match .tar.gz for Windows
	if re.MatchString("git-courer_1.5.0_windows_amd64.tar.gz") {
		t.Errorf("pattern %q should NOT match .tar.gz for Windows", pattern)
	}
}

func TestPlatformToAssetPattern_GoreleaserAssets(t *testing.T) {
	tests := []struct {
		name        string
		os          OS
		arch        string
		assetNames  []string
		shouldMatch bool
	}{
		{
			name:        "linux amd64 matches goreleaser lowercase",
			os:          OSLinux,
			arch:        "amd64",
			assetNames:  []string{"git-courer_1.5.0_linux_amd64.tar.gz"},
			shouldMatch: true,
		},
		{
			name:        "linux amd64 does NOT match Title-case",
			os:          OSLinux,
			arch:        "amd64",
			assetNames:  []string{"git-courer_1.5.0_Linux_amd64.tar.gz"},
			shouldMatch: false,
		},
		{
			name:        "darwin arm64 matches goreleaser lowercase",
			os:          OSMacOS,
			arch:        "arm64",
			assetNames:  []string{"git-courer_2.0.0_darwin_arm64.tar.gz"},
			shouldMatch: true,
		},
		{
			name:        "windows amd64 matches goreleaser .zip",
			os:          OSWindows,
			arch:        "amd64",
			assetNames:  []string{"git-courer_1.5.0_windows_amd64.zip"},
			shouldMatch: true,
		},
		{
			name:        "linux does not match darwin asset",
			os:          OSLinux,
			arch:        "amd64",
			assetNames:  []string{"git-courer_1.4.1_darwin_amd64.tar.gz"},
			shouldMatch: false,
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

func TestPlatform_GitHubAsset_Lowercase(t *testing.T) {
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

func TestArchiveExt(t *testing.T) {
	tests := []struct {
		os       OS
		expected string
	}{
		{OSLinux, ".tar.gz"},
		{OSMacOS, ".tar.gz"},
		{OSWindows, ".zip"},
	}

	for _, tt := range tests {
		t.Run(string(tt.os), func(t *testing.T) {
			p := &Platform{OS: tt.os}
			got := p.ArchiveExt()
			if got != tt.expected {
				t.Errorf("ArchiveExt() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestExtractBinaryFromArchive_TarGz(t *testing.T) {
	// Create a tar.gz archive with a binary
	binaryContent := []byte("fake binary content for linux")
	buf := createTarGzArchive(t, "git-courer", binaryContent)

	platform := &Platform{OS: OSLinux, Arch: "amd64"}
	data, err := extractBinaryFromArchive(bytes.NewReader(buf.Bytes()), platform)
	if err != nil {
		t.Fatalf("extractBinaryFromArchive() error: %v", err)
	}
	if string(data) != string(binaryContent) {
		t.Errorf("extracted content = %q, want %q", string(data), string(binaryContent))
	}
}

func TestExtractBinaryFromArchive_Zip(t *testing.T) {
	// Create a zip archive with git-courer.exe
	binaryContent := []byte("fake binary content for windows")
	buf := createZipArchive(t, "git-courer.exe", binaryContent)

	platform := &Platform{OS: OSWindows, Arch: "amd64"}
	data, err := extractBinaryFromArchive(bytes.NewReader(buf.Bytes()), platform)
	if err != nil {
		t.Fatalf("extractBinaryFromArchive() error: %v", err)
	}
	if string(data) != string(binaryContent) {
		t.Errorf("extracted content = %q, want %q", string(data), string(binaryContent))
	}
}

func TestExtractBinaryFromArchive_Zip_NoExe(t *testing.T) {
	// Create a zip archive with git-courer (no .exe extension)
	binaryContent := []byte("fake binary without exe extension")
	buf := createZipArchive(t, "git-courer", binaryContent)

	platform := &Platform{OS: OSWindows, Arch: "amd64"}
	data, err := extractBinaryFromArchive(bytes.NewReader(buf.Bytes()), platform)
	if err != nil {
		t.Fatalf("extractBinaryFromArchive() error: %v", err)
	}
	if string(data) != string(binaryContent) {
		t.Errorf("extracted content = %q, want %q", string(data), string(binaryContent))
	}
}

// Helper to create a tar.gz archive for testing
func createTarGzArchive(t *testing.T, name string, content []byte) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	hdr := &tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(content)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("tar.WriteHeader: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("tar.Write: %v", err)
	}
	tw.Close()
	gzw.Close()
	return &buf
}

// Helper to create a zip archive for testing
func createZipArchive(t *testing.T, name string, content []byte) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create(name)
	if err != nil {
		t.Fatalf("zip.Create: %v", err)
	}
	if _, err := f.Write(content); err != nil {
		t.Fatalf("zip.Write: %v", err)
	}
	w.Close()
	return &buf
}

// TestAtomicBinaryReplacement tests the temp file + rename pattern.
// This is tested through DownloadUpdate indirectly, but we test the core logic here.
func TestAtomicBinaryReplacement_WritesTempAndRenames(t *testing.T) {
	tmpDir := t.TempDir()
	currentPath := filepath.Join(tmpDir, "git-courer")
	oldContent := []byte("old binary")
	newContent := []byte("new binary content")

	// Create the "current binary"
	if err := os.WriteFile(currentPath, oldContent, 0o755); err != nil {
		t.Fatalf("WriteFile current: %v", err)
	}

	// Write new binary to temp file in same dir, then rename
	tmpPath := currentPath + ".new"
	if err := os.WriteFile(tmpPath, newContent, 0o755); err != nil {
		t.Fatalf("WriteFile temp: %v", err)
	}

	if err := os.Rename(tmpPath, currentPath); err != nil {
		t.Fatalf("Rename temp to current: %v", err)
	}

	// Verify temp file is gone
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Errorf("temp file should not exist after rename")
	}

	// Verify current file has new content
	data, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != string(newContent) {
		t.Errorf("current file content = %q, want %q", string(data), string(newContent))
	}
}

func TestAtomicBinaryReplacement_CrashBeforeRename(t *testing.T) {
	tmpDir := t.TempDir()
	currentPath := filepath.Join(tmpDir, "git-courer")
	oldContent := []byte("old binary")

	// Create the "current binary"
	if err := os.WriteFile(currentPath, oldContent, 0o755); err != nil {
		t.Fatalf("WriteFile current: %v", err)
	}

	// Simulate crash: temp file exists but rename never happened
	tmpPath := currentPath + ".new"
	newContent := []byte("incomplete new binary")
	if err := os.WriteFile(tmpPath, newContent, 0o755); err != nil {
		t.Fatalf("WriteFile temp: %v", err)
	}

	// Old binary should still be intact (crash before rename)
	data, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != string(oldContent) {
		t.Errorf("current file content = %q, want %q (old content should be intact)", string(data), string(oldContent))
	}

	// System is recoverable: temp file can be cleaned up on next run
	_ = os.Remove(tmpPath)
}

func TestAssetMatchers_Order(t *testing.T) {
	platform := &Platform{OS: OSLinux, Arch: "amd64"}
	matchers := assetMatchers(platform)

	if len(matchers) != 2 {
		t.Fatalf("assetMatchers() returned %d matchers, want 2", len(matchers))
	}

	// First matcher should be the goreleaser archive (preferred)
	if !matchers[0].IsArchive {
		t.Errorf("assetMatchers()[0].IsArchive = %v, want true (goreleaser archive should be first)", matchers[0].IsArchive)
	}

	// Second matcher should be the raw binary (fallback)
	if matchers[1].IsArchive {
		t.Errorf("assetMatchers()[1].IsArchive = %v, want false (raw binary should be second)", matchers[1].IsArchive)
	}
}

func TestAssetMatchers_NilPlatform(t *testing.T) {
	matchers := assetMatchers(nil)

	if len(matchers) != 0 {
		t.Errorf("assetMatchers(nil) returned %d matchers, want 0", len(matchers))
	}
}

func TestFindMatchingAsset_ArchivePreferredOverRawBinary(t *testing.T) {
	// When both a goreleaser archive and a raw binary are available,
	// the archive should be preferred (matched first).
	platform := &Platform{OS: OSLinux, Arch: "amd64"}
	matchers := assetMatchers(platform)

	assets := []assetEntry{
		{"git-courer_2.1.0_linux_amd64.tar.gz", "https://example.com/archive"},
		{"git-courer-linux-amd64", "https://example.com/binary"},
	}

	matchedURL, isArchive := findMatchingAsset(assets, matchers)

	if matchedURL != "https://example.com/archive" {
		t.Errorf("findMatchingAsset() URL = %q, want archive URL", matchedURL)
	}
	if !isArchive {
		t.Errorf("findMatchingAsset() isArchive = %v, want true (archive should be preferred)", isArchive)
	}
}

func TestFindMatchingAsset_RawBinaryFallback(t *testing.T) {
	// When only a raw binary is available, it should be matched.
	platform := &Platform{OS: OSMacOS, Arch: "arm64"}
	matchers := assetMatchers(platform)

	assets := []assetEntry{
		{"git-courer-darwin-arm64", "https://example.com/binary"},
	}

	matchedURL, isArchive := findMatchingAsset(assets, matchers)

	if matchedURL != "https://example.com/binary" {
		t.Errorf("findMatchingAsset() URL = %q, want binary URL", matchedURL)
	}
	if isArchive {
		t.Errorf("findMatchingAsset() isArchive = %v, want false (raw binary)", isArchive)
	}
}

func TestFindMatchingAsset_NoMatch(t *testing.T) {
	// When nothing matches, return empty URL.
	platform := &Platform{OS: OSLinux, Arch: "amd64"}
	matchers := assetMatchers(platform)

	assets := []assetEntry{
		{"git-courer_2.1.0_darwin_arm64.tar.gz", "https://example.com/darwin"},
		{"git-courer-2.1.0-unsupported.exe", "https://example.com/unsupported"},
	}

	matchedURL, _ := findMatchingAsset(assets, matchers)

	if matchedURL != "" {
		t.Errorf("findMatchingAsset() URL = %q, want empty (no match)", matchedURL)
	}
}

func TestFindMatchingAsset_WindowsExe(t *testing.T) {
	// Windows raw binary with .exe should match.
	platform := &Platform{OS: OSWindows, Arch: "amd64"}
	matchers := assetMatchers(platform)

	assets := []assetEntry{
		{"git-courer-windows-amd64.exe", "https://example.com/win-binary"},
	}

	matchedURL, isArchive := findMatchingAsset(assets, matchers)

	if matchedURL != "https://example.com/win-binary" {
		t.Errorf("findMatchingAsset() URL = %q, want windows binary URL", matchedURL)
	}
	if isArchive {
		t.Errorf("findMatchingAsset() isArchive = %v, want false (raw binary)", isArchive)
	}
}
