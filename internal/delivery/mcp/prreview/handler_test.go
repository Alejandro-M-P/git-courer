package prreview

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/blak0p/git-courer/internal/config"
	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/infra/chunkers"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockGit is a testify-based mock implementing ports.Git.
type mockGit struct {
	mock.Mock
}

func (m *mockGit) Status() (domain.Status, error) {
	args := m.Called()
	return args.Get(0).(domain.Status), args.Error(1)
}
func (m *mockGit) MergeBase(a, b string) (string, error) {
	args := m.Called(a, b)
	return args.String(0), args.Error(1)
}
func (m *mockGit) LogRange(from, to string) (string, error) { return "", nil }

func (m *mockGit) DiffRange(base, target, mode string, paths ...string) (string, error) {
	args := m.Called(base, target, mode)
	return args.String(0), args.Error(1)
}
func (m *mockGit) DiffStat(paths ...string) (string, error)          { panic("not implemented") }
func (m *mockGit) DiffStatStaged(paths ...string) (string, error)    { panic("not implemented") }
func (m *mockGit) CurrentBranch() (string, error)                    { panic("not implemented") }
func (m *mockGit) Add(paths []string) error                          { panic("not implemented") }
func (m *mockGit) Amend(msg string, paths []string) (string, error)  { panic("not implemented") }
func (m *mockGit) Blame(filepath string) ([]domain.BlameLine, error) { panic("not implemented") }
func (m *mockGit) Branch(name string) (string, error)                { panic("not implemented") }
func (m *mockGit) CatFile(revision, path string) (string, error)     { panic("not implemented") }
func (m *mockGit) CherryPick(commit string) (string, error)          { panic("not implemented") }
func (m *mockGit) Clean() error                                      { panic("not implemented") }
func (m *mockGit) Commit(message string) (string, error)             { panic("not implemented") }
func (m *mockGit) CommitsFromTag(sinceTag string) (string, error)    { panic("not implemented") }
func (m *mockGit) CreateBackup(op string, mode domain.StashMode) (domain.Backup, error) {
	panic("not implemented")
}
func (m *mockGit) CreateRelease(tagName, changelog string) (string, error) { panic("not implemented") }
func (m *mockGit) DeleteBackup(backup domain.Backup) error                 { panic("not implemented") }
func (m *mockGit) DeleteBranch(name string, force bool) (string, error)    { panic("not implemented") }
func (m *mockGit) DeleteRemoteBranch(name string) error                    { panic("not implemented") }
func (m *mockGit) DeleteTag(name string) (string, error)                   { panic("not implemented") }
func (m *mockGit) DeleteTagRemote(name string) (string, error)             { panic("not implemented") }
func (m *mockGit) Diff(paths ...string) (string, error)                    { panic("not implemented") }
func (m *mockGit) DiffAll(paths ...string) (string, error)                 { panic("not implemented") }
func (m *mockGit) DiffUntracked() (string, error)                         { panic("not implemented") }
func (m *mockGit) DiffStaged(paths ...string) (string, error)              { panic("not implemented") }
func (m *mockGit) Fetch() (string, error)                                  { panic("not implemented") }
func (m *mockGit) IsGHAuthenticated() (bool, error)                        { panic("not implemented") }
func (m *mockGit) IsRepo() bool                                            { panic("not implemented") }
func (m *mockGit) LatestTag() (string, error)                              { panic("not implemented") }
func (m *mockGit) ListBackups() ([]domain.Backup, error)                   { panic("not implemented") }
func (m *mockGit) ListBranches(pattern ...string) (string, error)          { panic("not implemented") }
func (m *mockGit) ListTags(pattern ...string) ([]string, error)            { panic("not implemented") }
func (m *mockGit) ListTree(revision, path string, recursive bool) ([]string, error) {
	panic("not implemented")
}
func (m *mockGit) ListUntracked() ([]string, error) { panic("not implemented") }
func (m *mockGit) Log(limit int, pattern string, paths ...string) (string, error) {
	panic("not implemented")
}
func (m *mockGit) LogFull(limit int) (string, error)            { panic("not implemented") }
func (m *mockGit) Merge(branch string) (string, error)          { panic("not implemented") }
func (m *mockGit) MergeAbort() (string, error)                  { panic("not implemented") }
func (m *mockGit) MergeContinue() (string, error)               { panic("not implemented") }
func (m *mockGit) MergeSkip() (string, error)                   { panic("not implemented") }
func (m *mockGit) PruneBackups(olderThan time.Duration) error   { panic("not implemented") }
func (m *mockGit) Pull() (string, error)                        { panic("not implemented") }
func (m *mockGit) PullFrom(remoteBranch string) (string, error) { panic("not implemented") }
func (m *mockGit) Push() (string, error)                        { panic("not implemented") }
func (m *mockGit) PushTag(name string) (string, error)          { panic("not implemented") }
func (m *mockGit) PushTo(remoteBranch string) (string, error)   { panic("not implemented") }
func (m *mockGit) Rebase(branch string) (string, error)         { panic("not implemented") }
func (m *mockGit) RebaseAbort() (string, error)                 { panic("not implemented") }
func (m *mockGit) RebaseContinue() (string, error)              { panic("not implemented") }
func (m *mockGit) RebaseSkip() (string, error)                  { panic("not implemented") }
func (m *mockGit) RebaseOnto(newBase, upstream, branch string) (string, error) {
	panic("not implemented")
}
func (m *mockGit) PushToBranch(remote, branch string) (string, error)   { panic("not implemented") }
func (m *mockGit) PullFromBranch(remote, branch string) (string, error) { panic("not implemented") }
func (m *mockGit) Reflog() ([]domain.ReflogEntry, error)                { panic("not implemented") }
func (m *mockGit) RemoteAdd(name, url string) (string, error)           { panic("not implemented") }
func (m *mockGit) RemoteInfo() (string, error)                          { panic("not implemented") }
func (m *mockGit) RemoteRemove(name string) (string, error)             { panic("not implemented") }
func (m *mockGit) RemoteURL() (string, error)                           { panic("not implemented") }
func (m *mockGit) Remove(paths []string) error                          { panic("not implemented") }
func (m *mockGit) RenameBranch(oldName, newName string) (string, error) { panic("not implemented") }
func (m *mockGit) Reset(mode, commit string) (string, error)            { panic("not implemented") }
func (m *mockGit) ResetSoft(ref string) error                           { panic("not implemented") }
func (m *mockGit) Restore(paths []string) error                         { panic("not implemented") }
func (m *mockGit) RestoreBackup(backup domain.Backup) error             { panic("not implemented") }
func (m *mockGit) Revert(commit string) (string, error)                 { panic("not implemented") }
func (m *mockGit) Search(pattern string, context, before, after int, paths ...string) (string, error) {
	panic("not implemented")
}
func (m *mockGit) SetUpstream(branch, remote string) (string, error) { panic("not implemented") }
func (m *mockGit) Show(hash string) (domain.ShowResult, error)       { panic("not implemented") }
func (m *mockGit) ShowCommit(commit string) (string, error)          { panic("not implemented") }
func (m *mockGit) Stash(message ...string) (string, error)           { panic("not implemented") }
func (m *mockGit) StashApply(index string) (string, error)           { panic("not implemented") }
func (m *mockGit) StashClear() (string, error)                       { panic("not implemented") }
func (m *mockGit) StashDiff(index string) (string, error)            { panic("not implemented") }
func (m *mockGit) StashDrop(index string) (string, error)            { panic("not implemented") }
func (m *mockGit) StashList() ([]domain.StashEntry, error)           { panic("not implemented") }
func (m *mockGit) StashPop() (string, error)                         { panic("not implemented") }
func (m *mockGit) StashShow() (string, error)                        { panic("not implemented") }
func (m *mockGit) StashWithUntracked(message string) (string, error) { panic("not implemented") }
func (m *mockGit) Switch(branch string) error                        { panic("not implemented") }
func (m *mockGit) Tag(name, message string) (string, error)          { panic("not implemented") }
func (m *mockGit) TagFromFile(name, path string) (string, error)     { panic("not implemented") }
func (m *mockGit) TagExists(name string) (bool, error)               { panic("not implemented") }
func (m *mockGit) UnsetUpstream(branch string) (string, error)       { panic("not implemented") }
func (m *mockGit) ConfigGet(key string) (string, error) {
	args := m.Called(key)
	return args.String(0), args.Error(1)
}
func (m *mockGit) ConfigSet(key, value string) (string, error) {
	args := m.Called(key, value)
	return args.String(0), args.Error(1)
}
func (m *mockGit) SymbolicRef(ref string) (string, error) {
	args := m.Called(ref)
	return args.String(0), args.Error(1)
}
func (m *mockGit) WriteTree() (string, error) { panic("not implemented") }
func (m *mockGit) CommitTree(treeHash, parentHash, message string) (string, error) {
	panic("not implemented")
}
func (m *mockGit) UpdateRef(ref, commitHash string) (string, error) { panic("not implemented") }
func (m *mockGit) Head() (string, error)                            { panic("not implemented") }
func (m *mockGit) HashObject(data []byte) (string, error)          { return "mock-blob-sha", nil }
func (m *mockGit) ShowRef(pattern string) (string, error)         { return "", nil }

