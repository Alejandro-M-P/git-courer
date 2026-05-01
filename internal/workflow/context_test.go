package workflow

import (
	"sync"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// contextTrackingLLM is a test double that records SetContext calls.
type contextTrackingLLM struct {
	stubLLM
	mu              sync.Mutex
	contextSet      string
	changelogCalls  int
	commitCalls     int
	changelogResult *domain.Changelog
}

func (l *contextTrackingLLM) SetContext(ctx string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.contextSet = ctx
}

func (l *contextTrackingLLM) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	l.mu.Lock()
	l.commitCalls++
	l.mu.Unlock()
	return l.stubLLM.GenerateChunkMessage(chunk)
}

func (l *contextTrackingLLM) GenerateChangelog(commits, previousChangelog, outputFile string) (*domain.Changelog, error) {
	l.mu.Lock()
	l.changelogCalls++
	l.mu.Unlock()
	if l.changelogResult != nil {
		return l.changelogResult, nil
	}
	return l.stubLLM.GenerateChangelog(commits, previousChangelog, outputFile)
}

// --- 5.1 CommitServiceConfig carries context ---

func TestCommitServiceConfig_ContextFieldExists(t *testing.T) {
	cfg := CommitServiceConfig{Context: "Project: X"}
	if cfg.Context != "Project: X" {
		t.Errorf("Context = %q, want Project: X", cfg.Context)
	}
}

func TestCommitService_SetContext_SetsOnLLM(t *testing.T) {
	git := &stubGit{}
	llm := &contextTrackingLLM{}
	chunker := &stubDiffChunker{}
	security := &stubSecurity{}
	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/c.log")
	cfg.Context = "test project context"

	svc := NewCommitService(git, llm, chunker, security, cfg)
	svc.SetContext(cfg.Context)

	if llm.contextSet != "test project context" {
		t.Errorf("SetContext called with %q, want test project context", llm.contextSet)
	}
}

func TestCommitService_SetContext_EmptyString_Allowed(t *testing.T) {
	git := &stubGit{}
	llm := &contextTrackingLLM{}
	chunker := &stubDiffChunker{}
	security := &stubSecurity{}
	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/c.log")
	cfg.Context = ""

	svc := NewCommitService(git, llm, chunker, security, cfg)
	svc.SetContext(cfg.Context)

	if llm.contextSet != "" {
		t.Errorf("SetContext called with %q, want empty string", llm.contextSet)
	}
}

func TestCommitService_PrepareCommit_CallsSetContext(t *testing.T) {
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}, Diff: "diff a"},
	}
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: "a.go", Status: "M ", Staged: true},
			},
		},
		diffStagedResult: "diff --git",
	}
	llm := &contextTrackingLLM{}
	security := &stubSecurity{}
	chunker := &multiChunkChunker{chunks: chunks}

	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/c.log")
	cfg.Context = "commit context"
	cfg.NumParallel = 1

	svc := NewCommitService(git, llm, chunker, security, cfg)
	svc.SetContext(cfg.Context)

	_, _, _, _, _, err := svc.PrepareCommit("commit")
	if err != nil {
		t.Fatalf("PrepareCommit() error: %v", err)
	}

	if llm.contextSet != "commit context" {
		t.Errorf("contextSet = %q, want commit context", llm.contextSet)
	}
	if llm.commitCalls < 1 {
		t.Errorf("commitCalls = %d, want >=1", llm.commitCalls)
	}
}

// --- 5.3 ReleaseServiceConfig carries context ---

func TestReleaseServiceConfig_ContextFieldExists(t *testing.T) {
	cfg := ReleaseServiceConfig{Context: "Project: Y"}
	if cfg.Context != "Project: Y" {
		t.Errorf("Context = %q, want Project: Y", cfg.Context)
	}
}

func TestReleaseService_SetContext_SetsOnLLM(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &contextTrackingLLM{}
	chunker := &mockLogChunker{}
	cfg := DefaultReleaseServiceConfig(4096, 20, 100, t.TempDir()+"/release.log")
	cfg.Context = "release context"

	svc := NewReleaseService(git, llm, chunker, cfg, nil)
	svc.SetContext(cfg.Context)

	if llm.contextSet != "release context" {
		t.Errorf("SetContext called with %q, want release context", llm.contextSet)
	}
}

func TestReleaseService_Generate_CallsSetContext(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &contextTrackingLLM{changelogResult: &domain.Changelog{Features: []string{"Changes"}}}
	chunker := &mockLogChunker{}
	cfg := DefaultReleaseServiceConfig(4096, 20, 100, t.TempDir()+"/release.log")
	cfg.Context = "gen context"
	cfg.NumParallel = 1
	cfg.BackgroundThreshold = 10 // force sync path

	svc := NewReleaseService(git, llm, chunker, cfg, nil)
	svc.SetContext(cfg.Context)

	_, _, err := svc.Generate("feat: something")
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	if llm.contextSet != "gen context" {
		t.Errorf("contextSet = %q, want gen context", llm.contextSet)
	}
	if llm.changelogCalls < 1 {
		t.Errorf("changelogCalls = %d, want >=1", llm.changelogCalls)
	}
}
