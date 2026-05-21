package workflow

import (
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// ---------------------------------------------------------------------------
// combineChunks type preservation tests (REQ-CTC-001)
// ---------------------------------------------------------------------------

func TestCombineChunksTypePreservation(t *testing.T) {
	s := &CommitService{}

	tests := []struct {
		name                string
		chunks              []domain.DiffChunk
		wantCommitType      string
		wantConfidenceScore float64
	}{
		{
			name: "highest_confidence_wins",
			chunks: []domain.DiffChunk{
				{Files: []string{"a.go"}, Diff: "diff-a", CommitType: "feat", ConfidenceScore: 0.95},
				{Files: []string{"b.go"}, Diff: "diff-b", CommitType: "chore", ConfidenceScore: 0.85},
				{Files: []string{"c.go"}, Diff: "diff-c", CommitType: "fix", ConfidenceScore: 0.80},
			},
			wantCommitType:      "feat",
			wantConfidenceScore: 0.95,
		},
		{
			name: "all_empty_commit_type",
			chunks: []domain.DiffChunk{
				{Files: []string{"a.go"}, Diff: "diff-a", CommitType: "", ConfidenceScore: 0.0},
				{Files: []string{"b.go"}, Diff: "diff-b", CommitType: "", ConfidenceScore: 0.0},
			},
			wantCommitType:      "",
			wantConfidenceScore: 0.0,
		},
		{
			name: "breaking_suffix_preserved",
			chunks: []domain.DiffChunk{
				{Files: []string{"a.go"}, Diff: "diff-a", CommitType: "refactor!", ConfidenceScore: 0.90},
				{Files: []string{"b.go"}, Diff: "diff-b", CommitType: "feat", ConfidenceScore: 0.95},
			},
			wantCommitType:      "feat!",
			wantConfidenceScore: 0.95,
		},
		{
			name: "tie_break_by_weight",
			chunks: []domain.DiffChunk{
				{Files: []string{"a.go"}, Diff: "diff-a", CommitType: "fix", ConfidenceScore: 0.85},
				{Files: []string{"b.go"}, Diff: "diff-b", CommitType: "refactor", ConfidenceScore: 0.85},
			},
			wantCommitType:      "fix",
			wantConfidenceScore: 0.85,
		},
		{
			name: "single_chunk_passthrough",
			chunks: []domain.DiffChunk{
				{Files: []string{"docs.md"}, Diff: "diff-docs", CommitType: "docs", ConfidenceScore: 0.90},
			},
			wantCommitType:      "docs",
			wantConfidenceScore: 0.90,
		},
		{
			name:                "empty_slice_zero_value",
			chunks:              []domain.DiffChunk{},
			wantCommitType:      "",
			wantConfidenceScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.combineChunks(tt.chunks)
			if got.CommitType != tt.wantCommitType {
				t.Errorf("combineChunks().CommitType = %q, want %q", got.CommitType, tt.wantCommitType)
			}
			if got.ConfidenceScore != tt.wantConfidenceScore {
				t.Errorf("combineChunks().ConfidenceScore = %f, want %f", got.ConfidenceScore, tt.wantConfidenceScore)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// formatFallbackMessage tests (REQ-CTC-003)
// ---------------------------------------------------------------------------

func TestFormatFallbackMessage(t *testing.T) {
	tests := []struct {
		name        string
		chunk       domain.DiffChunk
		description string
		want        string
	}{
		{
			name: "classified_chunk_feat",
			chunk: domain.DiffChunk{
				Files:      []string{"cmd/server.go"},
				CommitType: "feat",
			},
			description: "changes in cmd/server.go",
			want:        "feat: changes in cmd/server.go",
		},
		{
			name: "empty_commit_type_with_new_file",
			chunk: domain.DiffChunk{
				Files: []string{"cmd/server.go"},
				Diff:  "new file mode 100644\n--- /dev/null\n+++ b/cmd/server.go\n",
			},
			description: "changes in cmd/server.go",
			want:        "feat: changes in cmd/server.go",
		},
		{
			name: "empty_commit_type_config_only",
			chunk: domain.DiffChunk{
				Files: []string{"go.mod"},
				Diff:  "--- a/go.mod\n+++ b/go.mod\n",
			},
			description: "changes in go.mod",
			want:        "chore: changes in go.mod",
		},
		{
			name: "breaking_change_feat",
			chunk: domain.DiffChunk{
				Files:         []string{"api/handler.go"},
				CommitType:    "feat!",
				ConfidenceScore: 0.95,
			},
			description: "changes in api/handler.go",
			want:        "feat!: changes in api/handler.go",
		},
		{
			name: "nil_llm_with_fix_chunk",
			chunk: domain.DiffChunk{
				Files:         []string{"handler.go"},
				CommitType:    "fix",
				ConfidenceScore: 0.85,
			},
			description: "changes in handler.go",
			want:        "fix: changes in handler.go",
		},
		{
			name: "empty_commit_type_source_mods",
			chunk: domain.DiffChunk{
				Files: []string{"handler.go"},
				Diff:  "--- a/handler.go\n+++ b/handler.go\n@@ -10,3 +10,5 @@\n",
			},
			description: "changes in handler.go",
			want:        "fix: changes in handler.go",
		},
		{
			name: "synthesis_fallback_with_feat",
			chunk: domain.DiffChunk{
				CommitType: "feat",
			},
			description: "update staged files",
			want:        "feat: update staged files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFallbackMessage(tt.chunk, tt.description)
			if got != tt.want {
				t.Errorf("formatFallbackMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Nil-LLM paths use type-aware messages (REQ-CTC-003 part 2)
// These tests verify that generateMessages and prepareChunksAndMessages
// use formatFallbackMessage instead of hardcoded "chore:".
// ---------------------------------------------------------------------------

func TestGenerateMessages_NilLLM_UsesTypeAwareFallback(t *testing.T) {
	s := &CommitService{cfg: CommitServiceConfig{ChunkSize: 4000}}

	chunks := []domain.DiffChunk{
		{Files: []string{"cmd/server.go"}, Diff: "new file mode 100644\n--- /dev/null\n+++ b/cmd/server.go\n", CommitType: "feat", ConfidenceScore: 0.90},
		{Files: []string{"handler.go"}, Diff: "--- a/handler.go\n+++ b/handler.go\n"},
		{Files: []string{"go.mod"}, Diff: "--- a/go.mod\n+++ b/go.mod\n", CommitType: "chore", ConfidenceScore: 0.85},
	}

	messages, warnings := s.generateMessages(chunks, "", "")
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}
	// Chunk 0 has CommitType="feat" → "feat: changes in cmd/server.go"
	if !strings.HasPrefix(messages[0], "feat:") {
		t.Errorf("message[0] = %q, want prefix 'feat:'", messages[0])
	}
	// Chunk 1 has empty CommitType, source mod → "fix: changes in handler.go"
	if !strings.HasPrefix(messages[1], "fix:") {
		t.Errorf("message[1] = %q, want prefix 'fix:'", messages[1])
	}
	// Chunk 2 has CommitType="chore" → "chore: changes in go.mod"
	if !strings.HasPrefix(messages[2], "chore:") {
		t.Errorf("message[2] = %q, want prefix 'chore:'", messages[2])
	}
}