func newTestHandler(git *mockGit, workDir string, testRunner func(ctx context.Context, command string) TestResult) *Handler {
	chunker := chunkers.NewDiffChunker(
		chunkers.WithMaxFilesPerChunk(12),
		chunkers.WithMinForce(3),
	)
	h := NewHandler(git, workDir, chunker, "")
	if testRunner != nil {
		h.testRunner = testRunner
	}
	return h
}

func TestPRReview_NoTestCommand(t *testing.T) {
	git := new(mockGit)
	git.On("Status").Return(domain.Status{
		Branch:      "feat/branch",
		Ahead:       3,
		Behind:      1,
		HasUpstream: true,
		Conflicted:  0,
	}, nil)
	git.On("MergeBase", "feat/branch", "main").Return("abc123", nil)
	git.On("DiffRange", "abc123", "feat/branch", "..").Return("diff --git a/file.go b/file.go\n+added line\n-removed line\n", nil)

	tmpDir := t.TempDir()
	h := newTestHandler(git, tmpDir, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "pr_review",
			Arguments: map[string]any{},
		},
	}

	res, err := h.HandlePRReview(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result PRReviewResult
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.NoError(t, json.Unmarshal([]byte(text), &result))
	assert.Equal(t, "no_test_command", result.Status)
	assert.Equal(t, "feat/branch", result.Branch.Name)
	assert.Contains(t, result.Hint, "SET_TEST_COMMAND")
	git.AssertExpectations(t)
}

