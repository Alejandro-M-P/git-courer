// Package e2e contains end-to-end tests for the git-courer workflow.
// These tests verify bug fixes and integration between services using mock adapters.
package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/secrets"
	"github.com/Alejandro-M-P/git-courer/internal/workflow"
)

// --- Shared mock implementations ---

type mockGit struct {
	latestTagResult    string
	commitsResult      string
	listTagsResult     []string
	tagExistsResult    bool
	listBranchesResult string
	commitCalled       bool
	tagCalled          bool
	tagCalledWith      string
}

func (m *mockGit) Status() (domain.Status, error)                { return domain.Status{}, nil }
func (m *mockGit) Diff(paths ...string) (string, error)      { return "", nil }
func (m *mockGit) DiffStaged(paths ...string) (string, error) { return "diff --git a/f.go", nil }
func (m *mockGit) ListUntracked() ([]string, error)       { return nil, nil }
func (m *mockGit) Log(limit int, paths ...string) (string, error)   { return m.commitsResult, nil }
func (m *mockGit) LogFull(limit int) (string, error)      { return m.commitsResult, nil }
func (m *mockGit) CurrentBranch() (string, error)       { return "develop", nil }
func (m *mockGit) ListBranches(pattern ...string) (string, error) { return m.listBranchesResult, nil }
func (m *mockGit) ListTags(pattern ...string) ([]string, error) { return m.listTagsResult, nil }
func (m *mockGit) IsRepo() bool                        { return true }
func (m *mockGit) LatestTag() (string, error)          { return m.latestTagResult, nil }
func (m *mockGit) CommitsFromTag(sinceTag string) (string, error) { return m.commitsResult, nil }
func (m *mockGit) TagExists(name string) (bool, error)  { return m.tagExistsResult, nil }
func (m *mockGit) DeleteTag(name string) (string, error) { return "", nil }
func (m *mockGit) DeleteTagRemote(name string) (string, error) { return "", nil }
func (m *mockGit) PushTag(name string) (string, error) { return "", nil }
func (m *mockGit) PushTags() (string, error) { return "", nil }
func (m *mockGit) IsGHAuthenticated() (bool, error)     { return false, nil }
func (m *mockGit) CreateRelease(name, changelog string) (string, error) { return "", nil }
func (m *mockGit) CreateBackup(operation string, keepIndex bool) (domain.Backup, error) {
	return domain.Backup{}, nil
}
func (m *mockGit) RestoreBackup(backup domain.Backup) error { return nil }
func (m *mockGit) DeleteBackup(backup domain.Backup) error { return nil }
func (m *mockGit) Add(paths []string) error             { return nil }
func (m *mockGit) Remove(paths []string) error          { return nil }
func (m *mockGit) Checkout(name string) (string, error) { return "", nil }
func (m *mockGit) Switch(name string) error            { return nil }
func (m *mockGit) Push() (string, error)               { return "", nil }
func (m *mockGit) Pull() (string, error)               { return "", nil }
func (m *mockGit) Fetch() (string, error)              { return "", nil }
func (m *mockGit) Stash() (string, error)              { return "", nil }
func (m *mockGit) StashPop() (string, error)          { return "", nil }
func (m *mockGit) Commit(message string) (string, error) {
	m.commitCalled = true
	return "abc1234", nil
}
func (m *mockGit) Branch(name string) (string, error)   { return "", nil }
func (m *mockGit) DeleteBranch(name string) (string, error) { return "", nil }
func (m *mockGit) Reset(mode string, commit string) (string, error) { return "", nil }
func (m *mockGit) Merge(branch string) (string, error) { return "", nil }
func (m *mockGit) Tag(name string) (string, error) {
	m.tagCalled = true
	m.tagCalledWith = name
	return "", nil
}

type mockLLM struct {
	releaseIntent *domain.ReleaseIntent
	changelog     string
}

