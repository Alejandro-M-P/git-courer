package workflow

import (
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/shared/testutil"
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
			name: "highest_weight_wins",
			chunks: []domain.DiffChunk{
				{Files: []string{"a.go"}, Diff: "diff-a", CommitType: "feat", ConfidenceScore: 0.85},
				{Files: []string{"b.go"}, Diff: "diff-b", CommitType: "chore", ConfidenceScore: 0.92},
				{Files: []string{"c.go"}, Diff: "diff-c", CommitType: "test", ConfidenceScore: 0.95},
			},
			wantCommitType:      "feat",
			wantConfidenceScore: 0.85,
		},
		{
			name: "weight_beats_confidence",
			chunks: []domain.DiffChunk{
				{Files: []string{"a.go"}, Diff: "diff-a", CommitType: "feat", ConfidenceScore: 0.85},
				{Files: []string{"b.go"}, Diff: "diff-b", CommitType: "test", ConfidenceScore: 0.92},
			},
			wantCommitType:      "feat",
			wantConfidenceScore: 0.85,
		},
		{
			name: "confidence_tiebreaks_equal_weight",
			chunks: []domain.DiffChunk{
				{Files: []string{"a.go"}, Diff: "diff-a", CommitType: "chore", ConfidenceScore: 0.90},
				{Files: []string{"b.go"}, Diff: "diff-b", CommitType: "ci", ConfidenceScore: 0.85},
			},
			wantCommitType:      "chore",
			wantConfidenceScore: 0.90,
		},
		{
			name: "index_tiebreaks_equal_weight_confidence",
			chunks: []domain.DiffChunk{
				{Files: []string{"a.go"}, Diff: "diff-a", CommitType: "chore", ConfidenceScore: 0.85},
				{Files: []string{"b.go"}, Diff: "diff-b", CommitType: "ci", ConfidenceScore: 0.85},
			},
			wantCommitType:      "chore",
			wantConfidenceScore: 0.85,
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

// TestCombineChunks_MergesAnnotatedEntriesAndCallGraph verifies that
// combineChunks merges the structured AnnotatedEntries and CallGraph slices
// from all sub-chunks into the combined chunk (spec: combineChunks merges
// typed arrays).
func TestCombineChunks_MergesAnnotatedEntriesAndCallGraph(t *testing.T) {
	s := &CommitService{}

	chunks := []domain.DiffChunk{
		{
			Files: []string{"a.go"},
			Diff:  "diff-a",
			AnnotatedEntries: []domain.AnnotatedEntry{
				{File: "a.go", Symbol: "Alpha", Type: "NEW_FUNC", Line: 1},
			},
			CallGraph: []domain.CallGraphEntry{
				{From: "a.go", To: "b.go", Symbol: "Beta"},
			},
			CFGBefore: &domain.CFGCount{Branch: 1, Loop: 0, Return: 0, Error: 0},
			CFGAfter:  &domain.CFGCount{Branch: 2, Loop: 0, Return: 0, Error: 0},
		},
		{
			Files: []string{"b.go"},
			Diff:  "diff-b",
			AnnotatedEntries: []domain.AnnotatedEntry{
				{File: "b.go", Symbol: "Beta", Type: "MOD_SIG", Line: 5},
			},
			CallGraph: []domain.CallGraphEntry{
				{From: "b.go", To: "c.go", Symbol: "Gamma"},
			},
			CFGBefore: &domain.CFGCount{Branch: 0, Loop: 1, Return: 0, Error: 0},
			CFGAfter:  &domain.CFGCount{Branch: 0, Loop: 2, Return: 0, Error: 0},
		},
	}

	got := s.combineChunks(chunks)

	if len(got.AnnotatedEntries) != 2 {
		t.Errorf("AnnotatedEntries: got %d, want 2 (merged from both chunks)", len(got.AnnotatedEntries))
	} else {
		symbols := map[string]bool{}
		for _, e := range got.AnnotatedEntries {
			symbols[e.Symbol] = true
		}
		if !symbols["Alpha"] || !symbols["Beta"] {
			t.Errorf("AnnotatedEntries missing symbols; got %+v", got.AnnotatedEntries)
		}
	}

	if len(got.CallGraph) != 2 {
		t.Errorf("CallGraph: got %d, want 2 (merged from both chunks)", len(got.CallGraph))
	} else {
		syms := map[string]bool{}
		for _, c := range got.CallGraph {
			syms[c.Symbol] = true
		}
		if !syms["Beta"] || !syms["Gamma"] {
			t.Errorf("CallGraph missing edges; got %+v", got.CallGraph)
		}
	}

	// CFG counts are summed across sub-chunks.
	if got.CFGBefore == nil || got.CFGAfter == nil {
		t.Fatal("combined CFGBefore/CFGAfter should not be nil when sub-chunks have CFG")
	}
	if got.CFGBefore.Branch != 1 || got.CFGBefore.Loop != 1 {
		t.Errorf("CFGBefore sums wrong: %+v", got.CFGBefore)
	}
	if got.CFGAfter.Branch != 2 || got.CFGAfter.Loop != 2 {
		t.Errorf("CFGAfter sums wrong: %+v", got.CFGAfter)
	}
}

// TestCombineChunks_EmptyTypedArraysStayEmpty verifies that combining chunks
// with no structured entries/call graph yields empty (not nil-problem) slices.
func TestCombineChunks_EmptyTypedArraysStayEmpty(t *testing.T) {
	s := &CommitService{}
	got := s.combineChunks([]domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "d", CommitType: "feat"},
		{Files: []string{"b.go"}, Diff: "d", CommitType: "feat"},
	})
	if len(got.AnnotatedEntries) != 0 {
		t.Errorf("AnnotatedEntries should be empty; got %d", len(got.AnnotatedEntries))
	}
	if len(got.CallGraph) != 0 {
		t.Errorf("CallGraph should be empty; got %d", len(got.CallGraph))
	}
}