func TestPRReview_TestCommandFromProjectConfig(t *testing.T) {
	git := new(mockGit)
	git.On("Status").Return(domain.Status{
		Branch:      "feat/branch",
		Ahead:       2,
		Behind:      0,
		HasUpstream: true,
		Conflicted:  0,
	}, nil)
	git.On("MergeBase", "feat/branch", "main").Return("abc123", nil)
	git.On("DiffRange", "abc123", "feat/branch", "..").Return("", nil)

	tmpDir := t.TempDir()
	cfg := &config.ProjectConfig{
		Description: "test project",
		TestCommand: "go test ./...",
	}
	require.NoError(t, config.SaveProjectConfig(tmpDir, cfg))

	mockTestRunner := func(ctx context.Context, command string) TestResult {
		assert.Equal(t, "go test ./...", command, "test runner should receive command from project config")
		return TestResult{Status: "pass", Total: 10}
	}
	h := newTestHandler(git, tmpDir, mockTestRunner)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "pr_review",
			Arguments: map[string]any{},
		},
	}

	res, err := h.HandlePRReview(context.Background(), req)
	assert.NoError(t, err)

	var result PRReviewResult
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.NoError(t, json.Unmarshal([]byte(text), &result))
	assert.Equal(t, "test_ok", result.Status)
	assert.Equal(t, "pass", result.TestResult.Status)
	git.AssertExpectations(t)
}

