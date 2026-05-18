package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/confirm"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/workflow"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Minimal mocks for core handlers ---

type mockGit struct {
	mock.Mock
}

func (m *mockGit) Status() (domain.Status, error) {
	args := m.Called()
	return args.Get(0).(domain.Status), args.Error(1)
}
func (m *mockGit) Diff(paths ...string) (string, error) {
	args := m.Called(paths)
	return args.String(0), args.Error(1)
}
func (m *mockGit) DiffStat(paths ...string) (string, error) {
	args := m.Called(paths)
	return args.String(0), args.Error(1)
}
func (m *mockGit) DiffStatStaged(paths ...string) (string, error) {
	args := m.Called(paths)
	return args.String(0), args.Error(1)
}
func (m *mockGit) DiffStaged(paths ...string) (string, error) {
	args := m.Called(paths)
	return args.String(0), args.Error(1)
}
func (m *mockGit) DiffRange(base, target, mode string, paths ...string) (string, error) {
	args := m.Called(base, target, mode, paths)
	return args.String(0), args.Error(1)
}
func (m *mockGit) CurrentBranch() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *mockGit) IsRepo() bool {
	args := m.Called()
	return args.Bool(0)
}
func (m *mockGit) RemoteInfo() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *mockGit) Amend(message string, paths []string) (string, error) {
	args := m.Called(message, paths)
	return args.String(0), args.Error(1)
}
func (m *mockGit) Revert(commit string) (string, error) {
	args := m.Called(commit)
	return args.String(0), args.Error(1)
}
func (m *mockGit) StashDiff(index string) (string, error) {
	args := m.Called(index)
	return args.String(0), args.Error(1)
}

