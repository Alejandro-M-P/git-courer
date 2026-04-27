package secrets

import (
	"bytes"
	"os"
)

// IsBinary checks if a file is binary by looking at its first 512 bytes.
// It uses magic bytes detection for common executable formats.
func IsBinary(filePath string) bool {
	f, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer f.Close()

	buffer := make([]byte, 512)
	n, err := f.Read(buffer)
	if err != nil && n == 0 {
		return false
	}

	// 1. Check for common executable signatures (Magic Bytes)
	// ELF (Linux)
	if n >= 4 && bytes.Equal(buffer[:4], []byte{0x7f, 'E', 'L', 'F'}) {
		return true
	}
	// Mach-O (macOS)
	if n >= 4 && (bytes.Equal(buffer[:4], []byte{0xfe, 0xed, 0xfa, 0xce}) || bytes.Equal(buffer[:4], []byte{0xcf, 0xfa, 0xed, 0xfe})) {
		return true
	}
	// PE (Windows)
	if n >= 2 && bytes.Equal(buffer[:2], []byte{'M', 'Z'}) {
		return true
	}

	// 2. Statistical analysis: check for null bytes or high concentration of non-printable chars
	nullCount := 0
	nonPrintable := 0
	for i := 0; i < n; i++ {
		if buffer[i] == 0 {
			nullCount++
		}
		// Basic printable ASCII range + common control chars (tab, newline)
		if (buffer[i] < 32 && buffer[i] != 9 && buffer[i] != 10 && buffer[i] != 13) || buffer[i] > 126 {
			nonPrintable++
		}
	}

	// If more than 0 null bytes or > 15% non-printable characters, it's almost certainly binary
	// For very short files (< 8 bytes), we are more lenient unless there's a null byte.
	if nullCount > 0 {
		return true
	}
	if n > 8 && (float64(nonPrintable)/float64(n) > 0.15) {
		return true
	}

	return false
}
