package prompts

import (
	"testing"
)

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "null byte within first 8KB",
			data:     []byte("hello\x00world"),
			expected: true,
		},
		{
			name:     "text without null bytes",
			data:     []byte("This is a normal text file."),
			expected: false,
		},
		{
			name:     "empty slice",
			data:     []byte{},
			expected: false,
		},
		{
			name:     "binary file header with null byte",
			data:     []byte{0x89, 0x50, 0x4E, 0x47, 0x00, 0x0A}, // PNG-like with null
			expected: true,
		},
		{
			name:     "large file with null at 9001",
			data:     append(make([]byte, 9000, 9001), 0x00),
			expected: true,
		},
		{
			name:     "UTF-8 text",
			data:     []byte("Héllo, 世界! 🌍"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBinary(tt.data)
			if got != tt.expected {
				t.Errorf("IsBinary(%q) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}