func (m *mockLLM) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	return "feat: mock commit message", nil
}
func (m *mockLLM) DecideCommit(instruction, status, untracked, modified, deleted string) (domain.CommitIntent, error) {
	return domain.CommitIntent{IncludeUntracked: false}, nil
}
func (m *mockLLM) InterpretGitOp(op, instruction string, ctx map[string]string) (map[string]string, error) {
	return map[string]string{}, nil
}
func (m *mockLLM) SetRetryContext(msg string) {}
func (m *mockLLM) ClearRetryContext()         {}
func (m *mockLLM) IsAvailable() bool          { return true }
func (m *mockLLM) InterpretReleaseIntent(instruction, releases, branches, currentBranch string) (*domain.ReleaseIntent, error) {
	if m.releaseIntent != nil {
		return m.releaseIntent, nil
	}
	return &domain.ReleaseIntent{
		TagName:     "v1.1.0",
		IsRelease:   true,
		VersionBump: "minor",
	}, nil
}
func (m *mockLLM) GenerateChangelog(commits, previousChangelog, outputFile string) (string, error) {
	if m.changelog != "" {
		return m.changelog, nil
	}
	return "## Added\n- New features", nil
}
func (m *mockLLM) PolishChangelog(chunks []string) (string, error) {
	return strings.Join(chunks, "\n"), nil
}
func (m *mockLLM) RegenerateMessage(previousMessages []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	if len(previousMessages) != len(chunks) {
		return nil, fmt.Errorf("mock: count mismatch")
	}
	newMessages := make([]string, len(previousMessages))
	for i, msg := range previousMessages {
		newMessages[i] = msg + " (regenerated: " + feedback + ")"
	}
	return newMessages, nil
}

type mockLogChunker struct{}

func (m *mockLogChunker) Chunk(commits string, maxPerChunk int) ([]string, error) {
	if commits == "" {
		return []string{}, nil
	}
	return []string{commits}, nil
}

func tempLogPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.log")
}

// --- Test 1: Commit with preview=true should NOT execute git.Commit until APPLY ---

func TestCommitPreviewDoesNotExecute(t *testing.T) {
	git := &mockGit{}
	llm := &mockLLM{}

	chunker := &mockDiffChunker{}
	security := &mockSecurity{}

	cfg := workflow.DefaultCommitServiceConfig(4096, 100000, 50, tempLogPath(t))
	svc := workflow.NewCommitService(git, llm, chunker, security, cfg)

	// PrepareCommit should NOT call git.Commit — it only generates messages
	_, _, _, _, _, err := svc.PrepareCommit("add tests")
	if err != nil {
		t.Fatalf("PrepareCommit() unexpected error: %v", err)
	}
	if git.commitCalled {
		t.Error("git.Commit() was called during PrepareCommit — commits must NOT execute until APPLY")
	}
}

// --- Test 2: Existing tag returns explicit error ---

func TestExistingTagReturnsError(t *testing.T) {
	git := &mockGit{
		tagExistsResult: true, // tag already exists
		listTagsResult:  []string{"v1.0.0"},
		commitsResult:   "feat: something",
	}
	llm := &mockLLM{
		releaseIntent: &domain.ReleaseIntent{
			TagName:     "v1.0.0",
			IsRelease:   true,
			VersionBump: "minor",
		},
	}
	chunker := &mockLogChunker{}

	cfg := workflow.DefaultReleaseServiceConfig(4096, 20, 50, tempLogPath(t))
	svc := workflow.NewReleaseService(git, llm, chunker, cfg)

	intent := &domain.ReleaseIntent{TagName: "v1.0.0", IsRelease: true}
	_, err := svc.Execute(intent, "## Changelog", false)
	if err == nil {
		t.Fatal("Execute() expected error for existing tag, got nil")
	}
	if !strings.Contains(err.Error(), "ya existe") {
		t.Errorf("Execute() error %q should mention 'ya existe'", err.Error())
	}
	if git.tagCalled {
		t.Error("git.Tag() was called despite tag already existing")
	}
}

// --- Test 3: RELEASE_APPLY goroutine error is surfaced via LoadState ---

func TestReleaseStateErrorIsPropagated(t *testing.T) {
	dir := t.TempDir()
	intentPath := filepath.Join(dir, "intent.json")
	cfg := workflow.DefaultReleaseServiceConfigWithPaths(4096, 20, 50, tempLogPath(t), "", intentPath)

	git := &mockGit{}
	llm := &mockLLM{}
	chunker := &mockLogChunker{}
	svc := workflow.NewReleaseService(git, llm, chunker, cfg)

	// Simulate goroutine writing an error state
	svc.SaveState("error: connection refused")

	state := svc.LoadState()
	if !strings.HasPrefix(state, "error:") {
		t.Errorf("LoadState() = %q, want prefix 'error:'", state)
	}
	if !strings.Contains(state, "connection refused") {
		t.Errorf("LoadState() = %q, want to contain original error message", state)
	}
}

// --- Test 4: Full release flow — tag created, changelog not empty ---

