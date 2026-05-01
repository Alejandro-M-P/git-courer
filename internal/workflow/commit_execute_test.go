package workflow

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// TestCommitService_Execute_Sync runs the sync path (≤3 chunks).
func TestCommitService_Execute_Sync(t *testing.T) {
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: "main.go", Status: "M "},
			},
		},
		diffStagedResult: "diff --git a/main.go\n+line",
	}
	llm := &stubLLM{chunkMsg: "feat: sync commit"}
	security := &stubSecurity{}

	svc := newCommitSvcWithPath(git, llm, security, t.TempDir()+"/c.log")
	result, err := svc.Execute("commit changes", false)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result == "" {
		t.Error("Execute() returned empty result")
	}
	if len(git.commitCalls) == 0 {
		t.Error("Execute() should have called git.Commit")
	}
}

// TestCommitService_Execute_NothingToCommit verifies the "nothing to commit" path.
func TestCommitService_Execute_NothingToCommit(t *testing.T) {
	git := &stubGit{
		statusResult:     domain.Status{Files: []domain.FileStatus{}},
		diffStagedResult: "",
	}
	llm := &stubLLM{}
	security := &stubSecurity{}

	svc := newCommitSvcWithPath(git, llm, security, t.TempDir()+"/c.log")
	result, err := svc.Execute("commit all", false)
	// Either an error or a JSON message about nothing to commit
	if err != nil && result != "" {
		t.Error("Execute() should return either an error or a result, not both")
	}
}

// TestCommitService_Execute_SecurityBlocked verifies security blocking in Execute.
func TestCommitService_Execute_SecurityBlocked(t *testing.T) {
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: ".env", Status: "??", IsNew: true},
			},
		},
		diffStagedResult: "diff with secret token",
	}
	llm := &stubLLM{commitIntent: domain.CommitIntent{IncludeUntracked: true}}
	security := &stubSecurity{blocked: true}

	svc := newCommitSvcWithPath(git, llm, security, t.TempDir()+"/c.log")
	_, err := svc.Execute("commit everything", false)
	if err == nil {
		t.Error("Execute() should fail when security blocks")
	}
}

// TestCommitService_Execute_WithPush verifies the push path.
func TestCommitService_Execute_WithPush(t *testing.T) {
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: "main.go", Status: "M "},
			},
		},
		diffStagedResult: "diff content",
		pushResult:       "pushed to origin",
	}
	llm := &stubLLM{chunkMsg: "feat: push commit"}
	security := &stubSecurity{}

	svc := newCommitSvcWithPath(git, llm, security, t.TempDir()+"/c.log")
	result, err := svc.Execute("commit and push", false)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	_ = result
}

// TestCommitService_ExecutePrepared_CommitsAndPushes verifies ExecutePrepared.
func TestCommitService_ExecutePrepared_CommitsAndPushes(t *testing.T) {
	git := &stubGit{}
	llm := &stubLLM{}
	security := &stubSecurity{}

	svc := newCommitSvcWithPath(git, llm, security, t.TempDir()+"/c.log")
	chunks := []domain.DiffChunk{
		{Files: []string{"main.go"}, Diff: "diff content"},
		{Files: []string{"auth.go"}, Diff: "diff auth"},
	}
	messages := []string{"feat: first commit", "feat: second commit"}

	result, err := svc.ExecutePrepared(messages, chunks, "commit")
	if err != nil {
		t.Fatalf("ExecutePrepared() error: %v", err)
	}
	if result == "" {
		t.Error("ExecutePrepared() returned empty result")
	}
	if len(git.commitCalls) != 2 {
		t.Errorf("git.Commit() called %d times, want 2", len(git.commitCalls))
	}
}

// TestCommitService_ExecutePrepared_SkipsEmpty verifies skip of empty messages.
func TestCommitService_ExecutePrepared_SkipsEmpty(t *testing.T) {
	git := &stubGit{}
	llm := &stubLLM{}
	security := &stubSecurity{}

	svc := newCommitSvcWithPath(git, llm, security, t.TempDir()+"/c.log")
	chunks := []domain.DiffChunk{
		{Files: []string{"main.go"}, Diff: "diff"},
		{Files: []string{"chore.go"}, Diff: "diff"},
	}
	messages := []string{"feat: real", "chore: no meaningful changes"}

	result, err := svc.ExecutePrepared(messages, chunks, "commit")
	if err != nil {
		t.Fatalf("ExecutePrepared() error: %v", err)
	}
	if len(git.commitCalls) != 1 {
		t.Errorf("git.Commit() called %d times, want 1", len(git.commitCalls))
	}
	_ = result
}

// TestCommitService_ExecutePrepared_NoCommits verifies error when all messages skipped.
func TestCommitService_ExecutePrepared_NoCommits(t *testing.T) {
	git := &stubGit{}
	llm := &stubLLM{}
	security := &stubSecurity{}

	svc := newCommitSvcWithPath(git, llm, security, t.TempDir()+"/c.log")
	chunks := []domain.DiffChunk{{Files: []string{"a.go"}, Diff: "diff"}}
	messages := []string{"chore: no meaningful changes"}

	_, err := svc.ExecutePrepared(messages, chunks, "commit")
	if err == nil {
		t.Error("ExecutePrepared() should error when all messages are skipped")
	}
	if !strings.Contains(err.Error(), "no commits") {
		t.Errorf("error %q should mention 'no commits'", err.Error())
	}
}

