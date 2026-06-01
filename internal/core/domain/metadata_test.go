package domain

import (
	"testing"
)

func TestIsMetadataPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		path string
		want bool
	}{
		{".git-courer", true},
		{".git-courer/", true},
		{".git-courer/config.json", true},
		{".git-courer/branches/feat-add-pi/commits.json", true},
		{"internal/core/domain/metadata.go", false},
		{"git-courer", false},
		{"foo/.git-courer", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			t.Parallel()
			got := IsMetadataPath(tt.path)
			if got != tt.want {
				t.Errorf("IsMetadataPath(%q) = %v; want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestMetadataDirConstant(t *testing.T) {
	if MetadataDir != ".git-courer" {
		t.Errorf("MetadataDir = %q; want %q", MetadataDir, ".git-courer")
	}
}
