package secrets

import (
	"bytes"
	"os"
	"path/filepath"
)

// Binary signatures detected by magic bytes.
var binarySignatures = []struct {
	header []byte
	name   string
}{
	{[]byte{0x7F, 'E', 'L', 'F'}, "elf"},           // ELF (Linux)
	{[]byte{'M', 'Z'}, "pe"},                       // PE (Windows)
	{[]byte{0xFE, 0xED, 0xFA, 0xCE}, "macho_32"},   // Mach-O 32-bit (macOS)
	{[]byte{0xFE, 0xED, 0xFA, 0xCF}, "macho_64"},   // Mach-O 64-bit (macOS)
	{[]byte{0xCA, 0xFE, 0xBA, 0xBE}, "java_class"}, // Java .class
	{[]byte{0x50, 0x4B, 0x03, 0x04}, "zip"},        // ZIP/JAR
	{[]byte{0x1F, 0x8B}, "gzip"},                   // Gzip
	{[]byte{0x52, 0x61, 0x72, 0x21}, "rar"},        // RAR
}

// IsBinary returns true if the file at path is a binary executable.
func IsBinary(path string) bool {
	// Check for git-courer self-binary first
	if isGitCourerBinary(path) {
		return true
	}

	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	// Read first 8 bytes (largest signature is 8 bytes)
	header := make([]byte, 8)
	n, err := f.Read(header)
	if err != nil || n == 0 {
		return false
	}

	// Check against all known binary signatures
	for _, sig := range binarySignatures {
		if n >= len(sig.header) && bytes.HasPrefix(header, sig.header) {
			return true
		}
	}

	return false
}

// isGitCourerBinary checks if the file is the git-courer binary itself.
func isGitCourerBinary(path string) bool {
	// Check if filename is "git-courer" or "git-courer.exe"
	name := filepath.Base(path)
	return name == "git-courer" || name == "git-courer.exe"
}