// TestPrepareStages_PreservesCFGMetadata verifies that prepareStages no longer
// clears CFGBefore/CFGAfter after classification — CFG data is now needed for
// the CFGSummary in the LLM prompt (design decision 6). AnnotatedEntries must
// also survive so the structured JSON path can render them.
func TestPrepareStages_PreservesCFGMetadata(t *testing.T) {
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: "a.go", Status: "M ", Staged: true},
			},
		},
		diffStagedResult: "diff --git a/a.go",
	}
	llm := &stubLLM{}
	security := &stubSecurity{}
	chunker := &multiChunkChunker{
		chunks: []domain.DiffChunk{
			{
				Files:    []string{"a.go"},
				Diff:     "diff a",
				CommitType: "feat",
				AnnotatedEntries: []domain.AnnotatedEntry{
					{File: "a.go", Symbol: "F", Type: "NEW_FUNC", Line: 1},
				},
				CFGBefore: &domain.CFGCount{Branch: 1, Loop: 0, Return: 0, Error: 0},
				CFGAfter:  &domain.CFGCount{Branch: 2, Loop: 0, Return: 0, Error: 0},
			},
		},
	}

	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/test.log")
	cfg.ContentProvider = testutil.NewMockContentProvider()

	svc := NewCommitService(git, llm, chunker, security, cfg, nil)

	state, err := svc.prepareStages("test")
	if err != nil {
		t.Fatalf("prepareStages failed: %v", err)
	}
	if len(state.chunks) != 1 {
		t.Fatalf("got %d chunks, want 1", len(state.chunks))
	}

	c := state.chunks[0]
	if c.CFGBefore == nil {
		t.Error("CFGBefore should NOT be cleared by prepareStages (needed for CFGSummary)")
	}
	if c.CFGAfter == nil {
		t.Error("CFGAfter should NOT be cleared by prepareStages (needed for CFGSummary)")
	}
	if c.CFGBefore != nil && c.CFGBefore.Branch != 1 {
		t.Errorf("CFGBefore.Branch = %d, want 1 (preserved)", c.CFGBefore.Branch)
	}
	if len(c.AnnotatedEntries) == 0 {
		t.Error("AnnotatedEntries should be preserved through prepareStages")
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
				Files:           []string{"api/handler.go"},
				CommitType:      "feat!",
				ConfidenceScore: 0.95,
			},
			description: "changes in api/handler.go",
			want:        "feat!: changes in api/handler.go",
		},
		{
			name: "nil_llm_with_fix_chunk",
			chunk: domain.DiffChunk{
				Files:           []string{"handler.go"},
				CommitType:      "fix",
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

func TestPrepareStages_AutoStagesMetadata(t *testing.T) {
	tests := []struct {
		name         string
		files        []domain.FileStatus
		expectStaged bool
	}{
		{
			name: "unstaged_metadata_changes_are_staged",
			files: []domain.FileStatus{
				{Path: "a.go", Status: "M ", Staged: true},
				{Path: ".git/git-courer/branches/some-branch/commits.json", Status: " M", Staged: false},
			},
			expectStaged: true,
		},
		{
			name: "no_metadata_changes_does_not_stage",
			files: []domain.FileStatus{
				{Path: "a.go", Status: "M ", Staged: true},
			},
			expectStaged: false,
		},
		{
			name: "already_staged_metadata_does_not_stage_again",
			files: []domain.FileStatus{
				{Path: "a.go", Status: "M ", Staged: true},
				{Path: ".git/git-courer/branches/some-branch/commits.json", Status: "M ", Staged: true},
			},
			expectStaged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := &stubGit{
				statusResult: domain.Status{
					Files: tt.files,
				},
				diffStagedResult: "diff --git a/a.go",
			}
			llm := &stubLLM{}
			security := &stubSecurity{}
			chunker := &multiChunkChunker{
				chunks: []domain.DiffChunk{
					{Files: []string{"a.go"}, Diff: "diff a", CommitType: "feat"},
				},
			}

			cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/test.log")
			cfg.ContentProvider = testutil.NewMockContentProvider()

			svc := NewCommitService(git, llm, chunker, security, cfg, nil)

			_, err := svc.prepareStages("test")
			if err != nil {
				t.Fatalf("prepareStages failed: %v", err)
			}

			git.mu.Lock()
			defer git.mu.Unlock()
			found := false
			for _, call := range git.addCalls {
				if len(call) == 1 && call[0] == domain.MetadataDir {
					found = true
					break
				}
			}

			if tt.expectStaged && !found {
				t.Errorf("expected metadata directory %q to be staged, but it was not", domain.MetadataDir)
			}
			if !tt.expectStaged && found {
				t.Errorf("expected metadata directory %q NOT to be staged, but it was", domain.MetadataDir)
			}
		})
	}
}

// TestMetadataDirUsedInAutoStaging verifies the metadata directory constant used in staging.