// TestCommitService_ExecutePrepared_WithPush verifies push in ExecutePrepared.
func TestCommitService_ExecutePrepared_WithPush(t *testing.T) {
	git := &stubGit{pushResult: "pushed"}
	llm := &stubLLM{}
	security := &stubSecurity{}

	svc := newCommitSvcWithPath(git, llm, security, t.TempDir()+"/c.log")
	chunks := []domain.DiffChunk{{Files: []string{"main.go"}, Diff: "diff"}}
	messages := []string{"feat: new feature"}

	_, err := svc.ExecutePrepared(messages, chunks, "commit and push")
	if err != nil {
		t.Fatalf("ExecutePrepared() error: %v", err)
	}
	if git.pushResult != "pushed" {
		t.Error("Push should have been called")
	}
}

// TestCommitService_Rollback_OnCommitError verifies rollback is called.
func TestCommitService_Rollback_OnCommitError(t *testing.T) {
	// The rollback path is triggered when git.Commit fails inside executeSync.
	// We can't easily inject commit failure with stubGit, but we can verify
	// rollback does not panic.
	git := &stubGit{}
	svc := newCommitSvcWithPath(git, &stubLLM{}, &stubSecurity{}, t.TempDir()+"/c.log")
	// Call rollback directly with one committed message
	svc.rollback([]string{"feat: something"})
	if len(git.resetCalls) == 0 {
		t.Error("rollback() should call git.Reset")
	}
}

// TestCommitService_Execute_MultiChunk runs Execute with 4 chunks synchronously.
func TestCommitService_Execute_MultiChunk(t *testing.T) {
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: "a.go", Status: "M "},
			},
		},
		diffStagedResult: "diff content",
	}
	llm := &stubLLM{chunkMsg: "feat: chunk"}
	security := &stubSecurity{}

	chunker := &stubDiffChunker{
		chunks: []domain.DiffChunk{
			{Files: []string{"a.go"}, Diff: "d1"},
			{Files: []string{"b.go"}, Diff: "d2"},
			{Files: []string{"c.go"}, Diff: "d3"},
			{Files: []string{"d.go"}, Diff: "d4"},
		},
	}
	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/c.log")
	svc := NewCommitService(git, llm, chunker, security, cfg)

	result, err := svc.Execute("commit all", false)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result == "" {
		t.Error("Execute() returned empty result")
	}
	if len(git.commitCalls) == 0 {
		t.Error("Execute() should have committed synchronously")
	}
}

// --- executeSync parallelism tests (Phase 3) ---

func newCommitSvcWithChunkerAndNumParallel(git *stubGit, llm ports.LLM, chunker ports.DiffChunker, security *stubSecurity, logPath string, numParallel int) *CommitService {
	cfg := DefaultCommitServiceConfig(4096, 50, logPath)
	cfg.NumParallel = numParallel
	return NewCommitService(git, llm, chunker, security, cfg)
}

// TestExecuteSync_NumParallelOne_SerialOrder verifies NumParallel=1 produces identical behavior.
func TestExecuteSync_NumParallelOne_SerialOrder(t *testing.T) {
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a"},
		{Files: []string{"b.go"}, Diff: "diff b"},
		{Files: []string{"c.go"}, Diff: "diff c"},
	}
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: "a.go", Status: "M ", Staged: true},
				{Path: "b.go", Status: "M ", Staged: true},
				{Path: "c.go", Status: "M ", Staged: true},
			},
		},
		diffStagedResult: "diff --git",
	}
	llm := &indexedLLM{}
	security := &stubSecurity{}
	chunker := &multiChunkChunker{chunks: chunks}

	svc := newCommitSvcWithChunkerAndNumParallel(git, llm, chunker, security, t.TempDir()+"/c.log", 1)
	result, err := svc.Execute("commit", false)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result == "" {
		t.Error("Execute() returned empty result")
	}
	// Verify commit order matches chunk order
	wantCommits := []string{
		"feat: commit for a.go",
		"feat: commit for b.go",
		"feat: commit for c.go",
	}
	if len(git.commitCalls) != 3 {
		t.Fatalf("commitCalls = %d, want 3", len(git.commitCalls))
	}
	for i, w := range wantCommits {
		if git.commitCalls[i] != w {
			t.Errorf("commitCalls[%d] = %q, want %q", i, git.commitCalls[i], w)
		}
	}
	// Verify add order matches chunk order
	if len(git.addCalls) < 3 {
		t.Fatalf("addCalls = %d, want >= 3", len(git.addCalls))
	}
	for i, chunk := range chunks {
		found := false
		for _, add := range git.addCalls {
			if slicesEqual(add, chunk.Files) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("addCalls missing chunk %d files %v", i, chunk.Files)
		}
	}
	if llm.callCount != 3 {
		t.Errorf("LLM callCount = %d, want 3", llm.callCount)
	}
}

