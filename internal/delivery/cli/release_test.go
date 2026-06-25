package cli

import (
	"io"
	"strings"
	"testing"

	"github.com/blak0p/git-courer/internal/adapters/commitstore"
	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
)

// --- Mock ReleaseSvc for testing ---

type mockReleaseSvc struct {
	prepareResult       *domain.ReleaseIntent
	prepareCommits      string
	prepareWarnings     []string
	prepareErr          error
	prepareInstruction  string
	prepareUserBump     string
	generateResult      string
	generateErr         error
	executeResult       string
	executeErr          error
	loadIntentResult    *domain.ReleaseIntent
	loadIntentErr       error
	loadStateResult     string
	clearCalled         bool
	saveIntentCalled    bool
	saveChangelogCalled bool
	customMessage       string // tracks SetCustomMessage calls
}

func (m *mockReleaseSvc) Prepare(instruction, userBump string) (*domain.ReleaseIntent, string, []string, error) {
	m.prepareInstruction = instruction
	m.prepareUserBump = userBump
	return m.prepareResult, m.prepareCommits, m.prepareWarnings, m.prepareErr
}
func (m *mockReleaseSvc) Generate(commits string) (string, []string, bool, error) {
	return m.generateResult, nil, false, m.generateErr
}
func (m *mockReleaseSvc) Execute(intent *domain.ReleaseIntent, changelog string) (string, error) {
	return m.executeResult, m.executeErr
}
func (m *mockReleaseSvc) SaveIntent(intent *domain.ReleaseIntent) {
	m.saveIntentCalled = true
}
func (m *mockReleaseSvc) LoadIntent() (*domain.ReleaseIntent, error) {
	return m.loadIntentResult, m.loadIntentErr
}
func (m *mockReleaseSvc) SaveChangelog(changelog string) {
	m.saveChangelogCalled = true
}
func (m *mockReleaseSvc) LoadChangelog() (string, error) {
	return "changelog content", nil
}
func (m *mockReleaseSvc) ClearPending() {
	m.clearCalled = true
}
func (m *mockReleaseSvc) LoadState() string {
	return m.loadStateResult
}
func (m *mockReleaseSvc) BuildPreview(intent *domain.ReleaseIntent, changelog string) string {
	return "preview output"
}
func (m *mockReleaseSvc) SetCustomMessage(msg string) { m.customMessage = msg }

// Compile-time check
var _ ReleaseSvc = (*mockReleaseSvc)(nil)

// --- Mock CommitStore for testing ---

type mockCommitStoreForCLI struct {
	branchSet    string
	appended     []domain.CommitEntry
	clearCalls   int
	removeBranch string
}

func (m *mockCommitStoreForCLI) Append(entries ...domain.CommitEntry) error {
	m.appended = append(m.appended, entries...)
	return nil
}
func (m *mockCommitStoreForCLI) Read() ([]domain.CommitEntry, error) { return m.appended, nil }
func (m *mockCommitStoreForCLI) Clear() error {
	m.clearCalls++
	m.appended = nil
	return nil
}
func (m *mockCommitStoreForCLI) SetBranch(name string) error {
	m.branchSet = name
	return nil
}
func (m *mockCommitStoreForCLI) RemoveBranch(name string) error {
	m.removeBranch = name
	return nil
}
func (m *mockCommitStoreForCLI) Reconcile(gitEntries []domain.CommitEntry) error {
	m.appended = gitEntries
	return nil
}

func (m *mockCommitStoreForCLI) ReadAllBranches() (map[string][]domain.CommitEntry, error) {
	result := make(map[string][]domain.CommitEntry)
	if len(m.appended) > 0 {
		result["main"] = m.appended
	}
	return result, nil
}

func (m *mockCommitStoreForCLI) RemoveAllBranchDirs() error {
	return nil
}

// Compile-time check
var _ ports.CommitStore = (*mockCommitStoreForCLI)(nil)

// --- Branch scoping tests ---

func TestReleaseCommand_DetachedHEAD_UsesGlobalStore(t *testing.T) {
	// When branch is empty (detached HEAD),
	// SetBranch should NOT be called — the store uses the legacy global path.
	store := &mockCommitStoreForCLI{}
	cmd := NewReleaseCommand(nil, nil, nil, store, "/tmp")
	cmd.InitBranchScoping("") // empty branch = detached HEAD

	if store.branchSet != "" {
		t.Errorf("SetBranch should NOT be called on detached HEAD, but got branchSet = %q", store.branchSet)
	}
}

func TestReleaseCommand_BranchSet_ScopesStore(t *testing.T) {
	store := &mockCommitStoreForCLI{}
	cmd := NewReleaseCommand(nil, nil, nil, store, "/tmp")
	cmd.InitBranchScoping("feat/auth")

	if store.branchSet != "feat/auth" {
		t.Errorf("SetBranch should be called with 'feat/auth', got %q", store.branchSet)
	}
}