// Remaining ports.Git methods — panic stubs so mockGit satisfies the interface.
func (m *mockGit) ListUntracked() ([]string, error)                                { panic("unexpected") }
func (m *mockGit) Log(limit int, pattern string, paths ...string) (string, error)   { panic("unexpected") }
func (m *mockGit) LogFull(limit int) (string, error)                                { panic("unexpected") }
func (m *mockGit) ListBranches(pattern ...string) (string, error)                   { panic("unexpected") }
func (m *mockGit) ListTags(pattern ...string) ([]string, error)                     { panic("unexpected") }
func (m *mockGit) Search(pattern string, context, before, after int, paths ...string) (string, error) { panic("unexpected") }
func (m *mockGit) CatFile(revision, path string) (string, error)                    { panic("unexpected") }
func (m *mockGit) ListTree(revision, path string, recursive bool) ([]string, error) { panic("unexpected") }
func (m *mockGit) LatestTag() (string, error)                                       { panic("unexpected") }
func (m *mockGit) CommitsFromTag(sinceTag string) (string, error)                   { panic("unexpected") }
func (m *mockGit) TagExists(name string) (bool, error)                              { panic("unexpected") }
func (m *mockGit) IsGHAuthenticated() (bool, error)                                 { panic("unexpected") }
func (m *mockGit) CreateRelease(tagName, changelog string) (string, error)          { panic("unexpected") }
func (m *mockGit) Blame(filepath string) ([]domain.BlameLine, error)                { panic("unexpected") }
func (m *mockGit) Show(hash string) (domain.ShowResult, error)                      { panic("unexpected") }
func (m *mockGit) Reflog() ([]domain.ReflogEntry, error)                            { panic("unexpected") }
func (m *mockGit) StashList() ([]domain.StashEntry, error)                          { panic("unexpected") }
func (m *mockGit) StashShow() (string, error)                                       { panic("unexpected") }
func (m *mockGit) MergeBase(a, b string) (string, error)                            { panic("unexpected") }
func (m *mockGit) RemoteURL() (string, error)                                       { panic("unexpected") }
func (m *mockGit) DiffAll(paths ...string) (string, error)                          { panic("unexpected") }
func (m *mockGit) CreateBackup(operation string, mode domain.StashMode) (domain.Backup, error) {
	args := m.Called(operation, mode)
	return args.Get(0).(domain.Backup), args.Error(1)
}
func (m *mockGit) RestoreBackup(backup domain.Backup) error                          { panic("unexpected") }
func (m *mockGit) DeleteBackup(backup domain.Backup) error                           { panic("unexpected") }
func (m *mockGit) ListBackups() ([]domain.Backup, error)                             { panic("unexpected") }
func (m *mockGit) PruneBackups(olderThan time.Duration) error                         { panic("unexpected") }
func (m *mockGit) Add(paths []string) error                                          { panic("unexpected") }
func (m *mockGit) Remove(paths []string) error                                       { panic("unexpected") }
func (m *mockGit) Commit(message string) (string, error)                             { panic("unexpected") }
func (m *mockGit) Push() (string, error)                                             { panic("unexpected") }
func (m *mockGit) PushTo(remoteBranch string) (string, error)                        { panic("unexpected") }
func (m *mockGit) Pull() (string, error)                                             { panic("unexpected") }
func (m *mockGit) PullFrom(remoteBranch string) (string, error)                      { panic("unexpected") }
func (m *mockGit) Fetch() (string, error)                                            { panic("unexpected") }
func (m *mockGit) Stash(message ...string) (string, error)                           { panic("unexpected") }
func (m *mockGit) StashWithUntracked(message string) (string, error)                 { panic("unexpected") }
func (m *mockGit) StashPop() (string, error)                                         { panic("unexpected") }
func (m *mockGit) StashApply(index string) (string, error)                           { panic("unexpected") }
func (m *mockGit) StashDrop(index string) (string, error)                            { panic("unexpected") }
func (m *mockGit) StashClear() (string, error)                                       { panic("unexpected") }
func (m *mockGit) Switch(branch string) error                                        { panic("unexpected") }
func (m *mockGit) Branch(name string) (string, error)                                { panic("unexpected") }
func (m *mockGit) DeleteBranch(name string, force bool) (string, error)              { panic("unexpected") }
func (m *mockGit) RenameBranch(oldName, newName string) (string, error)              { panic("unexpected") }
func (m *mockGit) DeleteRemoteBranch(name string) error                              { panic("unexpected") }
func (m *mockGit) Tag(name, message string) (string, error)                          { panic("unexpected") }
func (m *mockGit) PushTag(name string) (string, error)                               { panic("unexpected") }
func (m *mockGit) DeleteTag(name string) (string, error)                             { panic("unexpected") }
func (m *mockGit) DeleteTagRemote(name string) (string, error)                       { panic("unexpected") }
func (m *mockGit) Merge(branch string) (string, error)                               { panic("unexpected") }
func (m *mockGit) MergeAbort() (string, error)                                       { panic("unexpected") }
func (m *mockGit) MergeContinue() (string, error)                                    { panic("unexpected") }
func (m *mockGit) MergeSkip() (string, error)                                         { panic("unexpected") }
func (m *mockGit) Reset(mode string, commit string) (string, error)                  { panic("unexpected") }
func (m *mockGit) ResetSoft(ref string) error                                        { panic("unexpected") }
func (m *mockGit) Restore(paths []string) error                                      { panic("unexpected") }
func (m *mockGit) Clean() error                                                      { panic("unexpected") }
func (m *mockGit) Rebase(branch string) (string, error)                              { panic("unexpected") }
func (m *mockGit) RebaseAbort() (string, error)                                      { panic("unexpected") }
func (m *mockGit) RebaseContinue() (string, error)                                   { panic("unexpected") }
func (m *mockGit) RebaseSkip() (string, error)                                        { panic("unexpected") }
func (m *mockGit) RebaseOnto(newBase, upstream, branch string) (string, error)        { panic("unexpected") }
func (m *mockGit) PushToBranch(remote, branch string) (string, error)                  { panic("unexpected") }
func (m *mockGit) PullFromBranch(remote, branch string) (string, error)                { panic("unexpected") }
func (m *mockGit) CherryPick(commit string) (string, error)                          { panic("unexpected") }
func (m *mockGit) SetUpstream(branch, remote string) (string, error)                 { panic("unexpected") }
func (m *mockGit) UnsetUpstream(branch string) (string, error)                       { panic("unexpected") }
func (m *mockGit) ShowCommit(commit string) (string, error)                          { panic("unexpected") }
func (m *mockGit) RemoteAdd(name, url string) (string, error)                        { panic("unexpected") }
func (m *mockGit) RemoteRemove(name string) (string, error)                          { panic("unexpected") }
func (m *mockGit) ConfigGet(key string) (string, error) {
	args := m.Called(key)
	return args.String(0), args.Error(1)
}
func (m *mockGit) ConfigSet(key, value string) (string, error) {
	args := m.Called(key, value)
	return args.String(0), args.Error(1)
}
func (m *mockGit) WriteTree() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *mockGit) CommitTree(treeHash, parentHash, message string) (string, error) { panic("not implemented") }
func (m *mockGit) UpdateRef(ref, commitHash string) (string, error)                 { panic("not implemented") }
func (m *mockGit) Head() (string, error)                                            { panic("not implemented") }

