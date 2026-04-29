package workflow

import (
	"fmt"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// --- Mocks ---

type stubGit struct {
	statusResult   domain.Status
	statusErr      error
	diffResult     string
	diffStagedResult string
	untrackedResult  []string
	commitCalls    []string
	addCalls       [][]string
	resetCalls     []string
	pushResult     string
	pushErr        error
}

func (s *stubGit) Status() (domain.Status, error)           { return s.statusResult, s.statusErr }
func (s *stubGit) Diff(paths ...string) (string, error)         { return s.diffResult, nil }
func (s *stubGit) DiffStaged(paths ...string) (string, error) { return s.diffStagedResult, nil }
func (s *stubGit) ListUntracked() ([]string, error)         { return s.untrackedResult, nil }
func (s *stubGit) Log(limit int, paths ...string) (string, error) { return "", nil }
func (s *stubGit) LogFull(limit int) (string, error)        { return "", nil }
func (s *stubGit) CurrentBranch() (string, error)           { return "main", nil }
func (s *stubGit) ListBranches(pattern ...string) (string, error) { return "main", nil }
func (s *stubGit) ListTags(pattern ...string) ([]string, error) { return nil, nil }
func (s *stubGit) IsRepo() bool                       { return true }
func (s *stubGit) LatestTag() (string, error)           { return "", nil }
func (s *stubGit) RemoteURL() (string, error)           { return "", nil }
func (s *stubGit) CommitsFromTag(sinceTag string) (string, error) { return "", nil }
func (s *stubGit) TagExists(name string) (bool, error)   { return false, nil }
func (s *stubGit) DeleteTag(name string) (string, error)  { return "", nil }
func (s *stubGit) DeleteTagRemote(name string) (string, error) { return "", nil }
func (s *stubGit) PushTag(name string) (string, error) { return "", nil }
func (s *stubGit) PushTags() (string, error) { return "", nil }
func (s *stubGit) IsGHAuthenticated() (bool, error)      { return false, nil }
func (s *stubGit) CreateRelease(tagName, changelog string) (string, error) { return "", nil }
func (s *stubGit) CreateBackup(operation string, stashUntracked bool) (domain.Backup, error) {
	return domain.Backup{}, nil
}
func (s *stubGit) RestoreBackup(backup domain.Backup) error { return nil }
func (s *stubGit) DeleteBackup(backup domain.Backup) error { return nil }
func (s *stubGit) Add(paths []string) error {
	s.addCalls = append(s.addCalls, paths)
	return nil
}
func (s *stubGit) Remove(paths []string) error           { return nil }
func (s *stubGit) Checkout(name string) (string, error) { return "", nil }
func (s *stubGit) Switch(name string) error              { return nil }
func (s *stubGit) Push() (string, error)                { return s.pushResult, s.pushErr }
func (s *stubGit) Pull() (string, error)                { return "", nil }
func (s *stubGit) Fetch() (string, error)                { return "", nil }
func (s *stubGit) Stash() (string, error)               { return "", nil }
func (s *stubGit) StashPop() (string, error)             { return "", nil }
func (s *stubGit) Commit(message string) (string, error) {
	s.commitCalls = append(s.commitCalls, message)
	return "abc1234", nil
}
func (s *stubGit) Branch(name string) (string, error)   { return "", nil }
func (s *stubGit) RenameBranch(oldName, newName string) (string, error) { return "", nil }
func (s *stubGit) DeleteBranch(name string) (string, error) { return "", nil }
func (s *stubGit) Reset(mode string, commit string) (string, error) {
	s.resetCalls = append(s.resetCalls, mode+":"+commit)
	return "", nil
}
func (s *stubGit) Merge(branch string) (string, error) { return "", nil }
func (s *stubGit) Tag(name, message string) (string, error) { return "", nil }

type stubLLM struct {
	chunkMsg    string
	commitIntent domain.CommitIntent
}

func (l *stubLLM) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	if l.chunkMsg != "" {
		return l.chunkMsg, nil
	}
	return "feat: generated commit message", nil
}
func (l *stubLLM) DecideCommit(instruction, status, untracked, modified, deleted string) (domain.CommitIntent, error) {
	return l.commitIntent, nil
}
func (l *stubLLM) InterpretGitOp(op, instruction string, ctx map[string]string) (map[string]string, error) {
	return map[string]string{}, nil
}
func (l *stubLLM) SetRetryContext(msg string)  {}
func (l *stubLLM) ClearRetryContext()          {}
func (l *stubLLM) IsAvailable() bool           { return true }
func (l *stubLLM) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	return false, nil
}
func (l *stubLLM) AuditBinaryContent(filename, content string) (bool, error) {
	return false, nil
}
func (l *stubLLM) GenerateChangelog(commits, prev, out string) (string, error) {
	return "## Changelog", nil
}
func (l *stubLLM) RegenerateMessage(previousMessages []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	if len(previousMessages) != len(chunks) {
		return nil, fmt.Errorf("mock: count mismatch")
	}
	newMessages := make([]string, len(previousMessages))
	for i, msg := range previousMessages {
		newMessages[i] = msg + " (regenerated)"
	}
	return newMessages, nil
}

