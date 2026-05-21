package classifier

import (
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// TestCommitTypeHelperAdapter_ImplementsPort verifies the adapter satisfies the port.
func TestCommitTypeHelperAdapter_ImplementsPort(t *testing.T) {
	t.Parallel()

	// Compile-time interface satisfaction check
	var _ ports.CommitTypeHelper = (*CommitTypeHelperAdapter)(nil)
}

// TestCommitTypeHelperAdapter_InferCommitType verifies InferCommitType delegates correctly.
func TestCommitTypeHelperAdapter_InferCommitType(t *testing.T) {
	t.Parallel()

	helper := NewCommitTypeHelperAdapter()

	tests := []struct {
		name  string
		chunk domain.DiffChunk
		want  string
	}{
		{
			name:  "classified chunk returns type",
			chunk: domain.DiffChunk{CommitType: "feat"},
			want:  "feat",
		},
		{
			name:  "new file returns feat",
			chunk: domain.DiffChunk{Diff: "new file mode 100644"},
			want:  "feat",
		},
		{
			name:  "source mod returns fix",
			chunk: domain.DiffChunk{Files: []string{"handler.go"}, Diff: "--- a/handler.go\n"},
			want:  "fix",
		},
		{
			name:  "empty chunk returns chore",
			chunk: domain.DiffChunk{},
			want:  "chore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := helper.InferCommitType(tt.chunk)
			if got != tt.want {
				t.Errorf("InferCommitType() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestCommitTypeHelperAdapter_CommitTypeWeight verifies CommitTypeWeight delegates correctly.
func TestCommitTypeHelperAdapter_CommitTypeWeight(t *testing.T) {
	t.Parallel()

	helper := NewCommitTypeHelperAdapter()

	tests := []struct {
		name       string
		commitType string
		want       int
	}{
		{"feat", "feat", 9},
		{"fix", "fix", 8},
		{"refactor", "refactor", 7},
		{"chore", "chore", 6},
		{"unknown", "unknown", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := helper.CommitTypeWeight(tt.commitType)
			if got != tt.want {
				t.Errorf("CommitTypeWeight(%q) = %d, want %d", tt.commitType, got, tt.want)
			}
		})
	}
}