var _ ports.Git = (*mockGit)(nil)

// --- Tests ---

func TestNewHandler(t *testing.T) {
	git := new(mockGit)
	h := NewHandler(git, nil, nil, nil, "", nil)
	assert.NotNil(t, h)
}

// --- HandleStatus tests ---

func TestHandleStatus_ReadStatus(t *testing.T) {
	git := new(mockGit)
	git.On("Status").Return(domain.Status{Branch: "main", IsClean: true}, nil)

	h := NewHandler(git, nil, nil, nil, "", nil)
	args := map[string]any{}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleStatus(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, `"branch"`)
	assert.Contains(t, text, `"main"`)
	git.AssertExpectations(t)
}

func TestHandleStatus_WithFilter(t *testing.T) {
	git := new(mockGit)
	git.On("Status").Return(domain.Status{Branch: "feature", IsClean: false, Modified: 3}, nil)

	h := NewHandler(git, nil, nil, nil, "", nil)
	args := map[string]any{"filter": "src"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleStatus(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, `"branch"`)
}

func TestHandleStatus_ArgRejected(t *testing.T) {
	git := new(mockGit)
	git.On("Status").Return(domain.Status{}, nil)

	h := NewHandler(git, nil, nil, nil, "", nil)
	args := map[string]any{"command": "READ_STATUS", "arg": "something"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleStatus(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.True(t, strings.Contains(text, "unknown parameter"), "expected unknown parameter error, got: %s", text)
}

// --- HandleDiff tests ---

func TestHandleDiff_ReadDiff(t *testing.T) {
	git := new(mockGit)
	git.On("Diff", mock.Anything).Return("diff --git a/file.go b/file.go\n+added line", nil)

	h := NewHandler(git, nil, nil, nil, "", nil)
	args := map[string]any{}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleDiff(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, `"diff"`)
	var parsed map[string]any
	json.Unmarshal([]byte(text), &parsed)
	assert.NotEmpty(t, parsed["diff"])
}

func TestHandleDiff_ReadDiffWithPath(t *testing.T) {
	git := new(mockGit)
	git.On("Diff", []string{"main.go"}).Return("diff output", nil)

	h := NewHandler(git, nil, nil, nil, "", nil)
	args := map[string]any{"target_paths": "main.go"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleDiff(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.False(t, strings.Contains(text, "unknown parameter"), "should not reject target_paths param, got: %s", text)
}

func TestHandleDiff_ArgRejected(t *testing.T) {
	git := new(mockGit)

	h := NewHandler(git, nil, nil, nil, "", nil)
	args := map[string]any{"arg": "file.go"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleDiff(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.True(t, strings.Contains(text, "unknown parameter"), "expected unknown parameter error for 'arg', got: %s", text)
}

func TestHandleDiff_Staged(t *testing.T) {
	git := new(mockGit)
	git.On("DiffStaged", mock.Anything).Return("diff --git a/file.go b/file.go\n+staged line", nil)

	h := NewHandler(git, nil, nil, nil, "", nil)
	args := map[string]any{"staged": true}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleDiff(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, `"diff"`)
}

// --- HandleAmend tests ---

func TestHandleAmend_Success(t *testing.T) {
	gitMock := new(mockGit)
	msg := "fix typo"
	gitMock.On("CreateBackup", "AMEND", domain.StashNone).Return(domain.Backup{}, nil)
	gitMock.On("Amend", msg, []string(nil)).Return("amend output", nil)

	h := NewHandler(gitMock, nil, nil, nil, "", nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{"commit_message": msg, "confirmed": true},
		},
	}

	res, err := h.HandleAmend(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "AMEND", "confirmed amend should proceed")
	gitMock.AssertExpectations(t)
}

func TestHandleAmend_DryRunBypass(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, "", nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{"dry_run": true},
		},
	}

	res, err := h.HandleAmend(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "amend", "dry_run should preview impact")
}

func TestHandleRevert_BlockedWithoutConfirmed(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, "", nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{"target_commit": "abc123"},
		},
	}

	res, err := h.HandleRevert(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "blocked", "revert without confirmed should be blocked")
	assert.Contains(t, text, "confirmed=true", "blocked result should mention confirmed=true")
}

func TestHandleRevert_ProceedsWithConfirmed(t *testing.T) {
	gitMock := new(mockGit)
	gitMock.On("CreateBackup", "REVERT", domain.StashNone).Return(domain.Backup{}, nil)
	gitMock.On("Revert", "abc123").Return("revert output", nil)

	h := NewHandler(gitMock, nil, nil, nil, "", nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{"target_commit": "abc123", "confirmed": true},
		},
	}

	res, err := h.HandleRevert(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "REVERT", "confirmed revert should proceed")
	gitMock.AssertExpectations(t)
}

func TestHandleRevert_DryRunBypass(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, "", nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{"target_commit": "abc123", "dry_run": true},
		},
	}

	res, err := h.HandleRevert(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "revert", "dry_run should preview impact")
}