type stubDiffChunker struct {
	chunks []domain.DiffChunk
}

func (c *stubDiffChunker) Chunk(diff string, maxSize int) ([]domain.DiffChunk, error) {
	if len(c.chunks) > 0 {
		return c.chunks, nil
	}
	if diff == "" {
		return nil, nil
	}
	return []domain.DiffChunk{{Files: []string{"main.go"}, Diff: diff}}, nil
}

type stubSecurity struct{ blocked bool }

func (s *stubSecurity) CheckFiles(files []string, diff string) *ports.SecurityCheckResult {
	if s.blocked {
		return &ports.SecurityCheckResult{Blocked: true, Files: []ports.SecurityResult{
			{Halted: true, Message: "[SECURITY] blocked", File: files[0]},
		}}
	}
	return &ports.SecurityCheckResult{}
}
func (s *stubSecurity) ShouldUseLLMScan() bool { return false }

func newCommitSvcWithPath(git *stubGit, llm *stubLLM, security *stubSecurity, logPath string) *CommitService {
	chunker := &stubDiffChunker{}
	cfg := DefaultCommitServiceConfig(4096, 50, logPath)
	return NewCommitService(git, llm, chunker, security, cfg)
}

// --- Tests ---

func TestCommitService_PrepareCommit_NoCommit(t *testing.T) {
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: "main.go", Status: "M ", Staged: true},
			},
		},
		diffStagedResult: "diff --git a/main.go b/main.go\n+added",
	}
	llm := &stubLLM{}
	security := &stubSecurity{}

	svc := newCommitSvcWithPath(git, llm, security, t.TempDir()+"/c.log")
	_, _, _, _, _, err := svc.PrepareCommit("add feature")
	if err != nil {
		t.Fatalf("PrepareCommit() error: %v", err)
	}
	if len(git.commitCalls) > 0 {
		t.Error("PrepareCommit() must NOT call git.Commit — commits happen in APPLY")
	}
}

func TestCommitService_PrepareCommit_ReturnsMessages(t *testing.T) {
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: "auth.go", Status: "M ", Staged: true},
			},
		},
		diffStagedResult: "diff --git a/auth.go b/auth.go\n+new code",
	}
	llm := &stubLLM{chunkMsg: "feat(auth): add JWT support"}
	security := &stubSecurity{}

	svc := newCommitSvcWithPath(git, llm, security, t.TempDir()+"/c.log")
	messages, chunks, deleted, _, _, err := svc.PrepareCommit("add JWT")
	if err != nil {
		t.Fatalf("PrepareCommit() error: %v", err)
	}
	if len(messages) == 0 {
		t.Error("PrepareCommit() returned no messages")
	}
	if len(chunks) == 0 {
		t.Error("PrepareCommit() returned no chunks")
	}
	_ = deleted
}

func TestCommitService_PrepareCommit_SecurityBlocked(t *testing.T) {
	git := &stubGit{
		statusResult: domain.Status{
			Files: []domain.FileStatus{
				{Path: ".env", Status: "??", IsNew: true},
			},
		},
		untrackedResult:  []string{".env"},
		diffStagedResult: "diff with secret",
	}
	llm := &stubLLM{commitIntent: domain.CommitIntent{IncludeUntracked: true}}
	security := &stubSecurity{blocked: true}

	svc := newCommitSvcWithPath(git, llm, security, t.TempDir()+"/c.log")
	_, _, _, _, _, err := svc.PrepareCommit("commit all")
	if err == nil {
		t.Error("PrepareCommit() should error when security blocks commit")
	}
	if !strings.Contains(err.Error(), "SECURITY") {
		t.Errorf("error %q should mention SECURITY", err.Error())
	}
}

