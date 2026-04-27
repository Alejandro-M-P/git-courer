package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsBinary(t *testing.T) {
	// Create temp dir for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		content  []byte
		expected bool
	}{
		// ELF (Linux)
		{"elf binary", []byte{0x7F, 0x45, 0x4C, 0x46, 0x00, 0x00, 0x00, 0x00}, true},

		// PE (Windows)
		{"pe executable", []byte{0x4D, 0x5A, 0x90, 0x00, 0x03, 0x00, 0x00, 0x00}, true},

		// Mach-O 64-bit
		{"macho 64", []byte{0xFE, 0xED, 0xFA, 0xCF, 0x00, 0x00, 0x00, 0x00}, true},

		// Mach-O 32-bit
		{"macho 32", []byte{0xFE, 0xED, 0xFA, 0xCE, 0x00, 0x00, 0x00, 0x00}, true},

		// Java class
		{"java class", []byte{0xCA, 0xFE, 0xBA, 0xBE, 0x00, 0x00, 0x00, 0x00}, true},

		// ZIP/JAR
		{"zip file", []byte{0x50, 0x4B, 0x03, 0x04, 0x00, 0x00, 0x00, 0x00}, true},

		// Plain text (NOT binary)
		{"plain text", []byte("Hello, World!"), false},

		// Source code
		{"go source", []byte("package main\n\nfunc main() {}"), false},

		// Short content (< 4 bytes)
		{"too short", []byte{0x7F, 0x45}, false},

		// Empty file
		{"empty", []byte{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with content
			f, err := os.CreateTemp(tmpDir, "test_*")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			if _, err := f.Write(tt.content); err != nil {
				t.Fatal(err)
			}

			got := IsBinary(f.Name())
			if got != tt.expected {
				t.Errorf("IsBinary(%q) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

func TestIsBinaryFileNotFound(t *testing.T) {
	// Non-existent file should return false
	got := IsBinary("/nonexistent/path/to/file")
	if got != false {
		t.Errorf("IsBinary(nonexistent) = %v, want false", got)
	}
}

func TestIsBinaryGzip(t *testing.T) {

	// Gzip magic bytes
	f, err := os.CreateTemp(t.TempDir(), "test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if _, err := f.Write([]byte{0x1F, 0x8B, 0x08, 0x00}); err != nil {
		t.Fatal(err)
	}

	got := IsBinary(f.Name())
	if got != true {
		t.Errorf("IsBinary(gzip) = %v, want true", got)
	}
}

func TestIsBinaryRAR(t *testing.T) {
	// RAR magic bytes
	f, err := os.CreateTemp(t.TempDir(), "test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if _, err := f.Write([]byte{0x52, 0x61, 0x72, 0x21, 0x1A, 0x07, 0x01, 0x00}); err != nil {
		t.Fatal(err)
	}

	got := IsBinary(f.Name())
	if got != true {
		t.Errorf("IsBinary(rar) = %v, want true", got)
	}
}

func TestIsBinaryPESuffix(t *testing.T) {
	// PE files can also be identified by .exe suffix, but the magic bytes are MZ
	// Test that MZ header alone triggers binary detection
	tmpDir := t.TempDir()
	f, err := os.CreateTemp(tmpDir, "test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// Write just the MZ header (2 bytes)
	if _, err := f.Write([]byte{0x4D, 0x5A}); err != nil {
		t.Fatal(err)
	}

	got := IsBinary(f.Name())
	if got != true {
		t.Errorf("IsBinary(pe_suffix) = %v, want true", got)
	}
}

func TestIsBinaryPathNormalization(t *testing.T) {
	// Test that filepath.Base is used correctly
	tmpDir := t.TempDir()
	f, err := os.CreateTemp(tmpDir, "test_*")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.Write([]byte("some random content")); err != nil {
		f.Close()
		t.Fatal(err)
	}

	// Close file before rename — required for Windows to release file lock
	originalPath := f.Name()
	f.Close()

	// Use filepath.Dir to create a nested path
	nestedPath := filepath.Join(tmpDir, "subdir", "file.txt")
	if err := os.MkdirAll(filepath.Dir(nestedPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(originalPath, nestedPath); err != nil {
		t.Fatal(err)
	}

	// Should still be detected as non-binary
	got := IsBinary(nestedPath)
	if got != false {
		t.Errorf("IsBinary(nested_path) = %v, want false", got)
	}
}