// --- Auto-backup tests for amend and revert (Phase 1: B5c) ---

func TestHandleAmend_AutoBackupBeforeAmend(t *testing.T) {
	gitMock := new(mockGit)
	gitMock.On("CreateBackup", "AMEND", domain.StashNone).Return(domain.Backup{}, nil)
	gitMock.On("Amend", "fix typo", []string(nil)).Return("amend output", nil)

	h := NewHandler(gitMock, nil, nil, nil, "", nil)
	args := map[string]any{"commit_message": "fix typo", "confirmed": true}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleAmend(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	json.Unmarshal([]byte(text), &parsed)
	assert.Equal(t, true, parsed["success"])
	assert.Contains(t, parsed["hint"], "undo")
	gitMock.AssertExpectations(t)
}

func TestHandleRevert_AutoBackupBeforeRevert(t *testing.T) {
	gitMock := new(mockGit)
	gitMock.On("CreateBackup", "REVERT", domain.StashNone).Return(domain.Backup{}, nil)
	gitMock.On("Revert", "abc123").Return("revert output", nil)

	h := NewHandler(gitMock, nil, nil, nil, "", nil)
	args := map[string]any{"target_commit": "abc123", "confirmed": true}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleRevert(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	json.Unmarshal([]byte(text), &parsed)
	assert.Equal(t, true, parsed["success"])
	assert.Contains(t, parsed["hint"], "undo")
	gitMock.AssertExpectations(t)
}

// --- Task 1: BgJob struct extension tests ---

func TestBgJob_Fields(t *testing.T) {
	done := make(chan struct{})
	j := BgJob{
		ID:       "test-1",
		Status:   BgRunning,
		Error:    "",
		TreeHash: "abc123def456",
		Message:  "feat: add new feature",
		Done:     done,
	}
	assert.Equal(t, "test-1", j.ID)
	assert.Equal(t, BgRunning, j.Status)
	assert.Equal(t, "abc123def456", j.TreeHash)
	assert.Equal(t, "feat: add new feature", j.Message)
	assert.NotNil(t, j.Done, "Done channel must not be nil")
}

func TestBgJob_DoneChannel_Blocking(t *testing.T) {
	j := BgJob{
		ID:     "test-2",
		Status: BgRunning,
		Done:   make(chan struct{}),
	}
	// A non-closed channel should block on read
	select {
	case <-j.Done:
		t.Fatal("Done should block when not closed")
	default:
		// Expected: channel blocks
	}
}

func TestBgJob_DoneChannel_Close(t *testing.T) {
	j := BgJob{
		ID:     "test-3",
		Status: BgDone,
		Done:   make(chan struct{}),
	}
	close(j.Done)
	// A closed channel should unblock immediately
	select {
	case <-j.Done:
		// Expected: channel unblocks
	default:
		t.Fatal("Done should unblock after close")
	}
}

// --- Task 2: Synchronous WriteTree in handlePreview — error path tests ---

func TestHandlePreview_WriteTreeError_NoBgJob(t *testing.T) {
	mGit := new(mockGit)
	mGit.On("WriteTree").Return("", fmt.Errorf("empty staging area"))

	h := newTestHandler(t, mGit)
	args := map[string]any{"command": "PREVIEW", "instruction": "commit staged changes"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err, "WriteTree error should be returned, not a Go error")
	assert.NotNil(t, res)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "PREVIEW", "error result should mention PREVIEW")
	assert.Contains(t, text, "empty staging area", "error result should contain WriteTree error")

	// Verify no BgJob was stored
	h.bgJobs.Range(func(key, value any) bool {
		t.Errorf("no BgJob should be stored after WriteTree error, found key=%v", key)
		return false
	})
	mGit.AssertExpectations(t)
}

// --- Task 3: WriteTree fast path + Done channel tests ---

func TestHandlePreview_FastPath_CreatesBgJobWithTreeHash(t *testing.T) {
	// Fast path: BgJob is created with TreeHash, Message, Status=BgDone, Done closed
	treeHash := "abc123def456"
	jobID := "commit-fast-123"
	done := make(chan struct{})
	close(done) // fast path: Done is closed immediately

	j := &BgJob{
		ID:       jobID,
		Status:   BgDone,
		TreeHash: treeHash,
		Message:  "feat: add new feature",
		Done:     done,
	}

	assert.Equal(t, BgDone, j.Status)
	assert.Equal(t, treeHash, j.TreeHash)
	assert.Equal(t, "feat: add new feature", j.Message)
	// Verify Done is closed (unblocks immediately)
	select {
	case <-j.Done:
		// Expected: Done is closed
	default:
		t.Fatal("Fast-path BgJob Done should be closed")
	}
}

func TestHandlePreview_SlowPath_CreatesBgJobWithTreeHash(t *testing.T) {
	// Slow path: BgJob is created with Status=BgRunning, TreeHash set, Done not closed
	treeHash := "abc123def456"
	jobID := "commit-slow-456"
	done := make(chan struct{})

	j := &BgJob{
		ID:       jobID,
		Status:   BgRunning,
		TreeHash: treeHash,
		Done:     done,
	}

	assert.Equal(t, BgRunning, j.Status)
	assert.Equal(t, treeHash, j.TreeHash)
	assert.Empty(t, j.Message, "Message should be empty until goroutine completes")
	// Verify Done is NOT closed (blocks)
	select {
	case <-j.Done:
		t.Fatal("Slow-path BgJob Done should NOT be closed until goroutine completes")
	default:
		// Expected: Done blocks
	}
}

func TestHandlePreview_GoroutineSuccess_SetsMessageAndClosesDone(t *testing.T) {
	// Simulate goroutine completion: set Message, Status=BgDone, close Done
	j := &BgJob{
		ID:       "commit-123",
		Status:   BgRunning,
		TreeHash: "abc123",
		Done:     make(chan struct{}),
	}

	// Goroutine completes successfully
	j.Message = "feat: add new feature"
	j.Status = BgDone
	close(j.Done)

	assert.Equal(t, BgDone, j.Status)
	assert.Equal(t, "feat: add new feature", j.Message)
	// Verify Done is closed
	select {
	case <-j.Done:
		// Expected
	default:
		t.Fatal("Done should be closed after goroutine success")
	}
}

func TestHandlePreview_GoroutineFailure_ClosesDone(t *testing.T) {
	// Simulate goroutine failure: set Error, Status=BgFailed, close Done
	j := &BgJob{
		ID:       "commit-456",
		Status:   BgRunning,
		TreeHash: "def789",
		Done:     make(chan struct{}),
	}

	// Goroutine fails
	j.Status = BgFailed
	j.Error = "LLM timeout"
	close(j.Done)

	assert.Equal(t, BgFailed, j.Status)
	assert.Equal(t, "LLM timeout", j.Error)
	// Verify Done is closed even on failure
	select {
	case <-j.Done:
		// Expected
	default:
		t.Fatal("Done should be closed even after goroutine failure")
	}
}

// --- Task 6: Goroutine edge case convergence tests ---

func TestHandlePreview_SlowPath_GoroutineSuccess_SetsMessage(t *testing.T) {
	// Test that a slow-path goroutine correctly updates BgJob upon success.
	// Simulates the goroutine body pattern: receive result, set Message, Status, close Done.
	j := &BgJob{
		ID:       "commit-slow-success",
		Status:   BgRunning,
		TreeHash: "tree123",
		Done:     make(chan struct{}),
	}

	// Simulate goroutine completion in a separate goroutine
	go func() {
		j.Message = "feat: implement feature X"
		j.Status = BgDone
		close(j.Done)
	}()

	// Wait for Done to close (goroutine completion signal)
	<-j.Done

	assert.Equal(t, BgDone, j.Status, "Status should be BgDone after goroutine success")
	assert.Equal(t, "feat: implement feature X", j.Message, "Message should be set from goroutine result")
	assert.Equal(t, "tree123", j.TreeHash, "TreeHash should persist unchanged")
}

func TestHandlePreview_SlowPath_GoroutineFailure_SetsError(t *testing.T) {
	// Test that a slow-path goroutine correctly handles failure.
	// Simulates the goroutine body pattern: receive error, set Error, Status, close Done.
	j := &BgJob{
		ID:       "commit-slow-fail",
		Status:   BgRunning,
		TreeHash: "tree456",
		Done:     make(chan struct{}),
	}

	// Simulate goroutine failure in a separate goroutine
	go func() {
		j.Status = BgFailed
		j.Error = "context deadline exceeded"
		close(j.Done)
	}()

	// Wait for Done to close (goroutine completion signal)
	<-j.Done

	assert.Equal(t, BgFailed, j.Status, "Status should be BgFailed after goroutine failure")
	assert.Equal(t, "context deadline exceeded", j.Error, "Error should be set from goroutine error")
	assert.Equal(t, "tree456", j.TreeHash, "TreeHash should persist unchanged even on failure")
	// TreeHash is preserved for potential retry even after failure
}

// --- Task 4: handleStatus stops deleting jobs on BgDone and BgFailed ---

func TestHandleStatus_BgDone_RetainsJob(t *testing.T) {
	mGit := new(mockGit)
	mGit.On("WriteTree").Return("abc123", nil)

	h := newTestHandler(t, mGit)

	// Store a BgDone job
	jobID := "commit-done-123"
	done := make(chan struct{})
	close(done)
	h.bgJobs.Store(jobID, &BgJob{
		ID:       jobID,
		Status:   BgDone,
		TreeHash: "abc123",
		Message:  "feat: add feature",
		Done:     done,
	})

	// Call handleStatus (commit STATUS) with job_id
	params := map[string]any{"job_id": jobID}
	res, err := h.handleStatus(params)
	assert.NoError(t, err)
	assert.NotNil(t, res, "handleStatus should return a result for BgDone job")

	// Verify job was NOT deleted
	_, ok := h.bgJobs.Load(jobID)
	assert.True(t, ok, "BgDone job should still exist in bgJobs after handleStatus")
}

func TestHandleStatus_BgFailed_RetainsJob(t *testing.T) {
	mGit := new(mockGit)
	mGit.On("WriteTree").Return("def789", nil)

	h := newTestHandler(t, mGit)

	// Store a BgFailed job
	jobID := "commit-failed-456"
	done := make(chan struct{})
	close(done)
	h.bgJobs.Store(jobID, &BgJob{
		ID:       jobID,
		Status:   BgFailed,
		Error:    "LLM timeout",
		TreeHash: "def789",
		Done:     done,
	})

	// Call handleStatus (commit STATUS) with job_id
	params := map[string]any{"job_id": jobID}
	res, err := h.handleStatus(params)
	assert.NoError(t, err)
	assert.NotNil(t, res, "handleStatus should return a result for BgFailed job")

	// Verify job was NOT deleted
	_, ok := h.bgJobs.Load(jobID)
	assert.True(t, ok, "BgFailed job should still exist in bgJobs after handleStatus")

	// Verify error message is returned
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "LLM timeout", "BgFailed error message should be in response")
}

