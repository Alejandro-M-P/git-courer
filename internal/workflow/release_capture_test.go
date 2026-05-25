package workflow

import (
	"strings"
	"sync"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// releaseStoreMock is a test double for CommitStore used in release tests.
type releaseStoreMock struct {
	mu       sync.Mutex
	entries  []domain.CommitEntry
	readErr  error
	clearErr error
	clearCalls int
}

func (m *releaseStoreMock) Append(entries ...domain.CommitEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append(m.entries, entries...)
	return nil
}

func (m *releaseStoreMock) Read() ([]domain.CommitEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readErr != nil {
		return nil, m.readErr
	}
	result := make([]domain.CommitEntry, len(m.entries))
	copy(result, m.entries)
	return result, nil
}

func (m *releaseStoreMock) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clearCalls++
	if m.clearErr != nil {
		return m.clearErr
	}
	m.entries = nil
	return nil
}

func (m *releaseStoreMock) SetBranch(name string) error {
	return nil
}

func (m *releaseStoreMock) RemoveBranch(name string) error {
	return nil
}

// --- Phase 4.1 RED: Prepare with CommitStore ---

func TestReleaseService_Prepare_UsesCommitStoreEntries(t *testing.T) {
	git := &mockGitForRelease{
		listTagsResult: []string{"v1.0.0"},
		commitsResult:  "", // should NOT be used when store has data
	}
	llm := &mockLLMForRelease{}
	store := &releaseStoreMock{}

	// Store has entries prepared by the commit cycle
	entry1, _ := domain.NewCommitEntry(
		"a1b2c3d4e5f6071829a0b1c2d3e4f50617283940",
		"feat: first commit",
	)
	entry2, _ := domain.NewCommitEntry(
		"b2c3d4e5f6071829a0b1c2d3e4f5061728394001",
		"fix: resolve bug",
	)
	store.entries = []domain.CommitEntry{entry1, entry2}

	svc := newReleaseSvcWithStore(t, git, llm, store)

	intent, commits, _, err := svc.Prepare("release minor version", "")
	if err != nil {
		t.Fatalf("Prepare() error: %v", err)
	}

	if !intent.IsRelease {
		t.Error("expected IsRelease = true")
	}

	// commits should contain the store entry messages, not from git.CommitsFromTag
	if !strings.Contains(commits, "feat: first commit") {
		t.Errorf("expected commits to contain 'feat: first commit', got: %s", commits)
	}
	if !strings.Contains(commits, "fix: resolve bug") {
		t.Errorf("expected commits to contain 'fix: resolve bug', got: %s", commits)
	}
}

func TestReleaseService_Prepare_FallsBackToGitWhenStoreEmpty(t *testing.T) {
	git := &mockGitForRelease{
		listTagsResult: []string{"v1.0.0"},
		commitsResult:  "feat: git commit output",
	}
	llm := &mockLLMForRelease{}
	store := &releaseStoreMock{} // empty store

	svc := newReleaseSvcWithStore(t, git, llm, store)

	_, commits, _, err := svc.Prepare("release minor version", "")
	if err != nil {
		t.Fatalf("Prepare() error: %v", err)
	}

	if !strings.Contains(commits, "feat: git commit output") {
		t.Errorf("expected commits to contain git fallback output, got: %s", commits)
	}
}

func TestReleaseService_Prepare_FallsBackWhenStoreErrors(t *testing.T) {
	git := &mockGitForRelease{
		listTagsResult: []string{"v1.0.0"},
		commitsResult:  "feat: fallback after store error",
	}
	llm := &mockLLMForRelease{}
	store := &releaseStoreMock{readErr: assertError("store unavailable")}
	store.entries = []domain.CommitEntry{
		{},
	}

	svc := newReleaseSvcWithStore(t, git, llm, store)

	_, commits, _, err := svc.Prepare("release minor version", "")
	if err != nil {
		t.Fatalf("Prepare() error: %v", err)
	}

	if !strings.Contains(commits, "feat: fallback after store error") {
		t.Errorf("expected commits to contain git fallback output, got: %s", commits)
	}
}

