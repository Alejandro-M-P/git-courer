package openai_standard

import (
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// ---------------------------------------------------------------------------
// extractCommitInfo smart fallback tests (REQ-CTC-002 scenarios 2.5, 2.6)
// ---------------------------------------------------------------------------

func TestExtractCommitInfo_SmartFallback(t *testing.T) {
	tests := []struct {
		name         string
		chunk        domain.DiffChunk
		wantType     string
		wantBreaking bool
	}{
		{
			name: "non_empty_commit_type_refactor",
			chunk: domain.DiffChunk{
				CommitType:      "refactor",
				ConfidenceScore: 0.80,
			},
			wantType:     "refactor",
			wantBreaking: false,
		},
		{
			name: "breaking_suffix_preserved_feat",
			chunk: domain.DiffChunk{
				CommitType:      "feat!",
				ConfidenceScore: 0.95,
			},
			wantType:     "feat",
			wantBreaking: true,
		},
		{
			name: "empty_commit_type_infers_from_diff",
			chunk: domain.DiffChunk{
				Files: []string{"cmd/server.go"},
				Diff:  "new file mode 100644\n--- /dev/null\n+++ b/cmd/server.go\n",
			},
			wantType:     "feat",
			wantBreaking: false,
		},
		{
			name: "empty_commit_type_config_infers_chore",
			chunk: domain.DiffChunk{
				Files: []string{"go.mod"},
				Diff:  "--- a/go.mod\n+++ b/go.mod\n",
			},
			wantType:     "chore",
			wantBreaking: false,
		},
		{
			name: "empty_commit_type_empty_diff_infers_chore",
			chunk: domain.DiffChunk{
				Files: []string{},
				Diff:  "",
			},
			wantType:     "chore",
			wantBreaking: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotBreaking := extractCommitInfo(tt.chunk)
			if gotType != tt.wantType {
				t.Errorf("extractCommitInfo().type = %q, want %q", gotType, tt.wantType)
			}
			if gotBreaking != tt.wantBreaking {
				t.Errorf("extractCommitInfo().breaking = %v, want %v", gotBreaking, tt.wantBreaking)
			}
		})
	}
}
