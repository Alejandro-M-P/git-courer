package git

import (
	"reflect"
	"testing"
)

func TestSplitPaths(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "space-separated",
			input:    "a.go b.go c.go",
			expected: []string{"a.go", "b.go", "c.go"},
		},
		{
			name:     "comma-separated",
			input:    "a.go,b.go,c.go",
			expected: []string{"a.go", "b.go", "c.go"},
		},
		{
			name:     "mixed delimiters",
			input:    "a.go, b.go c.go",
			expected: []string{"a.go", "b.go", "c.go"},
		},
		{
			name:     "empty string",
			input:    "",
			expected: nil,
		},
		{
			name:     "extra whitespace",
			input:    "  a.go,  b.go  ",
			expected: []string{"a.go", "b.go"},
		},
		{
			name:     "single file",
			input:    "single.go",
			expected: []string{"single.go"},
		},
		{
			name:     "glob patterns",
			input:    "docs/*, cmd/*",
			expected: []string{"docs/*", "cmd/*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitPaths(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("SplitPaths(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}