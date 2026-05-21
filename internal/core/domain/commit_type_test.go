package domain

import (
	"strings"
	"testing"
)

// TestCommitTypeWeight verifies the priority weights for commit types.
func TestCommitTypeWeight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		commitType string
		want       int
	}{
		{"feat weight 9", "feat", 9},
		{"fix weight 8", "fix", 8},
		{"refactor weight 7", "refactor", 7},
		{"chore weight 6", "chore", 6},
		{"ci weight 6", "ci", 6},
		{"docs weight 6", "docs", 6},
		{"test weight 5", "test", 5},
		{"unknown weight 0", "unknown", 0},
		{"empty weight 0", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CommitTypeWeight(tt.commitType)
			if got != tt.want {
				t.Errorf("CommitTypeWeight(%q) = %d, want %d", tt.commitType, got, tt.want)
			}
		})
	}
}

// TestInferCommitType_ClassifiedChunk verifies InferCommitType returns
// the existing CommitType (stripped of "!") when set.
func TestInferCommitType_ClassifiedChunk(t *testing.T) {
	t.Parallel()

	chunk := DiffChunk{CommitType: "feat!"}
	got := InferCommitType(chunk)
	if got != "feat" {
		t.Errorf("InferCommitType with CommitType=%q = %q, want %q", chunk.CommitType, got, "feat")
	}
}

// TestInferCommitType_NewFileDetected verifies new file mode → "feat".
func TestInferCommitType_NewFileDetected(t *testing.T) {
	t.Parallel()

	chunk := DiffChunk{Diff: "new file mode 100644\n--- /dev/null\n+++ b/cmd/server.go\n"}
	got := InferCommitType(chunk)
	if got != "feat" {
		t.Errorf("InferCommitType for new file = %q, want %q", got, "feat")
	}
}

// TestInferCommitType_DeletedFile verifies deleted file → "refactor".
func TestInferCommitType_DeletedFile(t *testing.T) {
	t.Parallel()

	chunk := DiffChunk{Diff: "deleted file mode 100644\n--- a/old.go\n+++ /dev/null\n"}
	got := InferCommitType(chunk)
	if got != "refactor" {
		t.Errorf("InferCommitType for deleted file = %q, want %q", got, "refactor")
	}
}

// TestInferCommitType_ConfigOnly verifies config-only files → "chore".
func TestInferCommitType_ConfigOnly(t *testing.T) {
	t.Parallel()

	chunk := DiffChunk{Files: []string{"go.mod"}, Diff: "--- a/go.mod\n+++ b/go.mod\n"}
	got := InferCommitType(chunk)
	if got != "chore" {
		t.Errorf("InferCommitType for go.mod = %q, want %q", got, "chore")
	}
}

// TestInferCommitType_TestFilesOnly verifies test-only files → "test".
func TestInferCommitType_TestFilesOnly(t *testing.T) {
	t.Parallel()

	chunk := DiffChunk{Files: []string{"handler_test.go"}, Diff: "--- a/handler_test.go\n+++ b/handler_test.go\n"}
	got := InferCommitType(chunk)
	if got != "test" {
		t.Errorf("InferCommitType for test file = %q, want %q", got, "test")
	}
}

// TestInferCommitType_DocsFiles verifies docs-only files → "docs".
func TestInferCommitType_DocsFiles(t *testing.T) {
	t.Parallel()

	chunk := DiffChunk{Files: []string{"README.md"}, Diff: "--- a/README.md\n+++ b/README.md\n"}
	got := InferCommitType(chunk)
	if got != "docs" {
		t.Errorf("InferCommitType for docs file = %q, want %q", got, "docs")
	}
}

// TestInferCommitType_SourceMod verifies source modifications → "fix".
func TestInferCommitType_SourceMod(t *testing.T) {
	t.Parallel()

	chunk := DiffChunk{Files: []string{"handler.go"}, Diff: "--- a/handler.go\n+++ b/handler.go\n@@ -10,3 +10,5 @@"}
	got := InferCommitType(chunk)
	if got != "fix" {
		t.Errorf("InferCommitType for source mod = %q, want %q", got, "fix")
	}
}

// TestInferCommitType_Fallback verifies absolute fallback → "chore".
func TestInferCommitType_Fallback(t *testing.T) {
	t.Parallel()

	chunk := DiffChunk{}
	got := InferCommitType(chunk)
	if got != "chore" {
		t.Errorf("InferCommitType fallback = %q, want %q", got, "chore")
	}
}

// TestInferCommitType_MixedConfigAndSource verifies mixed files still
// produce a sensible commit type.
func TestInferCommitType_MixedConfigAndSource(t *testing.T) {
	t.Parallel()

	// When files include both source and config, not ALL files match config patterns,
	// so it falls through to the "fix" path.
	chunk := DiffChunk{
		Files: []string{"handler.go", "go.mod"},
		Diff:  strings.Repeat("@@ -10,3 +10,5 @@\n", 10), // non-empty diff
	}
	got := InferCommitType(chunk)
	// "handler.go" doesn't match configFilePatterns, so allFilesMatch returns false
	// Then diff is non-empty → "fix"
	if got != "fix" {
		t.Errorf("InferCommitType for mixed = %q, want %q", got, "fix")
	}
}