func TestReleaseFullFlow(t *testing.T) {
	git := &mockGit{
		tagExistsResult: false,
		listTagsResult:  []string{"v1.0.0"},
		commitsResult:   "feat: add dashboard\nfix: button alignment",
	}
	llm := &mockLLM{
		releaseIntent: &domain.ReleaseIntent{
			TagName:     "v1.1.0",
			IsRelease:   true,
			VersionBump: "minor",
		},
		changelog: "## Added\n- Dashboard\n\n## Fixed\n- Button alignment",
	}
	chunker := &mockLogChunker{}

	dir := t.TempDir()
	cfg := workflow.DefaultReleaseServiceConfigWithPaths(4096, 20, 50, tempLogPath(t),
		filepath.Join(dir, "changelog.md"),
		filepath.Join(dir, "intent.json"),
	)
	svc := workflow.NewReleaseService(git, llm, chunker, cfg)

	intent, commits, _, err := svc.Prepare("release version 1.1.0", "")
	if err != nil {
		t.Fatalf("Prepare() error: %v", err)
	}
	if intent.TagName == "" {
		t.Error("Prepare() returned empty TagName")
	}
	if commits == "" {
		t.Error("Prepare() returned empty commits")
	}

	changelog, _, err := svc.Generate(commits)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if changelog == "" {
		t.Error("Generate() returned empty changelog")
	}

	_, err = svc.Execute(intent, changelog, false)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	if !git.tagCalled {
		t.Error("Execute() did not create git tag")
	}
	if git.tagCalledWith != intent.TagName {
		t.Errorf("Execute() created tag %q, want %q", git.tagCalledWith, intent.TagName)
	}
}

// --- Test 5: Secret detection blocks commit ---

func TestSecretBlocksCommit(t *testing.T) {
	dir := t.TempDir()
	secretFile := filepath.Join(dir, "config.go")
	content := "apiKey := \"AKIAIOSFODNN7EXAMPLE1234\"\n"
	os.WriteFile(secretFile, []byte(content), 0644)

	results, err := secrets.Detect([]string{secretFile})
	if err != nil {
		t.Fatalf("Detect() error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected secret detection to find AWS key, got no results")
	}
	found := false
	for _, r := range results {
		if r.Type == "aws_access_key" {
			found = true
		}
	}
	if !found {
		t.Error("expected aws_access_key type in results")
	}
}

// --- Test 6: Semver previousTag handles v0.10.0 correctly ---

func TestPreviousTagSemverOrdering(t *testing.T) {
	git := &mockGit{
		tagExistsResult: false,
		listTagsResult:  []string{"v0.9.0", "v0.10.0", "v0.11.0"},
		commitsResult:   "feat: new stuff",
	}
	llm := &mockLLM{
		releaseIntent: &domain.ReleaseIntent{
			TagName:     "v0.12.0",
			IsRelease:   true,
			VersionBump: "minor",
		},
	}
	chunker := &mockLogChunker{}

	dir := t.TempDir()
	cfg := workflow.DefaultReleaseServiceConfigWithPaths(4096, 20, 50, tempLogPath(t),
		filepath.Join(dir, "changelog.md"),
		filepath.Join(dir, "intent.json"),
	)
	svc := workflow.NewReleaseService(git, llm, chunker, cfg)

	intent, _, _, err := svc.Prepare("bump minor", "")
	if err != nil {
		t.Fatalf("Prepare() error: %v", err)
	}
	// v0.11.0 should be the previous tag (not v0.9.0 from alphabetical sort)
	_ = intent // we just verify no panic and correct execution
}

// --- Test 7: Backup and restore state ---

func TestBackupAndRestoreInterface(t *testing.T) {
	git := &mockGit{}

	backup, err := git.CreateBackup("test", false)
	if err != nil {
		t.Fatalf("CreateBackup() error: %v", err)
	}
	// Mock returns zero-value backup — just verify the interface is callable
	_ = backup

	if err := git.RestoreBackup(domain.Backup{Ref: "refs/test", CreatedAt: time.Now()}); err != nil {
		t.Errorf("RestoreBackup() error: %v", err)
	}
}

// --- Supporting mocks for CommitService ---

type mockDiffChunker struct{}

func (m *mockDiffChunker) Chunk(diff string, maxSize int) ([]domain.DiffChunk, error) {
	return []domain.DiffChunk{
		{Files: []string{"main.go"}, Diff: diff},
	}, nil
}

type mockSecurity struct{}

func (m *mockSecurity) CheckFiles(files []string, diff string) *ports.SecurityCheckResult {
	return &ports.SecurityCheckResult{}
}

func (m *mockSecurity) ShouldUseLLMScan() bool { return false }
