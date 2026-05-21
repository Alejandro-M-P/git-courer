package workflow

import (
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