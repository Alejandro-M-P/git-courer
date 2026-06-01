package workflow

import (
	"strings"
	"sync"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// mockCommitStore is a test double for CommitStore used in commit capture tests.
type mockCommitStore struct {
	mu        sync.Mutex
	appended  []domain.CommitEntry
	appendErr error
}

func (m *mockCommitStore) Append(entries ...domain.CommitEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.appendErr != nil {
		return m.appendErr
	}
	m.appended = append(m.appended, entries...)
	return nil
}

func (m *mockCommitStore) Read() ([]domain.CommitEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.appended, nil
}

func (m *mockCommitStore) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appended = nil
	return nil
}

func (m *mockCommitStore) SetBranch(name string) error {
	return nil
}

func (m *mockCommitStore) RemoveBranch(name string) error {
	return nil
}

func (m *mockCommitStore) Reconcile(gitEntries []domain.CommitEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.appended = gitEntries
	return nil
}

func (m *mockCommitStore) ReadAllBranches() (map[string][]domain.CommitEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string][]domain.CommitEntry)
	if len(m.appended) > 0 {
		result["main"] = m.appended
	}
	return result, nil
}

func (m *mockCommitStore) RemoveAllBranchDirs() error {
	return nil
}

// captureTestGit extends stubGit to return a proper SHA from Head() and author from ConfigGet.
type captureTestGit struct {
	stubGit
	headSHA  string
	headErr  error
	userName string
}

func (g *captureTestGit) Head() (string, error) {
	return g.headSHA, g.headErr
}

func (g *captureTestGit) ConfigGet(key string) (string, error) {
	if key == "user.name" && g.userName != "" {
		return g.userName, nil
	}
	return g.stubGit.ConfigGet(key)
}

// validSHA returns a 40-char hex string for testing.
// Use different suffixes for different commits.
func validSHA(suffix string) string {
	const base = "a1b2c3d4e5f6071829a0b1c2d3e4f50617283940"
	if suffix == "" {
		return base
	}
	if len(suffix) > 40 {
		suffix = suffix[:40]
	}
	runes := []rune(base)
	copy(runes[len(runes)-len(suffix):], []rune(suffix))
	return string(runes)
}

func TestCommitService_Capture_AppendsEntryAfterCommit(t *testing.T) {
	git := &captureTestGit{
		stubGit: stubGit{
			statusResult: domain.Status{
				Files: []domain.FileStatus{
					{Path: "main.go", Status: "M "},
				},
			},
			diffStagedResult: "diff --git a/main.go\n+line",
		},
		headSHA:  validSHA("00000001"),
		userName: "Test User",
	}
	llm := &stubLLM{chunkMsg: "feat: first commit"}
	security := &stubSecurity{}
	store := &mockCommitStore{}

	svc := newCommitSvcWithCapture(git, llm, security, t.TempDir()+"/c.log", store)

	_, err := svc.Execute("commit changes", false)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if len(store.appended) != 1 {
		t.Fatalf("expected 1 entry appended, got %d", len(store.appended))
	}

	entry := store.appended[0]
	if entry.SHA() != validSHA("00000001") {
		t.Errorf("entry SHA = %q, want %q", entry.SHA(), validSHA("00000001"))
	}
	if entry.Message() != "feat: first commit" {
		t.Errorf("entry Message = %q, want %q", entry.Message(), "feat: first commit")
	}
	if entry.Author() == "" {
		t.Error("expected non-empty Author")
	}
	if entry.Date() == "" {
		t.Error("expected non-empty Date")
	}
}

func TestCommitService_Capture_NilStoreDoesNotPanic(t *testing.T) {
	git := &captureTestGit{
		stubGit: stubGit{
			statusResult: domain.Status{
				Files: []domain.FileStatus{
					{Path: "main.go", Status: "M "},
				},
			},
			diffStagedResult: "diff --git a/main.go\n+line",
		},
		headSHA: validSHA("00000002"),
	}
	llm := &stubLLM{chunkMsg: "feat: nil store commit"}
	security := &stubSecurity{}

	svc := newCommitSvcWithCapture(git, llm, security, t.TempDir()+"/c.log", nil)

	_, err := svc.Execute("commit changes", false)
	if err != nil {
		t.Fatalf("Execute() with nil store should not error, got: %v", err)
	}
}

func TestCommitService_Capture_AppendFailureDoesNotFailCommit(t *testing.T) {
	git := &captureTestGit{
		stubGit: stubGit{
			statusResult: domain.Status{
				Files: []domain.FileStatus{
					{Path: "main.go", Status: "M "},
				},
			},
			diffStagedResult: "diff --git a/main.go\n+line",
		},
		headSHA: validSHA("00000003"),
	}
	llm := &stubLLM{chunkMsg: "feat: append will fail"}
	security := &stubSecurity{}
	store := &mockCommitStore{appendErr: assertError("storage full")}

	svc := newCommitSvcWithCapture(git, llm, security, t.TempDir()+"/c.log", store)

	_, err := svc.Execute("commit changes", false)
	if err != nil {
		t.Fatalf("Execute() should succeed even if Append fails, got: %v", err)
	}
}

