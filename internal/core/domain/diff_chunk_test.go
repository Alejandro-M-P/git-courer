package domain

import (
	"strings"
	"testing"
)

// TestDiffChunk_ExtendedFields verifies CommitType and AnnotatedDiff fields.
func TestDiffChunk_ExtendedFields(t *testing.T) {
	t.Parallel()
	chunk := DiffChunk{
		Files:         []string{"internal/server/webhook.go", "internal/auth/validator.go"},
		Diff:          "diff content",
		CommitType:    "feat",
		AnnotatedDiff: "internal/server/webhook.go\nHandleWebhook [NEW_FUNC]\n+ func HandleWebhook() {}",
	}
	if chunk.CommitType != "feat" {
		t.Errorf("CommitType = %q, want feat", chunk.CommitType)
	}
	if chunk.AnnotatedDiff == "" {
		t.Error("AnnotatedDiff should not be empty")
	}
}

// TestDiffChunk_CommitTypeValues verifies all valid CommitType values.
func TestDiffChunk_CommitTypeValues(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		typ  string
	}{
		{"feat", "feat"},
		{"fix", "fix"},
		{"refactor", "refactor"},
		{"docs", "docs"},
		{"test", "test"},
		{"chore", "chore"},
		{"ci", "ci"},
		{"empty", ""},
		{"breaking-feat", "feat!"},
		{"breaking-fix", "fix!"},
		{"breaking-refactor", "refactor!"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			chunk := DiffChunk{CommitType: tc.typ}
			if chunk.CommitType != tc.typ {
				t.Errorf("CommitType = %q, want %q", chunk.CommitType, tc.typ)
			}
		})
	}
}

// TestDiffChunk_AnnotatedDiffFormat verifies grouped-by-file format.
func TestDiffChunk_AnnotatedDiffFormat(t *testing.T) {
	t.Parallel()
	annotated := "internal/server/webhook.go\n" +
		"HandleWebhook [NEW_FUNC]\n" +
		"+ func HandleWebhook(w http.ResponseWriter, r *http.Request) {\n" +
		"+     token := r.Header.Get(\"Authorization\")\n" +
		"+ }\n\n" +
		"internal/auth/validator.go\n" +
		"validateToken [MOD_BODY]\n" +
		"- if token == \"\" {\n" +
		"+ if token == \"\" || len(token) < 10 {\n"

	chunk := DiffChunk{
		Files:         []string{"internal/server/webhook.go", "internal/auth/validator.go"},
		CommitType:    "feat",
		AnnotatedDiff: annotated,
	}

	if chunk.AnnotatedDiff != annotated {
		t.Errorf("AnnotatedDiff mismatch\ngot:\n%s\nwant:\n%s", chunk.AnnotatedDiff, annotated)
	}
	// Verify it contains file group markers
	if !strings.Contains(chunk.AnnotatedDiff, "internal/server/webhook.go") {
		t.Error("AnnotatedDiff should contain first file group")
	}
	if !strings.Contains(chunk.AnnotatedDiff, "internal/auth/validator.go") {
		t.Error("AnnotatedDiff should contain second file group")
	}
	// Verify it contains labels
	if !strings.Contains(chunk.AnnotatedDiff, "[NEW_FUNC]") {
		t.Error("AnnotatedDiff should contain NEW_FUNC label")
	}
	if !strings.Contains(chunk.AnnotatedDiff, "[MOD_BODY]") {
		t.Error("AnnotatedDiff should contain MOD_BODY label")
	}
}

// TestDiffChunk_EmptyDefaults verifies zero-value behavior.
func TestDiffChunk_EmptyDefaults(t *testing.T) {
	t.Parallel()
	var chunk DiffChunk
	if chunk.CommitType != "" {
		t.Errorf("zero-value CommitType = %q, want empty string", chunk.CommitType)
	}
	if chunk.AnnotatedDiff != "" {
		t.Errorf("zero-value AnnotatedDiff = %q, want empty string", chunk.AnnotatedDiff)
	}
	if chunk.Diff != "" {
		t.Errorf("zero-value Diff = %q, want empty string", chunk.Diff)
	}
	if len(chunk.Files) != 0 {
		t.Errorf("zero-value len(Files) = %d, want 0", len(chunk.Files))
	}
	if chunk.ConfidenceScore != 0.0 {
		t.Errorf("zero-value ConfidenceScore = %f, want 0.0", chunk.ConfidenceScore)
	}
}

// TestDiffChunk_ConfidenceScore validates the ConfidenceScore field range (0.0–1.0).
func TestDiffChunk_ConfidenceScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		score float64
	}{
		{"high confidence", 0.95},
		{"medium confidence", 0.75},
		{"low confidence", 0.30},
		{"zero confidence", 0.0},
		{"max confidence", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			chunk := DiffChunk{
				Files:           []string{"a.go"},
				ConfidenceScore: tt.score,
			}
			if chunk.ConfidenceScore != tt.score {
				t.Errorf("ConfidenceScore = %f, want %f", chunk.ConfidenceScore, tt.score)
			}
		})
	}
}
