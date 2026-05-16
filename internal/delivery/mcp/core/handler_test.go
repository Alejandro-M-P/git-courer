package core

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

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
func (m *mockGit) CreateBackup(operation string, mode domain.StashMode) (domain.Backup, error) { panic("unexpected") }
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
func (m *mockGit) Reset(mode string, commit string) (string, error)                  { panic("unexpected") }
func (m *mockGit) ResetSoft(ref string) error                                        { panic("unexpected") }
func (m *mockGit) Restore(paths []string) error                                      { panic("unexpected") }
func (m *mockGit) Clean() error                                                      { panic("unexpected") }
func (m *mockGit) Rebase(branch string) (string, error)                              { panic("unexpected") }
func (m *mockGit) RebaseAbort() (string, error)                                      { panic("unexpected") }
func (m *mockGit) RebaseContinue() (string, error)                                   { panic("unexpected") }
func (m *mockGit) CherryPick(commit string) (string, error)                          { panic("unexpected") }
func (m *mockGit) SetUpstream(branch, remote string) (string, error)                 { panic("unexpected") }
func (m *mockGit) UnsetUpstream(branch string) (string, error)                       { panic("unexpected") }
func (m *mockGit) ShowCommit(commit string) (string, error)                          { panic("unexpected") }
func (m *mockGit) RemoteAdd(name, url string) (string, error)                        { panic("unexpected") }
func (m *mockGit) RemoteRemove(name string) (string, error)                          { panic("unexpected") }

var _ ports.Git = (*mockGit)(nil)

// --- Tests ---

func TestNewHandler(t *testing.T) {
	git := new(mockGit)
	h := NewHandler(git, nil, nil, nil, nil, "")
	assert.NotNil(t, h)
	assert.NotNil(t, h.jobs)
}

// --- HandleStatus tests ---

func TestHandleStatus_ReadStatus(t *testing.T) {
	git := new(mockGit)
	git.On("Status").Return(domain.Status{Branch: "main", IsClean: true}, nil)

	h := NewHandler(git, nil, nil, nil, nil, "")
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

	h := NewHandler(git, nil, nil, nil, nil, "")
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

	h := NewHandler(git, nil, nil, nil, nil, "")
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

	h := NewHandler(git, nil, nil, nil, nil, "")
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

	h := NewHandler(git, nil, nil, nil, nil, "")
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

	h := NewHandler(git, nil, nil, nil, nil, "")
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

	h := NewHandler(git, nil, nil, nil, nil, "")
	args := map[string]any{"staged": true}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleDiff(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, `"diff"`)
}

// --- HandleAmend tests ---

func TestHandleAmend_Success(t *testing.T) {
	git := new(mockGit)
	msg := "updated commit message"
	git.On("Amend", msg, []string(nil)).Return("amend output", nil)

	h := NewHandler(git, nil, nil, nil, nil, "")
	args := map[string]any{"commit_message": msg}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleAmend(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	json.Unmarshal([]byte(text), &parsed)
	assert.Equal(t, true, parsed["success"])
	assert.Equal(t, "AMEND", parsed["operation"])
}

func TestHandleAmend_DryRun(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, nil, "")
	args := map[string]any{"commit_message": "msg", "dry_run": true}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleAmend(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	json.Unmarshal([]byte(text), &parsed)
	assert.Equal(t, "amend", parsed["operation"])
}

// --- HandleRevert tests ---

func TestHandleRevert_Success(t *testing.T) {
	git := new(mockGit)
	commit := "abc123"
	git.On("Revert", commit).Return("revert output", nil)

	h := NewHandler(git, nil, nil, nil, nil, "")
	args := map[string]any{"target_commit": commit}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleRevert(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	json.Unmarshal([]byte(text), &parsed)
	assert.Equal(t, true, parsed["success"])
	assert.Equal(t, "REVERT", parsed["operation"])
}

func TestHandleRevert_DryRun(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, nil, "")
	args := map[string]any{"target_commit": "abc123", "dry_run": true}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleRevert(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	json.Unmarshal([]byte(text), &parsed)
	assert.Equal(t, "revert", parsed["operation"])
}

func TestHandleAmend_DryRunParamAccepted(t *testing.T) {
	// Verify that dry_run param is accepted without unknown-parameter error
	h := NewHandler(nil, nil, nil, nil, nil, "")
	args := map[string]any{"dry_run": true, "commit_message": "test"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleAmend(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.False(t, strings.Contains(text, "unknown parameter"), "dry_run should be accepted, got: %s", text)
}

func TestHandleRevert_DryRunParamAccepted(t *testing.T) {
	// Verify that dry_run param is accepted without unknown-parameter error
	h := NewHandler(nil, nil, nil, nil, nil, "")
	args := map[string]any{"target_commit": "abc123", "dry_run": true}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleRevert(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.False(t, strings.Contains(text, "unknown parameter"), "dry_run should be accepted, got: %s", text)
}

func TestHandleRevert_MissingTargetCommit(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, nil, "")
	args := map[string]any{} // no target_commit
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleRevert(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "error")
}

// --- HandleCommit tests (stub for now — will be implemented in 4.3) ---

func TestHandleCommit_Preview(t *testing.T) {
	// This test will fail until the handler is implemented in 4.3
	// We are in RED phase — writing the test before the implementation.
	git := new(mockGit)
	h := NewHandler(git, nil, nil, nil, nil, "")
	args := map[string]any{"command": "PREVIEW", "instruction": "commit all changes"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
}

func TestHandleCommit_Abort(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, nil, "")
	args := map[string]any{"command": "ABORT", "job_id": "nonexistent"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "aborted")
}

// --- Job storage helpers test ---

func TestHandler_StoreAndGetJob(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, nil, "")
	jobID := "test-job-123"
	plan := &domain.OperationPlan{
		Operation:  "commit",
		Messages:   []string{"feat: test"},
		Instruction: "test",
	}
	h.jobs.Store(jobID, plan)

	loaded, ok := h.jobs.Load(jobID)
	assert.True(t, ok)
	assert.Equal(t, plan, loaded)
}

func TestHandler_DeleteJob(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, nil, "")
	jobID := "test-job-456"
	h.jobs.Store(jobID, "dummy")
	h.jobs.Delete(jobID)

	_, ok := h.jobs.Load(jobID)
	assert.False(t, ok)
}

func TestHandler_NonExistentJob(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, nil, "")
	_, ok := h.jobs.Load("does-not-exist")
	assert.False(t, ok)
}

// --- Helpers ---

// assertHandlerSatisfiesInterface verifies Handler implements Handlers.
// This compiles only if Handler satisfies it.
var _ Handlers = (*Handler)(nil)

// Ensure sync.Map is properly initialized by constructor.
func TestHandler_JobsInitialized(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, nil, "")
	assert.NotNil(t, h.jobs)

	// Verify sync.Map is a pointer to zero-value sync.Map (usable).
	var zero sync.Map
	h.jobs.Store("key", "value")
	v, ok := h.jobs.Load("key")
	assert.True(t, ok)
	assert.Equal(t, "value", v)
	h.jobs.Delete("key")

	// Reset to zero-value (h.jobs is a struct field, so we assign zero)
	h.jobs = zero
	assert.NotNil(t, &h.jobs)
}

// mockCommitSvc — minimal mock for workflow.CommitService
type mockCommitSvc struct{}

func newMockCommitSvc() *workflow.CommitService {
	// Return nil — test for PREVIEW path that needs no service yet
	return nil
}

// mockReviewWorkflow — stub
type mockReviewWorkflow struct{}