func TestPRReview_TestFail(t *testing.T) {
	git := new(mockGit)
	git.On("Status").Return(domain.Status{
		Branch:      "feat/branch",
		Ahead:       2,
		Behind:      0,
		HasUpstream: true,
		Conflicted:  0,
	}, nil)
	git.On("MergeBase", "feat/branch", "main").Return("abc123", nil)
	git.On("DiffRange", "abc123", "feat/branch", "..").Return("", nil)

	tmpDir := t.TempDir()
	cfg := &config.ProjectConfig{
		Description: "test project",
		TestCommand: "go test ./...",
	}
	require.NoError(t, config.SaveProjectConfig(tmpDir, cfg))

	mockTestRunner := func(ctx context.Context, command string) TestResult {
		return TestResult{
			Status:       "fail",
			Total:        5,
			Failed:       2,
			FailingTests: []FailingTest{{Package: "pkg/foo", TestName: "TestBar", Output: "expected true, got false"}},
		}
	}
	h := newTestHandler(git, tmpDir, mockTestRunner)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "pr_review",
			Arguments: map[string]any{},
		},
	}

	res, err := h.HandlePRReview(context.Background(), req)
	assert.NoError(t, err)

	var result PRReviewResult
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.NoError(t, json.Unmarshal([]byte(text), &result))
	assert.Equal(t, "test_fail", result.Status)
	assert.Equal(t, 2, result.TestResult.Failed)
	assert.Len(t, result.TestResult.FailingTests, 1)
	assert.Contains(t, result.Hint, "test(s) failed")
	git.AssertExpectations(t)
}

func TestPRReview_Conflict(t *testing.T) {
	git := new(mockGit)
	git.On("Status").Return(domain.Status{
		Branch:     "feat/branch",
		Ahead:      1,
		Behind:     0,
		Conflicted: 2,
		Files: []domain.FileStatus{
			{Path: "file1.go", Status: "U"},
			{Path: "file2.go", Status: "U"},
			{Path: "file3.go", Status: "M"},
		},
	}, nil)
	git.On("MergeBase", "feat/branch", "main").Return("abc123", nil)
	git.On("DiffRange", "abc123", "feat/branch", "..").Return("diff --git a/file1.go b/file1.go\n+added\n-removed\n", nil)

	tmpDir := t.TempDir()
	cfg := &config.ProjectConfig{
		Description: "test project",
		TestCommand: "go test ./...",
	}
	require.NoError(t, config.SaveProjectConfig(tmpDir, cfg))

	h := newTestHandler(git, tmpDir, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "pr_review",
			Arguments: map[string]any{},
		},
	}

	res, err := h.HandlePRReview(context.Background(), req)
	assert.NoError(t, err)

	var result PRReviewResult
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.NoError(t, json.Unmarshal([]byte(text), &result))
	assert.Equal(t, "conflict", result.Status)
	assert.Contains(t, result.Conflict.Files, "file1.go")
	assert.Contains(t, result.Conflict.Files, "file2.go")
	assert.NotContains(t, result.Conflict.Files, "file3.go")
	git.AssertExpectations(t)
}

func TestPRReview_Error(t *testing.T) {
	git := new(mockGit)
	git.On("Status").Return(domain.Status{}, fmt.Errorf("not a git repository"))

	tmpDir := t.TempDir()
	h := newTestHandler(git, tmpDir, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "pr_review",
			Arguments: map[string]any{},
		},
	}

	res, err := h.HandlePRReview(context.Background(), req)
	assert.NoError(t, err)

	var result PRReviewResult
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.NoError(t, json.Unmarshal([]byte(text), &result))
	assert.Equal(t, "error", result.Status)
	assert.Contains(t, result.Hint, "failed to get repository status")
	git.AssertExpectations(t)
}

func TestPRReview_DefaultTargetBranch(t *testing.T) {
	git := new(mockGit)
	git.On("Status").Return(domain.Status{
		Branch:      "feat/branch",
		Ahead:       1,
		Behind:      0,
		HasUpstream: true,
		Conflicted:  0,
	}, nil)
	git.On("MergeBase", "feat/branch", "main").Return("abc123", nil)
	git.On("DiffRange", "abc123", "feat/branch", "..").Return("", nil)

	tmpDir := t.TempDir()
	h := newTestHandler(git, tmpDir, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "pr_review",
			Arguments: map[string]any{},
		},
	}

	res, err := h.HandlePRReview(context.Background(), req)
	assert.NoError(t, err)

	var result PRReviewResult
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.NoError(t, json.Unmarshal([]byte(text), &result))
	assert.Equal(t, "no_test_command", result.Status)
	git.AssertExpectations(t)
}