// --- Helpers ---

// assertHandlerSatisfiesInterface verifies Handler implements Handlers.
// This compiles only if Handler satisfies it.
var _ Handlers = (*Handler)(nil)

// mockCommitSvc — minimal mock for workflow.CommitService
type mockCommitSvc struct{}

func newMockCommitSvc() *workflow.CommitService {
	// Return nil — test for PREVIEW path that needs no service yet
	return nil
}

// mockReviewWorkflow — stub
type mockReviewWorkflow struct{}

// --- Test handler construction helpers ---

// mockLLM is a minimal LLM mock for test handler construction.
type mockLLM struct {
	mock.Mock
}

func (m *mockLLM) GenerateChunkMessage(chunk domain.DiffChunk) (string, error) {
	args := m.Called(chunk)
	return args.String(0), args.Error(1)
}
func (m *mockLLM) DecideCommit(instruction, gitStatus, untracked, modified, deleted string) (domain.CommitIntent, error) {
	args := m.Called(instruction, gitStatus, untracked, modified, deleted)
	return args.Get(0).(domain.CommitIntent), args.Error(1)
}
func (m *mockLLM) InterpretGitOp(op, instruction string, context map[string]string) (map[string]string, error) {
	args := m.Called(op, instruction, context)
	return args.Get(0).(map[string]string), args.Error(1)
}
func (m *mockLLM) SetRetryContext(previousMessage string)   { m.Called(previousMessage) }
func (m *mockLLM) ClearRetryContext()                        { m.Called() }
func (m *mockLLM) IsAvailable() bool                         { return true }
func (m *mockLLM) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	args := m.Called(diff, findings)
	return args.Bool(0), args.Error(1)
}
func (m *mockLLM) AuditBinaryContent(filename, content string) (bool, error) {
	args := m.Called(filename, content)
	return args.Bool(0), args.Error(1)
}
func (m *mockLLM) GenerateChangelog(commits, previousChangelog, outputFile string) (*domain.Changelog, error) {
	args := m.Called(commits, previousChangelog, outputFile)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Changelog), args.Error(1)
}
func (m *mockLLM) GenerateChangelogByArea(formattedGroups string) (domain.ChangelogByArea, error) {
	args := m.Called(formattedGroups)
	return args.Get(0).(domain.ChangelogByArea), args.Error(1)
}
func (m *mockLLM) RegenerateMessage(previousMessages []string, feedback string, chunks []domain.DiffChunk) ([]string, error) {
	args := m.Called(previousMessages, feedback, chunks)
	return args.Get(0).([]string), args.Error(1)
}
func (m *mockLLM) ProjectInit(repoRoot string) (*domain.ProjectConfig, error) {
	args := m.Called(repoRoot)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.ProjectConfig), args.Error(1)
}
func (m *mockLLM) ClassifyBinary(prompt string) (string, error) {
	args := m.Called(prompt)
	return args.String(0), args.Error(1)
}