func TestReleaseCommand_InitBranchScoping_ErrorBranch(t *testing.T) {
	store := &mockCommitStoreForCLI{}
	cmd := NewReleaseCommand(nil, nil, nil, store, "/tmp")

	// Empty string = detached HEAD = no SetBranch call
	cmd.InitBranchScoping("")

	if store.branchSet != "" {
		t.Errorf("SetBranch should NOT be called on empty branch, got %q", store.branchSet)
	}
}

func TestReleaseCommand_InitBranchScoping_SpecialCharacterBranch(t *testing.T) {
	store := &mockCommitStoreForCLI{}
	cmd := NewReleaseCommand(nil, nil, nil, store, "/tmp")

	// Branch with slash — SetBranch is called with raw name.
	// The FilesystemCommitStore adapter sanitizes it internally.
	cmd.InitBranchScoping("feat/auth")

	if store.branchSet != "feat/auth" {
		t.Errorf("SetBranch should be called with 'feat/auth', got %q", store.branchSet)
	}
}

// --- Interactive flow tests ---

func TestReleaseCommand_Interactive_Apply(t *testing.T) {
	store := &mockCommitStoreForCLI{}
	cmd := NewReleaseCommand(nil, nil, nil, store, "/tmp")
	svc := &mockReleaseSvc{
		prepareResult: &domain.ReleaseIntent{
			TagName:     "v1.1.0",
			IsRelease:   true,
			VersionBump: "minor",
		},
		prepareCommits: "feat: new feature",
		generateResult: "## Features\n- new thing",
		executeResult:  `{"operation":"release","tag_name":"v1.1.0"}`,
	}
	cmd.SetReleaseService(svc)
	// Enter for tag, Enter for message, "s" for action
	cmd.Stdin = strings.NewReader("\n\ns\n")
	cmd.Stdout = io.Discard

	err := cmd.Run()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !svc.saveIntentCalled {
		t.Error("expected SaveIntent to be called")
	}
	if !svc.saveChangelogCalled {
		t.Error("expected SaveChangelog to be called")
	}
}

func TestReleaseCommand_Interactive_Abort(t *testing.T) {
	store := &mockCommitStoreForCLI{}
	cmd := NewReleaseCommand(nil, nil, nil, store, "/tmp")
	svc := &mockReleaseSvc{
		prepareResult: &domain.ReleaseIntent{
			TagName:     "v1.1.0",
			IsRelease:   true,
			VersionBump: "minor",
		},
		prepareCommits: "feat: new feature",
		generateResult: "## Features\n- new thing",
	}
	cmd.SetReleaseService(svc)
	// Enter for tag, Enter for message, "N" for action
	cmd.Stdin = strings.NewReader("\n\nN\n")
	cmd.Stdout = io.Discard

	err := cmd.Run()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !svc.clearCalled {
		t.Error("expected ClearPending to be called")
	}
}

func TestReleaseCommand_Interactive_Regenerate(t *testing.T) {
	store := &mockCommitStoreForCLI{}
	cmd := NewReleaseCommand(nil, nil, nil, store, "/tmp")
	svc := &mockReleaseSvc{
		prepareResult: &domain.ReleaseIntent{
			TagName:     "v1.1.0",
			IsRelease:   true,
			VersionBump: "minor",
		},
		prepareCommits: "feat: new feature",
		generateResult: "## Features\n- new thing",
		executeResult:  `{"operation":"release","tag_name":"v1.1.0"}`,
	}
	cmd.SetReleaseService(svc)
	// Enter for tag, Enter for message, "r" → feedback → loop back → Enter tag, Enter message, "s"
	cmd.Stdin = strings.NewReader("\n\nr\nmake it clearer\n\n\ns\n")
	cmd.Stdout = io.Discard

	err := cmd.Run()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if svc.customMessage != "make it clearer" {
		t.Errorf("SetCustomMessage = %q, want %q", svc.customMessage, "make it clearer")
	}
	if !svc.saveIntentCalled {
		t.Error("expected SaveIntent to be called after regenerate")
	}
}

