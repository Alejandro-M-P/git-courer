package mcp

import (
	"testing"

	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
)

// TestMatchesFilterBaseName tests that filter matches both full path and base name.
func TestMatchesFilterBaseName(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		pattern string
		want    bool
	}{
		{"full path match", "internal/foo/bar.go", "foo/bar", true},
		{"base name match", "internal/foo/bar.go", "bar.go", true},
		{"no match", "internal/foo/bar.go", "baz.go", false},
		{"empty pattern", "anything", "", true},
		{"partial base", "pkg/utils/helper.go", "helper", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shared.MatchesFilter(tt.s, tt.pattern)
			if got != tt.want {
				t.Errorf("shared.MatchesFilter(%q, %q) = %v, want %v", tt.s, tt.pattern, got, tt.want)
			}
		})
	}
}