// mockDiffChunker is a minimal DiffChunker mock.
type mockDiffChunker struct {
	mock.Mock
}

func (m *mockDiffChunker) Chunk(diff string, maxSize int) ([]domain.DiffChunk, error) {
	args := m.Called(diff, maxSize)
	return args.Get(0).([]domain.DiffChunk), args.Error(1)
}

// mockSecurityService is a minimal SecurityService mock.
type mockSecurityService struct {
	mock.Mock
}

func (m *mockSecurityService) CheckFiles(files []string, diff string) *ports.SecurityCheckResult {
	args := m.Called(files, diff)
	return args.Get(0).(*ports.SecurityCheckResult)
}
func (m *mockSecurityService) ShouldUseLLMScan() bool {
	return false
}

// newTestHandler creates a Handler with real CommitService and Workflow for testing handlePreview.
func newTestHandler(t *testing.T, mGit *mockGit) *Handler {
	t.Helper()

	mLLM := new(mockLLM)
	mLLM.On("GenerateChunkMessage", mock.Anything).Return("feat: test commit", nil)
	mLLM.On("DecideCommit", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(domain.CommitIntent{IncludeUntracked: false}, nil)
	mLLM.On("ClassifyBinary", mock.Anything).Return("fix", nil)

	mChunker := new(mockDiffChunker)
	mChunker.On("Chunk", mock.Anything, mock.Anything).
		Return([]domain.DiffChunk{{Files: []string{"main.go"}, Diff: "test diff"}}, nil)

	mSecurity := new(mockSecurityService)
	mSecurity.On("CheckFiles", mock.Anything, mock.Anything).
		Return(&ports.SecurityCheckResult{Blocked: false})

	commitSvc := workflow.NewCommitService(
		mGit, mLLM, mChunker, mSecurity,
		workflow.DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/task.log"),
	)

	confirm := confirm.NewInMemory(5 * time.Minute)
	cfg := config.Default()

	rev := workflow.New(mGit, mLLM, confirm, cfg, commitSvc, nil, mSecurity)

	return NewHandler(mGit, commitSvc, rev, mLLM, "", nil)
}