func TestPRReview_DiffStats(t *testing.T) {
	git := new(mockGit)
	git.On("Status").Return(domain.Status{
		Branch:      "feat/branch",
		Ahead:       3,
		Behind:      0,
		HasUpstream: true,
		Conflicted:  0,
	}, nil)
	git.On("MergeBase", "feat/branch", "main").Return("abc123", nil)
	git.On("DiffRange", "abc123", "feat/branch", "..").Return(
		"diff --git a/main.go b/main.go\n+added5lines\n-removed3lines\n\ndiff --git a/helper.go b/helper.go\n+added2lines\n", nil)

	tmpDir := t.TempDir()
	cfg := &config.ProjectConfig{
		Description: "test project",
		TestCommand: "go test ./...",
	}
	require.NoError(t, config.SaveProjectConfig(tmpDir, cfg))

	mockTestRunner := func(ctx context.Context, command string) TestResult {
		return TestResult{Status: "pass", Total: 5}
	}
	h := newTestHandler(git, tmpDir, mockTestRunner)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "pr_review",
			Arguments: map[string]any{},
		},
	}

	res, err := h.HandlePRReview(context.Background(), req)
	assert.NoError(t, err)

	var result PRReviewResult
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.NoError(t, json.Unmarshal([]byte(text), &result))
	assert.Equal(t, "test_ok", result.Status)
	assert.Len(t, result.DiffStats.Files, 2)
	assert.Equal(t, "main.go", result.DiffStats.Files[0].Path)
	assert.GreaterOrEqual(t, result.DiffStats.Files[0].Additions, 1)
	assert.GreaterOrEqual(t, result.DiffStats.Files[0].Deletions, 1)
	assert.Equal(t, "helper.go", result.DiffStats.Files[1].Path)
	assert.GreaterOrEqual(t, result.DiffStats.Files[1].Additions, 1)
	git.AssertExpectations(t)
}

func TestParseDiffFromOutput(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantFiles int
		wantAdd   int
		wantDel   int
	}{
		{
			name:      "empty input",
			input:     "",
			wantFiles: 0,
			wantAdd:   0,
			wantDel:   0,
		},
		{
			name: "single file",
			input: `diff --git a/main.go b/main.go
+added line
+another addition
-removed line
`,
			wantFiles: 1,
			wantAdd:   2,
			wantDel:   1,
		},
		{
			name: "multiple files",
			input: `diff --git a/main.go b/main.go
+added
-removed
diff --git a/helper.go b/helper.go
+added2
+added3
`,
			wantFiles: 2,
			wantAdd:   3,
			wantDel:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDiffFromOutput(tt.input)
			assert.Len(t, result.Files, tt.wantFiles)
			assert.Equal(t, tt.wantAdd, result.TotalAdditions)
			assert.Equal(t, tt.wantDel, result.TotalDeletions)
		})
	}
}

func TestPRReview_ProjectConfigLoadError(t *testing.T) {
	git := new(mockGit)
	git.On("Status").Return(domain.Status{
		Branch:      "feat/branch",
		Ahead:       1,
		Behind:      0,
		HasUpstream: true,
		Conflicted:  0,
	}, nil)
	git.On("MergeBase", "feat/branch", "main").Return("abc123", nil)
	git.On("DiffRange", "abc123", "feat/branch", "..").Return("", nil)

	// Path that doesn't exist
	h := newTestHandler(git, "/nonexistent/path", nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "pr_review",
			Arguments: map[string]any{},
		},
	}

	res, err := h.HandlePRReview(context.Background(), req)
	assert.NoError(t, err)

	var result PRReviewResult
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.NoError(t, json.Unmarshal([]byte(text), &result))
	assert.Equal(t, "no_test_command", result.Status)
	assert.Contains(t, result.Hint, "SET_TEST_COMMAND")
}
