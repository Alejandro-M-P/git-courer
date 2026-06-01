package workflow

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// releaseStoreMock is a test double for CommitStore used in release tests.
type releaseStoreMock struct {
	mu                      sync.Mutex
	entries                 []domain.CommitEntry
	readErr                 error
	clearErr                error
	clearCalls              int
	removeAllBranchDirErr   error
	removeAllBranchDirCalls int
	readAllBranchesResult   map[string][]domain.CommitEntry
	readAllBranchesErr      error
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

func (m *releaseStoreMock) Reconcile(gitEntries []domain.CommitEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = gitEntries
	return nil
}

func (m *releaseStoreMock) ReadAllBranches() (map[string][]domain.CommitEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.readAllBranchesErr != nil {
		return nil, m.readAllBranchesErr
	}
	if m.readAllBranchesResult != nil {
		return m.readAllBranchesResult, nil
	}
	// Default: return entries grouped under "main"
	result := make(map[string][]domain.CommitEntry)
	if len(m.entries) > 0 {
		result["main"] = m.entries
	}
	return result, nil
}

func (m *releaseStoreMock) RemoveAllBranchDirs() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeAllBranchDirCalls++
	return m.removeAllBranchDirErr
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
	store := &releaseStoreMock{readErr: assertError("store unavailable"), readAllBranchesErr: assertError("scan unavailable")}
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

// --- T3.3: Tag-based reconstruction (AC-6.1) ---
// When the branch store is empty, Prepare falls back to git.CommitsFromTag.

func TestReleaseService_Prepare_EmptyStoreFallsBackToGitHistory(t *testing.T) {
	// Simulate a new branch with no captured commits in the store.
	// The release service should fall back to git.CommitsFromTag.
	git := &mockGitForRelease{
		listTagsResult: []string{"v1.0.0"},
		commitsResult:  "feat: new feature from git history\nfix: bug from git history",
	}
	llm := &mockLLMForRelease{}
	store := &releaseStoreMock{
		entries: []domain.CommitEntry{}, // empty store — simulates new branch or post-release clear
	}

	svc := newReleaseSvcWithStore(t, git, llm, store)

	intent, commits, _, err := svc.Prepare("release minor version", "")
	if err != nil {
		t.Fatalf("Prepare() error: %v", err)
	}

	if !intent.IsRelease {
		t.Fatal("Prepare() should detect a releaseable change from git history")
	}
	if commits == "" {
		t.Error("Prepare() should return commits from git.CommitsFromTag fallback when store is empty")
	}
	if !strings.Contains(commits, "new feature from git history") {
		t.Errorf("Prepare() fallback commits should contain git history data, got: %s", commits)
	}
}

func TestReleaseService_Prepare_StoreWithDataSkipsGitFallback(t *testing.T) {
	// When the store HAS entries, Prepare should use them instead of git history.
	git := &mockGitForRelease{
		listTagsResult: []string{"v1.0.0"},
		commitsResult:  "SHOULD NOT BE USED — git fallback data",
	}
	llm := &mockLLMForRelease{}
	store := &releaseStoreMock{}

	entry1, _ := domain.NewCommitEntry(
		"a1b2c3d4e5f6071829a0b1c2d3e4f50617283940",
		"feat: from branch store",
	)
	store.entries = []domain.CommitEntry{entry1}

	svc := newReleaseSvcWithStore(t, git, llm, store)

	intent, commits, _, err := svc.Prepare("release minor version", "")
	if err != nil {
		t.Fatalf("Prepare() error: %v", err)
	}

	if !intent.IsRelease {
		t.Fatal("Prepare() should detect a releaseable change from store entries")
	}
	if strings.Contains(commits, "SHOULD NOT BE USED") {
		t.Error("Prepare() should NOT use git fallback when store has data")
	}
	if !strings.Contains(commits, "from branch store") {
		t.Errorf("Prepare() should use store data, got: %s", commits)
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

// --- Phase 2: ReadAllBranches aggregation and RemoveAllBranchDirs cleanup ---

func TestReleaseService_Prepare_AggregatesAllBranchCommits(t *testing.T) {
	git := &mockGitForRelease{
		listTagsResult: []string{"v1.0.0"},
		commitsResult:  "SHOULD NOT BE USED — git fallback data",
	}
	llm := &mockLLMForRelease{}

	entry1, _ := domain.NewCommitEntry(
		"a1b2c3d4e5f6071829a0b1c2d3e4f50617283940",
		"feat: shared commit",
	)
	entry2, _ := domain.NewCommitEntry(
		"b2c3d4e5f6071829a0b1c2d3e4f5061728394001",
		"fix: feature-a only",
	)
	entry3, _ := domain.NewCommitEntry(
		"c3d4e5f6071829a0b1c2d3e4f506172839400002",
		"feat: feature-b only",
	)

	// Simulate multi-branch data: entry1 appears in both branches (same SHA)
	store := &releaseStoreMock{
		readAllBranchesResult: map[string][]domain.CommitEntry{
			"feature-a": {entry1, entry2},
			"feature-b": {entry1, entry3},
		},
	}

	svc := newReleaseSvcWithStore(t, git, llm, store)
	intent, commits, _, err := svc.Prepare("release minor version", "")
	if err != nil {
		t.Fatalf("Prepare() error: %v", err)
	}

	if !intent.IsRelease {
		t.Fatal("Prepare() should detect a releaseable change")
	}

	// Should contain messages from all branches, deduplicated by SHA
	if !strings.Contains(commits, "shared commit") {
		t.Error("Prepare() should contain 'shared commit' from ReadAllBranches aggregation")
	}
	if !strings.Contains(commits, "feature-a only") {
		t.Error("Prepare() should contain 'feature-a only'")
	}
	if !strings.Contains(commits, "feature-b only") {
		t.Error("Prepare() should contain 'feature-b only'")
	}
	// Should NOT fall back to git
	if strings.Contains(commits, "SHOULD NOT BE USED") {
		t.Error("Prepare() should NOT use git fallback when ReadAllBranches succeeds")
	}
}

func TestReleaseService_Prepare_FallsBackOnReadAllBranchesError(t *testing.T) {
	git := &mockGitForRelease{
		listTagsResult: []string{"v1.0.0"},
		commitsResult:  "feat: fallback from git",
	}
	llm := &mockLLMForRelease{}

	store := &releaseStoreMock{
		readAllBranchesErr: fmt.Errorf("scan failed"),
		entries: []domain.CommitEntry{
			mustEntry(t, "a1b2c3d4e5f6071829a0b1c2d3e4f50617283940", "feat: from store fallback"),
		},
	}

	svc := newReleaseSvcWithStore(t, git, llm, store)
	_, commits, _, err := svc.Prepare("release minor version", "")
	if err != nil {
		t.Fatalf("Prepare() error: %v", err)
	}

	// Should fall back to Read() (single branch) when ReadAllBranches fails
	if !strings.Contains(commits, "from store fallback") {
		t.Errorf("Prepare() should use Read() fallback when ReadAllBranches fails, got: %s", commits)
	}
	// Should NOT fall back to git when Read() has data
	if strings.Contains(commits, "fallback from git") {
		t.Error("Prepare() should NOT use git fallback when Read() has data")
	}
}

func TestReleaseService_Execute_CleansUpBranchDirs(t *testing.T) {
	git := &mockGitForRelease{}
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

	if store.removeAllBranchDirCalls != 1 {
		t.Errorf("RemoveAllBranchDirs() called %d times, want 1", store.removeAllBranchDirCalls)
	}
}

func TestReleaseService_Execute_CleanupFailureDoesNotFailRelease(t *testing.T) {
	git := &mockGitForRelease{}
	llm := &mockLLMForRelease{}
	store := &releaseStoreMock{
		entries: []domain.CommitEntry{
			mustEntry(t, "a1b2c3d4e5f6071829a0b1c2d3e4f50617283940", "feat: test"),
		},
		removeAllBranchDirErr: assertError("permission denied"),
	}

	svc := newReleaseSvcWithStore(t, git, llm, store)

	intent := &domain.ReleaseIntent{
		TagName:   "v1.0.0",
		IsRelease: true,
	}

	_, err := svc.Execute(intent, "")
	if err != nil {
		t.Fatalf("Execute() should succeed even if RemoveAllBranchDirs fails, got: %v", err)
	}

	if store.removeAllBranchDirCalls != 1 {
		t.Errorf("RemoveAllBranchDirs() should have been called once, got %d calls", store.removeAllBranchDirCalls)
	}
}
