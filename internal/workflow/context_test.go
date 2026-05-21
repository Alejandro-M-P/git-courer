package workflow

import (
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// contextTrackingLLM is a test double that records SetContext calls.
type contextTrackingLLM struct {
	stubLLM
	mu              sync.Mutex
	contextSet      string
	whySet          string
	changelogCalls  int
	commitCalls     int
	changelogResult *domain.Changelog
}

func (l *contextTrackingLLM) SetContext(ctx string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.contextSet = ctx
}

func (l *contextTrackingLLM) SetWhy(why string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.whySet = why
}

func (l *contextTrackingLLM) ClearWhy() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.whySet = ""
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

func (l *contextTrackingLLM) GenerateChangelogByArea(formattedGroups string) (domain.ChangelogByArea, error) {
	l.mu.Lock()
	l.changelogCalls++
	l.mu.Unlock()
	return domain.ChangelogByArea{}, nil
}

func (l *contextTrackingLLM) GenerateChangelogGeneric(commits, previousChangelog, outputFile string) (*domain.Changelog, error) {
	l.mu.Lock()
	l.changelogCalls++
	l.mu.Unlock()
	return &domain.Changelog{}, nil
}


// --- ProjectConfig scope injection ---

func TestNewCommitService_ProjectConfigScopeInjection(t *testing.T) {
	// Create a temporary project config
	tmpDir := t.TempDir()
	config := &domain.ProjectConfig{
		Description: "Test project for scope injection",
		Areas: map[string][]string{
			"core": {"internal/core/"},
		},
	}
	if err := config.Save(tmpDir); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Change to temp dir so LoadProjectConfig(".") finds it
	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	defer os.Chdir(origDir)

	git := &stubGit{}
	llm := &contextTrackingLLM{}
	chunker := &stubDiffChunker{}
	security := &stubSecurity{}
	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/c.log")
	// Intentionally empty Context — should be populated from ProjectConfig
	cfg.Context = ""

	_ = NewCommitService(git, llm, chunker, security, cfg)

	if llm.contextSet == "" {
		t.Error("SetContext should be called with project scope when ProjectConfig exists")
	}
	if !strings.Contains(llm.contextSet, "Test project for scope injection") {
		t.Errorf("contextSet = %q, want to contain project description", llm.contextSet)
	}
	if !strings.Contains(llm.contextSet, "core") {
		t.Errorf("contextSet = %q, want to contain area 'core'", llm.contextSet)
	}
}

func TestNewCommitService_ContextConfigTakesPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	config := &domain.ProjectConfig{
		Description: "Project config description",
		Areas:       map[string][]string{"auth": {"internal/auth/"}},
	}
	if err := config.Save(tmpDir); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	defer os.Chdir(origDir)

	git := &stubGit{}
	llm := &contextTrackingLLM{}
	chunker := &stubDiffChunker{}
	security := &stubSecurity{}
	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/c.log")
	// Explicit cfg.Context should take precedence over ProjectConfig
	cfg.Context = "explicit context override"

	_ = NewCommitService(git, llm, chunker, security, cfg)

	if llm.contextSet != "explicit context override" {
		t.Errorf("contextSet = %q, want 'explicit context override' (explicit cfg.Context takes precedence)", llm.contextSet)
	}
}

func TestNewCommitService_NoProjectConfig_NoError(t *testing.T) {
	tmpDir := t.TempDir()
	// No config file created — should not set context

	origDir, _ := os.Getwd()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir() error: %v", err)
	}
	defer os.Chdir(origDir)

	git := &stubGit{}
	llm := &contextTrackingLLM{}
	chunker := &stubDiffChunker{}
	security := &stubSecurity{}
	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/c.log")

	_ = NewCommitService(git, llm, chunker, security, cfg)

	if llm.contextSet != "" {
		t.Errorf("contextSet = %q, want empty string when no ProjectConfig exists", llm.contextSet)
	}
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

	svc := NewReleaseService(git, llm, chunker, cfg, nil)
	svc.SetContext(cfg.Context)

	_, _, _, err := svc.Generate("feat: something")
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

// --- SetWhy/ClearWhy on CommitService ---

func TestCommitService_SetWhy_PropagatesToLLM(t *testing.T) {
	git := &stubGit{}
	llm := &contextTrackingLLM{}
	chunker := &stubDiffChunker{}
	security := &stubSecurity{}
	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/c.log")

	svc := NewCommitService(git, llm, chunker, security, cfg)
	svc.SetWhy("refactor login")

	if llm.whySet != "refactor login" {
		t.Errorf("SetWhy called: whySet = %q, want 'refactor login'", llm.whySet)
	}
}

func TestCommitService_SetWhy_NoPanicOnUnsupportedLLM(t *testing.T) {
	git := &stubGit{}
	llm := &stubLLM{} // stubLLM does NOT implement SetWhy
	chunker := &stubDiffChunker{}
	security := &stubSecurity{}
	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/c.log")

	svc := NewCommitService(git, llm, chunker, security, cfg)
	// Must not panic
	svc.SetWhy("refactor login")
}

func TestCommitService_ClearWhy_ResetsWhy(t *testing.T) {
	git := &stubGit{}
	llm := &contextTrackingLLM{}
	chunker := &stubDiffChunker{}
	security := &stubSecurity{}
	cfg := DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/c.log")

	svc := NewCommitService(git, llm, chunker, security, cfg)
	svc.SetWhy("refactor login")
	svc.ClearWhy()

	if llm.whySet != "" {
		t.Errorf("ClearWhy called: whySet = %q, want empty string", llm.whySet)
	}
}