func TestCommitService_Capture_MultipleCommits(t *testing.T) {
	git := &captureTestGit{
		stubGit: stubGit{
			statusResult: domain.Status{
				Files: []domain.FileStatus{
					{Path: "main.go", Status: "M "},
				},
			},
			diffStagedResult: "diff --git a/main.go\n+line",
		},
		headSHA: validSHA("00000004"),
	}
	llm := &indexedCaptureLLM{
		messages: []string{"feat: first", "feat: second"},
	}
	security := &stubSecurity{}
	store := &mockCommitStore{}

	// Use multiChunkChunker to produce multiple chunks
	chunker := &multiChunkChunker{
		chunks: []domain.DiffChunk{
			{Files: []string{"main.go"}, Diff: "diff --git a/main.go\n+line1"},
			{Files: []string{"foo.go"}, Diff: "diff --git a/foo.go\n+line2"},
		},
	}

	svc := newCommitSvcWithCaptureMultiChunk(git, llm, security, chunker, t.TempDir()+"/c.log", store)

	// Execute via ExecuteFromPlan so we can pass pre-generated messages
	chunks := []domain.DiffChunk{
		{Files: []string{"main.go"}, Diff: "diff --git a/main.go\n+line1"},
		{Files: []string{"foo.go"}, Diff: "diff --git a/foo.go\n+line2"},
	}
	msgs := []string{"feat: first", "feat: second"}
	chunkFiles := DiffChunksToChunkFiles(chunks)

	_, err := svc.ExecuteFromPlan(msgs, chunkFiles, nil, "commit")
	if err != nil {
		t.Fatalf("ExecuteFromPlan() error: %v", err)
	}

	if len(store.appended) != 2 {
		t.Fatalf("expected 2 entries appended, got %d", len(store.appended))
	}
	if store.appended[0].Message() != "feat: first" {
		t.Errorf("entry[0] Message = %q, want %q", store.appended[0].Message(), "feat: first")
	}
	if store.appended[1].Message() != "feat: second" {
		t.Errorf("entry[1] Message = %q, want %q", store.appended[1].Message(), "feat: second")
	}
}

func TestCommitService_Capture_FromPlanAppendsEntry(t *testing.T) {
	git := &captureTestGit{
		stubGit: stubGit{
			statusResult: domain.Status{
				Files: []domain.FileStatus{
					{Path: "main.go", Status: "M "},
				},
			},
			diffStagedResult: "diff --git a/main.go\n+line",
		},
		headSHA: validSHA("00000005"),
	}
	llm := &stubLLM{chunkMsg: "feat: from plan"}
	security := &stubSecurity{}
	store := &mockCommitStore{}

	svc := newCommitSvcWithCapture(git, llm, security, t.TempDir()+"/c.log", store)

	msgs := []string{"feat: from plan"}
	chunkFiles := [][]string{{"main.go"}}
	_, err := svc.ExecuteFromPlan(msgs, chunkFiles, nil, "commit")
	if err != nil {
		t.Fatalf("ExecuteFromPlan() error: %v", err)
	}

	if len(store.appended) != 1 {
		t.Fatalf("expected 1 entry appended, got %d", len(store.appended))
	}
	if store.appended[0].Message() != "feat: from plan" {
		t.Errorf("entry Message = %q, want %q", store.appended[0].Message(), "feat: from plan")
	}
}

// --- Helpers ---

// newCommitSvcWithCapture creates a CommitService with the new signature including CommitStore.
func newCommitSvcWithCapture(git ports.Git, llm ports.LLM, security *stubSecurity, logPath string, store ports.CommitStore) *CommitService {
	chunker := &stubDiffChunker{}
	cfg := DefaultCommitServiceConfig(4096, 50, logPath)
	cfg.ContentProvider = nil
	return NewCommitService(git, llm, chunker, security, cfg, store)
}

// newCommitSvcWithCaptureMultiChunk creates a CommitService with custom chunker and CommitStore.
func newCommitSvcWithCaptureMultiChunk(git ports.Git, llm ports.LLM, security *stubSecurity, chunker ports.DiffChunker, logPath string, store ports.CommitStore) *CommitService {
	cfg := DefaultCommitServiceConfig(4096, 50, logPath)
	cfg.ContentProvider = nil
	return NewCommitService(git, llm, chunker, security, cfg, store)
}

// indexedCaptureLLM returns pre-defined messages in sequence.
type indexedCaptureLLM struct {
	stubLLM
	messages []string
	index    int
}

func (l *indexedCaptureLLM) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	if l.index < len(l.messages) {
		msg := l.messages[l.index]
		l.index++
		return msg, nil
	}
	return "feat: fallback", nil
}

func (l *indexedCaptureLLM) GenerateCommitSynthesis(combinedChunk domain.DiffChunk, fileMessages []string) (string, error) {
	return strings.Join(fileMessages, "\n"), nil
}

// assertError is a simple error for testing.
type assertError string

func (e assertError) Error() string { return string(e) }