func TestReleaseCommand_Interactive_NoCommits(t *testing.T) {
	store := &mockCommitStoreForCLI{}
	cmd := NewReleaseCommand(nil, nil, nil, store, "/tmp")
	svc := &mockReleaseSvc{
		prepareResult: &domain.ReleaseIntent{
			TagName:   "v1.1.0",
			IsRelease: false,
		},
		prepareCommits: "",
	}
	cmd.SetReleaseService(svc)
	cmd.Stdin = strings.NewReader("\n")
	cmd.Stdout = io.Discard

	err := cmd.Run()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestReleaseCommand_Interactive_CustomTag(t *testing.T) {
	store := &mockCommitStoreForCLI{}
	cmd := NewReleaseCommand(nil, nil, nil, store, "/tmp")
	svc := &mockReleaseSvc{
		prepareResult: &domain.ReleaseIntent{
			TagName:     "v1.1.0",
			IsRelease:   true,
			VersionBump: "minor",
		},
		prepareCommits: "feat: new feature",
		generateResult: "## Features\n- new thing",
		executeResult:  `{"operation":"release","tag_name":"v2.0.0"}`,
	}
	cmd.SetReleaseService(svc)
	// Type custom tag "v2.0.0", then "s" to apply
	cmd.Stdin = strings.NewReader("v2.0.0\n\ns\n")
	cmd.Stdout = io.Discard

	err := cmd.Run()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if svc.prepareInstruction != "v2.0.0" {
		t.Errorf("Prepare instruction = %q, want %q", svc.prepareInstruction, "v2.0.0")
	}
}

// --- Integration tests — release clears branch store ---

func TestReleaseCommand_Apply_ClearsBranchStore(t *testing.T) {
	// AC-2.5: After `gcourer release apply` on `feat/auth`,
	// `.git-courer/branches/feat-auth/commits.json` is empty.
	dir := t.TempDir()
	store := commitstore.NewFilesystemCommitStore(dir, nil)
	store.SetBranch("feat/auth")

	// Write some entries
	entry, _ := domain.NewCommitEntry("a1b2c3d4e5f6071829a0b1c2d3e4f50617283940", "feat: new feature")
	if err := store.Append(entry); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Verify entries are there
	entries, err := store.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry before clear, got %d", len(entries))
	}

	// Clear (simulates what ReleaseService.Execute does after successful push)
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify store is empty after clear
	entries, err = store.Read()
	if err != nil {
		t.Fatalf("Read after clear failed: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries after clear, got %d", len(entries))
	}
}

func TestReleaseCommand_ReleaseOnBranchDoesNotClearOtherBranch(t *testing.T) {
	// AC-4.4: Other branches' stores are NOT cleared by a release on a different branch
	dir := t.TempDir()

	// Create two branch stores
	featStore := commitstore.NewFilesystemCommitStore(dir, nil)
	featStore.SetBranch("feat/auth")

	mainStore := commitstore.NewFilesystemCommitStore(dir, nil)
	mainStore.SetBranch("main")

	// Write entries on feat/auth
	featEntry, _ := domain.NewCommitEntry("a1b2c3d4e5f6071829a0b1c2d3e4f50617283940", "feat: auth feature")
	if err := featStore.Append(featEntry); err != nil {
		t.Fatalf("Append to feat/auth failed: %v", err)
	}

	// Write entries on main
	mainEntry, _ := domain.NewCommitEntry("b2c3d4e5f6071829a0b1c2d3e4f5061728394001", "fix: bug on main")
	if err := mainStore.Append(mainEntry); err != nil {
		t.Fatalf("Append to main failed: %v", err)
	}

	// Simulate release on main — clear main's store
	if err := mainStore.Clear(); err != nil {
		t.Fatalf("Clear on main failed: %v", err)
	}

	// Verify main is empty
	mainEntries, err := mainStore.Read()
	if err != nil {
		t.Fatalf("Read main after clear failed: %v", err)
	}
	if len(mainEntries) != 0 {
		t.Fatalf("expected 0 entries on main after clear, got %d", len(mainEntries))
	}

	// Verify feat/auth is still intact
	featEntries, err := featStore.Read()
	if err != nil {
		t.Fatalf("Read feat/auth failed: %v", err)
	}
	if len(featEntries) != 1 {
		t.Fatalf("expected 1 entry on feat/auth (unaffected by main release), got %d", len(featEntries))
	}
	if featEntries[0].Message() != "feat: auth feature" {
		t.Errorf("feat/auth entry message = %q, want %q", featEntries[0].Message(), "feat: auth feature")
	}
}

func TestReleaseCommand_BranchStoreAfterSetBranchAndAppend(t *testing.T) {
	// AC-2.9: On `feat/auth` branch, release creates tag and clears
	// `.git-courer/branches/feat-auth/commits.json`
	// This test verifies the store path resolution.
	dir := t.TempDir()

	store := commitstore.NewFilesystemCommitStore(dir, nil)
	store.SetBranch("feat/auth")

	// Append and verify
	entry, _ := domain.NewCommitEntry("a1b2c3d4e5f6071829a0b1c2d3e4f50617283940", "feat: something")
	if err := store.Append(entry); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	readBack, err := store.Read()
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(readBack) != 1 || readBack[0].Message() != "feat: something" {
		t.Fatalf("expected 1 entry with message 'feat: something', got %d entries", len(readBack))
	}

	// Clear and verify empty
	if err := store.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	readBack, err = store.Read()
	if err != nil {
		t.Fatalf("Read after clear failed: %v", err)
	}
	if len(readBack) != 0 {
		t.Fatalf("expected empty store after clear, got %d entries", len(readBack))
	}
}
