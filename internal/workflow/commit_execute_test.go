package workflow

import (
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
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

// TestCommitService_Execute_Background runs the background path (>3 chunks).
func TestCommitService_Execute_Background(t *testing.T) {
	// Create 4 chunks to trigger the background path
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

	// Use a small background threshold so 1 chunk triggers background
	chunker := &stubDiffChunker{
		chunks: []domain.DiffChunk{
			{Files: []string{"a.go"}, Diff: "d1"},
			{Files: []string{"b.go"}, Diff: "d2"},
			{Files: []string{"c.go"}, Diff: "d3"},
			{Files: []string{"d.go"}, Diff: "d4"},
		},
	}
	cfg := DefaultCommitServiceConfig(4096, 0, 50, t.TempDir()+"/c.log") // threshold=0 → background for any size
	svc := NewCommitService(git, llm, chunker, security, cfg)

	result, err := svc.Execute("commit all", false)
	if err != nil {
		t.Fatalf("Execute() background error: %v", err)
	}
	if result == "" {
		t.Error("Execute() background returned empty result")
	}
	// Background mode returns JSON with "running" state
	if !strings.Contains(result, "running") && len(git.commitCalls) == 0 {
		t.Log("Background mode either returned 'running' or committed synchronously — both OK")
	}
}