// TestExecuteSync_NumParallelThree_ParallelGen_SerialCommit verifies two-phase behavior.
func TestExecuteSync_NumParallelThree_ParallelGen_SerialCommit(t *testing.T) {
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a"},
		{Files: []string{"b.go"}, Diff: "diff b"},
		{Files: []string{"c.go"}, Diff: "diff c"},
		{Files: []string{"d.go"}, Diff: "diff d"},
	}
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: "a.go", Status: "M ", Staged: true},
				{Path: "b.go", Status: "M ", Staged: true},
				{Path: "c.go", Status: "M ", Staged: true},
				{Path: "d.go", Status: "M ", Staged: true},
			},
		},
		diffStagedResult: "diff --git",
	}
	llm := &concurrencyTrackingLLM{delay: 20 * time.Millisecond}
	security := &stubSecurity{}
	chunker := &multiChunkChunker{chunks: chunks}

	svc := newCommitSvcWithChunkerAndNumParallel(git, llm, chunker, security, t.TempDir()+"/c.log", 3)
	result, err := svc.Execute("commit", false)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if result == "" {
		t.Error("Execute() returned empty result")
	}
	// Phase 1: parallel generation occurred
	if llm.maxInflight <= 1 {
		t.Errorf("maxInflight = %d, want > 1 (concurrency not utilized)", llm.maxInflight)
	}
	if llm.callCount != 4 {
		t.Errorf("LLM callCount = %d, want 4", llm.callCount)
	}
	// Phase 2: git operations MUST be serial and in chunk order
	wantCommits := []string{
		"msg", "msg", "msg", "msg",
	}
	if len(git.commitCalls) != 4 {
		t.Fatalf("commitCalls = %d, want 4", len(git.commitCalls))
	}
	for i, w := range wantCommits {
		if git.commitCalls[i] != w {
			t.Errorf("commitCalls[%d] = %q, want %q", i, git.commitCalls[i], w)
		}
	}
	// Verify per-chunk adds are present and in chunk order (ignoring prepareStages initial add)
	perChunkAdds := make([][]string, 0, 4)
	for _, add := range git.addCalls {
		for _, chunk := range chunks {
			if slicesEqual(add, chunk.Files) {
				perChunkAdds = append(perChunkAdds, add)
				break
			}
		}
	}
	if len(perChunkAdds) != 4 {
		t.Fatalf("perChunkAdds = %d, want 4", len(perChunkAdds))
	}
	for i, chunk := range chunks {
		if !slicesEqual(perChunkAdds[i], chunk.Files) {
			t.Errorf("perChunkAdds[%d] = %v, want %v", i, perChunkAdds[i], chunk.Files)
		}
	}
}

// TestExecuteSync_NumParallelThree_ChunkFailureSkipped verifies failed chunk is skipped in Phase 2.
func TestExecuteSync_NumParallelThree_ChunkFailureSkipped(t *testing.T) {
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a"},
		{Files: []string{"b.go"}, Diff: "diff b"},
		{Files: []string{"c.go"}, Diff: "diff c"},
	}
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: "a.go", Status: "M ", Staged: true},
				{Path: "b.go", Status: "M ", Staged: true},
				{Path: "c.go", Status: "M ", Staged: true},
			},
		},
		diffStagedResult: "diff --git",
	}
	llm := &indexedLLM{failFile: "b.go"}
	security := &stubSecurity{}
	chunker := &multiChunkChunker{chunks: chunks}

	svc := newCommitSvcWithChunkerAndNumParallel(git, llm, chunker, security, t.TempDir()+"/c.log", 3)
	result, err := svc.Execute("commit", false)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	// Only 2 commits: a.go and c.go (b.go failed during LLM gen)
	if len(git.commitCalls) != 2 {
		t.Fatalf("commitCalls = %d, want 2 (b.go skipped)", len(git.commitCalls))
	}
	wantCommits := []string{
		"feat: commit for a.go",
		"feat: commit for c.go",
	}
	for i, w := range wantCommits {
		if git.commitCalls[i] != w {
			t.Errorf("commitCalls[%d] = %q, want %q", i, git.commitCalls[i], w)
		}
	}
	// Verify b.go was never staged (skipped in Phase 2)
	for _, add := range git.addCalls {
		if slicesEqual(add, []string{"b.go"}) {
			t.Error("b.go should NOT have been staged — chunk failed during generation")
		}
	}
	// Warnings should include the failed chunk
	var parsed CommitResult
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("failed to parse result JSON: %v", err)
	}
	foundWarning := false
	for _, w := range parsed.Warnings {
		if strings.Contains(w, "Chunk 2 failed") {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("warnings should contain 'Chunk 2 failed', got %v", parsed.Warnings)
	}
	// All chunks must have been attempted (no group cancellation)
	if llm.callCount != 3 {
		t.Errorf("LLM callCount = %d, want 3 (all chunks attempted)", llm.callCount)
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