func TestReleaseService_Prepare_NilStoreIsNoOp(t *testing.T) {
	git := &mockGitForRelease{
		listTagsResult: []string{"v1.0.0"},
		commitsResult:  "feat: nil store fallback",
	}
	llm := &mockLLMForRelease{}

	svc := newReleaseSvcWithStore(t, git, llm, nil)

	_, commits, _, err := svc.Prepare("release minor version", "")
	if err != nil {
		t.Fatalf("Prepare() error: %v", err)
	}

	if !strings.Contains(commits, "feat: nil store fallback") {
		t.Errorf("expected commits to contain git fallback output, got: %s", commits)
	}
}

// --- Phase 4.3 RED: Execute with CommitStore Clear ---

func TestReleaseService_Execute_ClearsStoreAfterPush(t *testing.T) {
	git := &mockGitForRelease{} // default: tag succeeds, push succeeds
	llm := &mockLLMForRelease{}
	store := &releaseStoreMock{
		entries: []domain.CommitEntry{
			mustEntry(t, "a1b2c3d4e5f6071829a0b1c2d3e4f50617283940", "feat: test"),
		},
	}

	svc := newReleaseSvcWithStore(t, git, llm, store)

	intent := &domain.ReleaseIntent{
		TagName:   "v1.0.0",
		IsRelease: true,
	}

	_, err := svc.Execute(intent, "")
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	if store.clearCalls != 1 {
		t.Errorf("Clear() called %d times, want 1", store.clearCalls)
	}
}

func TestReleaseService_Execute_ClearFailureDoesNotFailRelease(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	store := &releaseStoreMock{
		entries: []domain.CommitEntry{
			mustEntry(t, "a1b2c3d4e5f6071829a0b1c2d3e4f50617283940", "feat: test"),
		},
		clearErr: assertError("clear failed"),
	}

	svc := newReleaseSvcWithStore(t, git, llm, store)

	intent := &domain.ReleaseIntent{
		TagName:   "v1.0.0",
		IsRelease: true,
	}

	_, err := svc.Execute(intent, "")
	if err != nil {
		t.Fatalf("Execute() should succeed even if Clear fails, got: %v", err)
	}

	if store.clearCalls != 1 {
		t.Errorf("Clear() should have been called once, got %d calls", store.clearCalls)
	}
}

func TestReleaseService_Execute_DoesNotClearOnTagFailure(t *testing.T) {
	git := &mockGitForRelease{
		tagExistsResult: true, // tag already exists → Execute fails
	}
	llm := &mockLLMForRelease{}
	store := &releaseStoreMock{
		entries: []domain.CommitEntry{
			mustEntry(t, "a1b2c3d4e5f6071829a0b1c2d3e4f50617283940", "feat: test"),
		},
	}

	svc := newReleaseSvcWithStore(t, git, llm, store)

	intent := &domain.ReleaseIntent{
		TagName:   "v1.0.0",
		IsRelease: true,
	}

	_, err := svc.Execute(intent, "")
	if err == nil {
		t.Fatal("Execute() should error when tag already exists")
	}

	if store.clearCalls != 0 {
		t.Errorf("Clear() should NOT be called on failure, got %d calls", store.clearCalls)
	}
}

func TestReleaseService_Execute_NilStoreNoPanic(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}

	svc := newReleaseSvcWithStore(t, git, llm, nil)

	intent := &domain.ReleaseIntent{
		TagName:   "v1.0.0",
		IsRelease: true,
	}

	_, err := svc.Execute(intent, "")
	if err != nil {
		t.Fatalf("Execute() with nil store should not error, got: %v", err)
	}
}

// --- Helpers ---

func newReleaseSvcWithStore(t *testing.T, git *mockGitForRelease, llm *mockLLMForRelease, store ports.CommitStore) *ReleaseService {
	t.Helper()
	dir := t.TempDir()
	cfg := DefaultReleaseServiceConfig(
		4096, 20, 100,
		dir+"/release.log",
	)
	chunker := &mockLogChunker{}
	return NewReleaseService(git, llm, chunker, cfg, nil, store)
}

func mustEntry(t *testing.T, sha, msg string) domain.CommitEntry {
	t.Helper()
	e, err := domain.NewCommitEntry(sha, msg)
	if err != nil {
		t.Fatalf("NewCommitEntry(%q, %q): %v", sha, msg, err)
	}
	return e
}