func TestCommitService_PrepareCommit_NothingToCommit(t *testing.T) {
	git := &stubGit{
		statusResult:     domain.Status{Files: []domain.FileStatus{}},
		diffStagedResult: "",
	}
	llm := &stubLLM{}
	security := &stubSecurity{}

	svc := newCommitSvcWithPath(git, llm, security, t.TempDir()+"/c.log")
	_, _, _, _, _, err := svc.PrepareCommit("commit")
	if err == nil {
		t.Error("PrepareCommit() should error when nothing to commit")
	}
}

func TestCommitService_ExecuteFromPlan_CommitsMessages(t *testing.T) {
	git := &stubGit{}
	llm := &stubLLM{}
	security := &stubSecurity{}

	svc := newCommitSvcWithPath(git, llm, security, t.TempDir()+"/c.log")
	messages := []string{"feat: add feature", "fix: typo"}
	chunkFiles := [][]string{{"main.go"}, {"auth.go"}}

	result, err := svc.ExecuteFromPlan(messages, chunkFiles, nil, "")
	if err != nil {
		t.Fatalf("ExecuteFromPlan() error: %v", err)
	}
	if result == "" {
		t.Error("ExecuteFromPlan() returned empty result")
	}
	if len(git.commitCalls) != 2 {
		t.Errorf("git.Commit() called %d times, want 2", len(git.commitCalls))
	}
}

func TestCommitService_ExecuteFromPlan_SkipsEmptyMessages(t *testing.T) {
	git := &stubGit{}
	llm := &stubLLM{}
	security := &stubSecurity{}

	svc := newCommitSvcWithPath(git, llm, security, t.TempDir()+"/c.log")
	messages := []string{"feat: real commit", "", "chore: no meaningful changes"}
	chunkFiles := [][]string{{"main.go"}, {"ignored.go"}, {"chore.go"}}

	_, err := svc.ExecuteFromPlan(messages, chunkFiles, nil, "")
	if err != nil {
		t.Fatalf("ExecuteFromPlan() error: %v", err)
	}
	if len(git.commitCalls) != 1 {
		t.Errorf("git.Commit() called %d times, want 1 (skip empty/no-op messages)", len(git.commitCalls))
	}
}

func TestCommitService_ExecuteFromPlan_WithDeletedFiles(t *testing.T) {
	git := &stubGit{}
	llm := &stubLLM{}
	security := &stubSecurity{}

	svc := newCommitSvcWithPath(git, llm, security, t.TempDir()+"/c.log")
	messages := []string{"feat: add feature"}
	chunkFiles := [][]string{{"main.go"}}
	deleted := []string{"old_file.go", "deprecated.go"}

	_, err := svc.ExecuteFromPlan(messages, chunkFiles, deleted, "")
	if err != nil {
		t.Fatalf("ExecuteFromPlan() error: %v", err)
	}
	// Should have 2 commits: 1 for the message + 1 for deleted files
	if len(git.commitCalls) < 2 {
		t.Errorf("git.Commit() called %d times, want >= 2 (feat + deleted)", len(git.commitCalls))
	}
}

func TestCommitService_ExecuteFromPlan_NoCommitsGenerated(t *testing.T) {
	git := &stubGit{}
	llm := &stubLLM{}
	security := &stubSecurity{}

	svc := newCommitSvcWithPath(git, llm, security, t.TempDir()+"/c.log")
	// All messages are skippable
	messages := []string{"", "chore: no meaningful changes"}
	chunkFiles := [][]string{nil, nil}

	_, err := svc.ExecuteFromPlan(messages, chunkFiles, nil, "")
	if err == nil {
		t.Error("ExecuteFromPlan() should error when no commits generated")
	}
}

func TestCommitService_DefaultConfig(t *testing.T) {
	cfg := DefaultCommitServiceConfig(4096, 100, "/tmp/log")
	if cfg.ChunkSize <= 0 {
		t.Error("ChunkSize should be positive")
	}
	if cfg.MaxLogLines != 100 {
		t.Errorf("MaxLogLines = %d, want 100", cfg.MaxLogLines)
	}
	if cfg.LogPath != "/tmp/log" {
		t.Errorf("LogPath = %q, want /tmp/log", cfg.LogPath)
	}
}

func TestCommitService_DefaultConfig_ZeroContextWindow(t *testing.T) {
	cfg := DefaultCommitServiceConfig(0, 50, "/tmp/log")
	// Should use default context window
	if cfg.ChunkSize <= 0 {
		t.Error("ChunkSize should be positive even with 0 context window input")
	}
}

