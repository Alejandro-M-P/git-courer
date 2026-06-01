package workflow

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/shared/testutil"
)

// whyCaptureLLM captures the state at GenerateChunkMessage call time.
type whyCaptureLLM struct {
	stubLLM
	mu         sync.Mutex
	whyAtCall  string // why value when GenerateChunkMessage was called
	whyCleared bool   // set to true when ClearWhy() is called
	chunks     []domain.DiffChunk
	callCount  int
}

func (l *whyCaptureLLM) SetWhy(why string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.whyAtCall = why
}

func (l *whyCaptureLLM) ClearWhy() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.whyCleared = true
}

func (l *whyCaptureLLM) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	l.mu.Lock()
	l.callCount++
	l.chunks = append(l.chunks, chunk)
	l.mu.Unlock()
	return "feat: generated commit", nil
}

// --- GenerateCommitMessage tests ---

func TestGenerateCommitMessage_ReturnsMessages(t *testing.T) {
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a", CommitType: "feat"},
	}
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: "a.go", Status: "M ", Staged: true},
			},
		},
		diffStagedResult: "diff --git",
	}
	llm := &whyCaptureLLM{}
	security := &stubSecurity{}
	chunker := &multiChunkChunker{chunks: chunks}

	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/c.log")
	cfg.ContentProvider = testutil.NewMockContentProvider()

	svc := NewCommitService(git, llm, chunker, security, cfg, nil)
	messages, err := svc.GenerateCommitMessage(context.Background(), "fix auth bug")
	if err != nil {
		t.Fatalf("GenerateCommitMessage() error: %v", err)
	}
	if len(messages) == 0 {
		t.Error("GenerateCommitMessage() returned no messages")
	}
}

func TestGenerateCommitMessage_SetsWhy(t *testing.T) {
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a", CommitType: "feat"},
	}
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: "a.go", Status: "M ", Staged: true},
			},
		},
		diffStagedResult: "diff --git",
	}
	llm := &whyCaptureLLM{}
	security := &stubSecurity{}
	chunker := &multiChunkChunker{chunks: chunks}

	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/c.log")
	cfg.ContentProvider = testutil.NewMockContentProvider()

	svc := NewCommitService(git, llm, chunker, security, cfg, nil)
	_, err := svc.GenerateCommitMessage(context.Background(), "refactor auth")
	if err != nil {
		t.Fatalf("GenerateCommitMessage() error: %v", err)
	}
	if llm.whyAtCall != "refactor auth" {
		t.Errorf("SetWhy should have been called with 'refactor auth', got %q", llm.whyAtCall)
	}
}

func TestGenerateCommitMessage_ClearsWhyOnExit(t *testing.T) {
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a", CommitType: "feat"},
	}
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: "a.go", Status: "M ", Staged: true},
			},
		},
		diffStagedResult: "diff --git",
	}
	llm := &whyCaptureLLM{}
	security := &stubSecurity{}
	chunker := &multiChunkChunker{chunks: chunks}

	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/c.log")
	cfg.ContentProvider = testutil.NewMockContentProvider()

	svc := NewCommitService(git, llm, chunker, security, cfg, nil)
	_, err := svc.GenerateCommitMessage(context.Background(), "refactor auth")
	if err != nil {
		t.Fatalf("GenerateCommitMessage() error: %v", err)
	}
	if !llm.whyCleared {
		t.Error("ClearWhy should have been called after GenerateCommitMessage")
	}
}

func TestGenerateCommitMessage_NoGitWrites(t *testing.T) {
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a", CommitType: "feat"},
	}
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: "a.go", Status: "M ", Staged: true},
			},
		},
		diffStagedResult: "diff --git",
	}
	llm := &whyCaptureLLM{}
	security := &stubSecurity{}
	chunker := &multiChunkChunker{chunks: chunks}

	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/c.log")
	cfg.ContentProvider = testutil.NewMockContentProvider()

	svc := NewCommitService(git, llm, chunker, security, cfg, nil)
	_, err := svc.GenerateCommitMessage(context.Background(), "fix bug")
	if err != nil {
		t.Fatalf("GenerateCommitMessage() error: %v", err)
	}

	git.mu.Lock()
	commitCount := len(git.commitCalls)
	addCount := len(git.addCalls)
	git.mu.Unlock()

	if commitCount > 0 {
		t.Errorf("GenerateCommitMessage should NOT call git.Commit, but got %d calls", commitCount)
	}
	if addCount > 0 {
		t.Errorf("GenerateCommitMessage should NOT call git.Add, but got %d calls", addCount)
	}
}

func TestGenerateCommitMessage_EmptyStagingArea(t *testing.T) {
	git := &stubGit{
		statusResult:     domain.Status{Files: []domain.FileStatus{}},
		diffStagedResult: "",
	}
	llm := &whyCaptureLLM{}
	security := &stubSecurity{}
	chunker := &stubDiffChunker{}

	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/c.log")
	cfg.ContentProvider = testutil.NewMockContentProvider()

	svc := NewCommitService(git, llm, chunker, security, cfg, nil)
	_, err := svc.GenerateCommitMessage(context.Background(), "fix bug")
	if err == nil {
		t.Fatal("GenerateCommitMessage should error when nothing staged")
	}
	if !strings.Contains(err.Error(), "nothing staged") {
		t.Errorf("error should contain 'nothing staged', got: %v", err)
	}
}

func TestGenerateCommitMessage_MultipleChunks(t *testing.T) {
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a", CommitType: "feat"},
		{Files: []string{"b.go"}, Diff: "diff b", CommitType: "fix"},
	}
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: "a.go", Status: "M ", Staged: true},
				{Path: "b.go", Status: "M ", Staged: true},
			},
		},
		diffStagedResult: "diff --git",
	}
	llm := &whyCaptureLLM{}
	security := &stubSecurity{}
	chunker := &multiChunkChunker{chunks: chunks}

	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/c.log")
	cfg.NumParallel = 2
	cfg.ChunkSize = 5 // Force fallback
	cfg.ContentProvider = testutil.NewMockContentProvider()

	svc := NewCommitService(git, llm, chunker, security, cfg, nil)
	messages, err := svc.GenerateCommitMessage(context.Background(), "multi-chunk change")
	if err != nil {
		t.Fatalf("GenerateCommitMessage() error: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 unified message, got %d", len(messages))
	}
	expected := "feat: synthesized commit message"
	if messages[0] != expected {
		t.Errorf("message = %q, want %q", messages[0], expected)
	}
}

func TestGenerateCommitMessage_EmptyWhy(t *testing.T) {
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a", CommitType: "feat"},
	}
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: "a.go", Status: "M ", Staged: true},
			},
		},
		diffStagedResult: "diff --git",
	}
	llm := &whyCaptureLLM{}
	security := &stubSecurity{}
	chunker := &multiChunkChunker{chunks: chunks}

	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/c.log")
	cfg.ContentProvider = testutil.NewMockContentProvider()

	svc := NewCommitService(git, llm, chunker, security, cfg, nil)
	messages, err := svc.GenerateCommitMessage(context.Background(), "")
	if err != nil {
		t.Fatalf("GenerateCommitMessage() with empty why error: %v", err)
	}
	if len(messages) == 0 {
		t.Error("GenerateCommitMessage with empty why should still return messages")
	}
}