// --- formatCommitStatus ---

func TestFormatCommitStatus(t *testing.T) {
	status := domain.Status{
		Files: []domain.FileStatus{
			{Path: "main.go", Status: "M "},
			{Path: "new.go", Status: "A "},
			{Path: "old.go", Status: "D "},
		},
	}
	result := formatCommitStatus(status)
	if result == "" {
		t.Error("formatCommitStatus() returned empty string")
	}
	if !strings.Contains(result, "main.go") {
		t.Error("formatCommitStatus() should contain file names")
	}
}

// --- getFilesToCommit ---

func TestGetFilesToCommit_IncludesTracked(t *testing.T) {
	status := domain.Status{
		Files: []domain.FileStatus{
			{Path: "main.go", Status: "M "},
			{Path: "auth.go", Status: "A "},
		},
	}
	decision := domain.CommitIntent{IncludeUntracked: false}
	files := getFilesToCommit(status, decision)
	if len(files) != 2 {
		t.Errorf("getFilesToCommit() = %d files, want 2", len(files))
	}
}

func TestGetFilesToCommit_IncludesUntracked_WhenFlagSet(t *testing.T) {
	status := domain.Status{
		Files: []domain.FileStatus{
			{Path: "main.go", Status: "M "},
			{Path: "new.go", Status: "??", IsNew: true},
		},
	}
	decision := domain.CommitIntent{IncludeUntracked: true}
	files := getFilesToCommit(status, decision)
	if len(files) != 2 {
		t.Errorf("getFilesToCommit() with untracked = %d files, want 2", len(files))
	}
}

func TestGetFilesToCommit_ExcludesUntracked_WhenFlagNotSet(t *testing.T) {
	status := domain.Status{
		Files: []domain.FileStatus{
			{Path: "main.go", Status: "M "},
			{Path: "new.go", Status: "??", IsNew: true},
		},
	}
	decision := domain.CommitIntent{IncludeUntracked: false}
	files := getFilesToCommit(status, decision)
	if len(files) != 1 {
		t.Errorf("getFilesToCommit() without untracked = %d files, want 1", len(files))
	}
}

func TestGetFilesToCommit_NoDuplicates(t *testing.T) {
	status := domain.Status{
		Files: []domain.FileStatus{
			{Path: "main.go", Status: "M "},
			{Path: "main.go", Status: "M "}, // duplicate
		},
	}
	decision := domain.CommitIntent{}
	files := getFilesToCommit(status, decision)
	seen := make(map[string]int)
	for _, f := range files {
		seen[f]++
	}
	for f, count := range seen {
		if count > 1 {
			t.Errorf("file %q appears %d times, want 1", f, count)
		}
	}
}

func TestDiffChunksToChunkFiles_Empty(t *testing.T) {
	result := DiffChunksToChunkFiles([]domain.DiffChunk{})
	if result != nil {
		t.Errorf("DiffChunksToChunkFiles([]) = %v, want nil", result)
	}
}

func TestDiffChunksToChunkFiles_SingleChunk(t *testing.T) {
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go", "b.go"}},
	}
	result := DiffChunksToChunkFiles(chunks)
	if len(result) != 1 {
		t.Fatalf("DiffChunksToChunkFiles returned %d chunks, want 1", len(result))
	}
	if len(result[0]) != 2 || result[0][0] != "a.go" || result[0][1] != "b.go" {
		t.Errorf("DiffChunksToChunkFiles[0] = %v, want [a.go b.go]", result[0])
	}
}

func TestDiffChunksToChunkFiles_MultipleChunks(t *testing.T) {
	chunks := []domain.DiffChunk{
		{Files: []string{"a.go"}},
		{Files: []string{"b.go", "c.go"}},
		{Files: []string{}},
	}
	result := DiffChunksToChunkFiles(chunks)
	if len(result) != 3 {
		t.Fatalf("DiffChunksToChunkFiles returned %d chunks, want 3", len(result))
	}
	if len(result[0]) != 1 || result[0][0] != "a.go" {
		t.Errorf("result[0] = %v, want [a.go]", result[0])
	}
	if len(result[1]) != 2 || result[1][0] != "b.go" || result[1][1] != "c.go" {
		t.Errorf("result[1] = %v, want [b.go c.go]", result[1])
	}
	if len(result[2]) != 0 {
		t.Errorf("result[2] = %v, want empty slice", result[2])
	}
}
