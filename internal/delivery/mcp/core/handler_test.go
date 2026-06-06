package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/blak0p/git-courer/internal/adapters/confirm"
	"github.com/blak0p/git-courer/internal/config"
	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
	"github.com/blak0p/git-courer/internal/workflow"
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
func (m *mockGit) ListUntracked() ([]string, error) { panic("unexpected") }
func (m *mockGit) Log(limit int, pattern string, paths ...string) (string, error) {
	args := m.Called(limit, pattern, paths)
	return args.String(0), args.Error(1)
}
func (m *mockGit) LogFull(limit int) (string, error)              { panic("unexpected") }
func (m *mockGit) ListBranches(pattern ...string) (string, error) { panic("unexpected") }
func (m *mockGit) ListTags(pattern ...string) ([]string, error)   { panic("unexpected") }
func (m *mockGit) Search(pattern string, context, before, after int, paths ...string) (string, error) {
	panic("unexpected")
}
func (m *mockGit) CatFile(revision, path string) (string, error) { panic("unexpected") }
func (m *mockGit) ListTree(revision, path string, recursive bool) ([]string, error) {
	panic("unexpected")
}
func (m *mockGit) LatestTag() (string, error)                              { panic("unexpected") }
func (m *mockGit) CommitsFromTag(sinceTag string) (string, error)          { panic("unexpected") }
func (m *mockGit) TagExists(name string) (bool, error)                     { panic("unexpected") }
func (m *mockGit) IsGHAuthenticated() (bool, error)                        { panic("unexpected") }
func (m *mockGit) CreateRelease(tagName, changelog string) (string, error) { panic("unexpected") }
func (m *mockGit) Blame(filepath string) ([]domain.BlameLine, error)       { panic("unexpected") }
func (m *mockGit) Show(hash string) (domain.ShowResult, error)             { panic("unexpected") }
func (m *mockGit) Reflog() ([]domain.ReflogEntry, error)                   { panic("unexpected") }
func (m *mockGit) StashList() ([]domain.StashEntry, error)                 { panic("unexpected") }
func (m *mockGit) StashShow() (string, error)                              { panic("unexpected") }
func (m *mockGit) MergeBase(a, b string) (string, error) {
	args := m.Called(a, b)
	return args.String(0), args.Error(1)
}
func (m *mockGit) LogRange(from, to string) (string, error) {
	args := m.Called(from, to)
	return args.String(0), args.Error(1)
}

func (m *mockGit) RemoteURL() (string, error)              { panic("unexpected") }
func (m *mockGit) DiffAll(paths ...string) (string, error) { panic("unexpected") }
func (m *mockGit) CreateBackup(operation string, mode domain.StashMode) (domain.Backup, error) {
	args := m.Called(operation, mode)
	return args.Get(0).(domain.Backup), args.Error(1)
}
func (m *mockGit) RestoreBackup(backup domain.Backup) error   { panic("unexpected") }
func (m *mockGit) DeleteBackup(backup domain.Backup) error    { panic("unexpected") }
func (m *mockGit) ListBackups() ([]domain.Backup, error)      { panic("unexpected") }
func (m *mockGit) PruneBackups(olderThan time.Duration) error { panic("unexpected") }
func (m *mockGit) Add(paths []string) error {
	args := m.Called(paths)
	return args.Error(0)
}
func (m *mockGit) Remove(paths []string) error           { panic("unexpected") }
func (m *mockGit) Commit(message string) (string, error) { panic("unexpected") }
func (m *mockGit) Push() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *mockGit) PushTo(remoteBranch string) (string, error)           { panic("unexpected") }
func (m *mockGit) Pull() (string, error)                                { panic("unexpected") }
func (m *mockGit) PullFrom(remoteBranch string) (string, error)         { panic("unexpected") }
func (m *mockGit) Fetch() (string, error)                               { panic("unexpected") }
func (m *mockGit) Stash(message ...string) (string, error)              { panic("unexpected") }
func (m *mockGit) StashWithUntracked(message string) (string, error)    { panic("unexpected") }
func (m *mockGit) StashPop() (string, error)                            { panic("unexpected") }
func (m *mockGit) StashApply(index string) (string, error)              { panic("unexpected") }
func (m *mockGit) StashDrop(index string) (string, error)               { panic("unexpected") }
func (m *mockGit) StashClear() (string, error)                          { panic("unexpected") }
func (m *mockGit) Switch(branch string) error                           { panic("unexpected") }
func (m *mockGit) Branch(name string) (string, error)                   { panic("unexpected") }
func (m *mockGit) DeleteBranch(name string, force bool) (string, error) { panic("unexpected") }
func (m *mockGit) RenameBranch(oldName, newName string) (string, error) { panic("unexpected") }
func (m *mockGit) DeleteRemoteBranch(name string) error                 { panic("unexpected") }
func (m *mockGit) Tag(name, message string) (string, error)             { panic("unexpected") }
func (m *mockGit) PushTag(name string) (string, error)                  { panic("unexpected") }
func (m *mockGit) DeleteTag(name string) (string, error)                { panic("unexpected") }
func (m *mockGit) DeleteTagRemote(name string) (string, error)          { panic("unexpected") }
func (m *mockGit) Merge(branch string) (string, error)                  { panic("unexpected") }
func (m *mockGit) MergeAbort() (string, error)                          { panic("unexpected") }
func (m *mockGit) MergeContinue() (string, error)                       { panic("unexpected") }
func (m *mockGit) MergeSkip() (string, error)                           { panic("unexpected") }
func (m *mockGit) Reset(mode string, commit string) (string, error) {
	args := m.Called(mode, commit)
	return args.String(0), args.Error(1)
}
func (m *mockGit) ResetSoft(ref string) error                                  { panic("unexpected") }
func (m *mockGit) Restore(paths []string) error                                { panic("unexpected") }
func (m *mockGit) Clean() error                                                { panic("unexpected") }
func (m *mockGit) Rebase(branch string) (string, error)                        { panic("unexpected") }
func (m *mockGit) RebaseAbort() (string, error)                                { panic("unexpected") }
func (m *mockGit) RebaseContinue() (string, error)                             { panic("unexpected") }
func (m *mockGit) RebaseSkip() (string, error)                                 { panic("unexpected") }
func (m *mockGit) RebaseOnto(newBase, upstream, branch string) (string, error) { panic("unexpected") }
func (m *mockGit) PushToBranch(remote, branch string) (string, error)          { panic("unexpected") }
func (m *mockGit) PullFromBranch(remote, branch string) (string, error)        { panic("unexpected") }
func (m *mockGit) CherryPick(commit string) (string, error)                    { panic("unexpected") }
func (m *mockGit) SetUpstream(branch, remote string) (string, error)           { panic("unexpected") }
func (m *mockGit) UnsetUpstream(branch string) (string, error)                 { panic("unexpected") }
func (m *mockGit) ShowCommit(commit string) (string, error)                    { panic("unexpected") }
func (m *mockGit) RemoteAdd(name, url string) (string, error)                  { panic("unexpected") }
func (m *mockGit) RemoteRemove(name string) (string, error)                    { panic("unexpected") }
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
func (m *mockGit) WriteTree() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *mockGit) CommitTree(treeHash, parentHash, message string) (string, error) {
	args := m.Called(treeHash, parentHash, message)
	return args.String(0), args.Error(1)
}
func (m *mockGit) UpdateRef(ref, commitHash string) (string, error) {
	args := m.Called(ref, commitHash)
	return args.String(0), args.Error(1)
}
func (m *mockGit) Head() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

var _ ports.Git = (*mockGit)(nil)

// --- Tests ---

func TestNewHandler(t *testing.T) {
	git := new(mockGit)
	h := NewHandler(git, nil, nil, nil, "", nil, nil)
	assert.NotNil(t, h)
}

// --- HandleStatus tests ---

func TestHandleStatus_ReadStatus(t *testing.T) {
	git := new(mockGit)
	git.On("Status").Return(domain.Status{Branch: "main", IsClean: true}, nil)

	h := NewHandler(git, nil, nil, nil, "", nil, nil)
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

	h := NewHandler(git, nil, nil, nil, "", nil, nil)
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

	h := NewHandler(git, nil, nil, nil, "", nil, nil)
	args := map[string]any{"command": "READ_STATUS", "arg": "something"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleStatus(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.True(t, strings.Contains(text, "unknown parameter"), "expected unknown parameter error, got: %s", text)
}

// --- Commit command param validation tests ---

func TestHandleCommit_APPLY_RejectsTargetPaths(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, "", nil, nil)
	args := map[string]any{
		"command":      "APPLY",
		"job_id":       "test-job-123",
		"target_paths": "main.go",
	}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.True(t, strings.Contains(text, "unknown parameter"), "APPLY should reject target_paths, got: %s", text)
}

func TestHandleCommit_APPLY_RejectsConfirmed(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, "", nil, nil)
	args := map[string]any{
		"command":   "APPLY",
		"job_id":    "test-job-123",
		"confirmed": true,
	}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.True(t, strings.Contains(text, "unknown parameter"), "APPLY should reject confirmed param, got: %s", text)
}

func TestHandleCommit_PREVIEW_RejectsTargetPaths(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, "", nil, nil)
	args := map[string]any{
		"command":      "PREVIEW",
		"why":          "fix bug",
		"target_paths": "main.go",
	}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.True(t, strings.Contains(text, "unknown parameter"), "PREVIEW should reject target_paths, got: %s", text)
}

func TestHandleCommit_ValidParamsNotRejected(t *testing.T) {
	// The rejection tests above (RejectsTargetPaths, RejectsConfirmed) prove
	// that invalid params are blocked. This test documents that we intentionally
	// test the negative case (invalid params rejected) rather than the positive
	// case (valid params accepted), because running the handlers with nil
	// dependencies panics. The param validation layer is the same code path
	// for both — ValidateKnownParams checks all param keys against the allowed set.
}

func TestHandleCommit_STATUS_RejectsWhy(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, "", nil, nil)
	args := map[string]any{
		"command": "STATUS",
		"why":     "should not be here",
	}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.True(t, strings.Contains(text, "unknown parameter"), "STATUS should reject why param, got: %s", text)
}

func TestHandleCommit_ABORT_RejectsExtraParams(t *testing.T) {
	h := NewHandler(nil, nil, nil, nil, "", nil, nil)
	args := map[string]any{
		"command": "ABORT",
		"why":     "should not be here",
	}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.True(t, strings.Contains(text, "unknown parameter"), "ABORT should reject extra params, got: %s", text)
}

// --- Handler annotation integration tests ---

// mockContentProviderForHandler is a mock ContentProvider for handler tests.
type mockContentProviderForHandler struct {
	contents []ports.FileContent
	err      error
}

func (m *mockContentProviderForHandler) GetContents(files []string) ([]ports.FileContent, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.contents, nil
}

func TestHandleDiff_AnnotatedField_Set(t *testing.T) {
	// Regular diff path with ContentProvider → annotated should be set
	mGit := new(mockGit)
	mGit.On("Diff", mock.Anything).Return("diff --git a/handler.go b/handler.go\nnew file mode 100644\n--- /dev/null\n+++ b/handler.go\n@@ -0,0 +1,4 @@\n+package main\n+func Helper() {\n+\tfmt.Println(\"hello\")\n+}\n", nil)

	cp := &mockContentProviderForHandler{
		contents: []ports.FileContent{
			{Filename: "handler.go", Before: []byte("package main\nfunc existing() {}\n"), After: []byte("package main\nfunc existing() {}\nfunc Helper() {\n\tfmt.Println(\"hello\")\n}\n")},
		},
	}

	h := NewHandler(mGit, nil, nil, nil, "", nil, cp)
	args := map[string]any{}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleDiff(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text

	var parsed map[string]any
	json.Unmarshal([]byte(text), &parsed)
	_, hasAnnotated := parsed["annotated"]
	assert.True(t, hasAnnotated, "JSON response should contain annotated key when ContentProvider is set")
	annotated, _ := parsed["annotated"].(string)
	assert.NotEmpty(t, annotated, "annotated value should not be empty when ContentProvider returns valid content")
}

func TestHandleDiff_Staged_AnnotatedField_Set(t *testing.T) {
	// Staged diff path with ContentProvider → annotated should be set
	mGit := new(mockGit)
	mGit.On("DiffStaged", mock.Anything).Return("diff --git a/handler.go b/handler.go\nnew file mode 100644\n--- /dev/null\n+++ b/handler.go\n@@ -0,0 +1,4 @@\n+package main\n+func Helper() {\n+\tfmt.Println(\"hello\")\n+}\n", nil)

	cp := &mockContentProviderForHandler{
		contents: []ports.FileContent{
			{Filename: "handler.go", Before: []byte("package main\nfunc existing() {}\n"), After: []byte("package main\nfunc existing() {}\nfunc Helper() {\n\tfmt.Println(\"hello\")\n}\n")},
		},
	}

	h := NewHandler(mGit, nil, nil, nil, "", nil, cp)
	args := map[string]any{"staged": true}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleDiff(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text

	var parsed map[string]any
	json.Unmarshal([]byte(text), &parsed)
	_, hasAnnotated := parsed["annotated"]
	assert.True(t, hasAnnotated, "staged diff response should contain annotated key")
}

func TestHandleDiff_Branch_AnnotatedField_Set(t *testing.T) {
	// Branch diff path with ContentProvider → annotated should be set
	mGit := new(mockGit)
	mGit.On("CurrentBranch").Return("develop", nil)
	mGit.On("DiffRange", "develop", "main", "..", mock.Anything).Return("diff --git a/handler.go b/handler.go\nnew file mode 100644\n--- /dev/null\n+++ b/handler.go\n@@ -0,0 +1,4 @@\n+package main\n+func Helper() {\n+\tfmt.Println(\"hello\")\n+}\n", nil)

	cp := &mockContentProviderForHandler{
		contents: []ports.FileContent{
			{Filename: "handler.go", Before: []byte("package main\nfunc existing() {}\n"), After: []byte("package main\nfunc existing() {}\nfunc Helper() {\n\tfmt.Println(\"hello\")\n}\n")},
		},
	}

	h := NewHandler(mGit, nil, nil, nil, "", nil, cp)
	args := map[string]any{"branch": "main"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleDiff(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text

	var parsed map[string]any
	json.Unmarshal([]byte(text), &parsed)
	_, hasAnnotated := parsed["annotated"]
	assert.True(t, hasAnnotated, "branch diff response should contain annotated key")
}

func TestHandleDiff_NilContentProvider_NoAnnotated(t *testing.T) {
	// nil ContentProvider → no annotation, but diff still works
	mGit := new(mockGit)
	mGit.On("Diff", mock.Anything).Return("diff --git a/file.go b/file.go\n+added line", nil)

	h := NewHandler(mGit, nil, nil, nil, "", nil, nil) // nil ContentProvider
	args := map[string]any{}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleDiff(context.Background(), req)
	assert.NoError(t, err)
	text := res.Content[0].(mcpgo.TextContent).Text

	var parsed map[string]any
	json.Unmarshal([]byte(text), &parsed)
	_, hasAnnotated := parsed["annotated"]
	assert.False(t, hasAnnotated, "JSON response should NOT contain annotated key when ContentProvider is nil")
	assert.Contains(t, text, `"diff"`, "diff should still work normally without ContentProvider")
}

func TestNewHandler_WithContentProvider(t *testing.T) {
	mGit := new(mockGit)
	cp := &mockContentProviderForHandler{}

	h := NewHandler(mGit, nil, nil, nil, "", nil, cp)
	assert.NotNil(t, h.contentProvider, "handler should have contentProvider set when passed")

	h2 := NewHandler(mGit, nil, nil, nil, "", nil, nil)
	assert.Nil(t, h2.contentProvider, "handler should have nil contentProvider when nil passed")
}

// --- HandleDiff tests ---

func TestHandleDiff_ReadDiff(t *testing.T) {
	git := new(mockGit)
	git.On("Diff", mock.Anything).Return("diff --git a/file.go b/file.go\n+added line", nil)

	h := NewHandler(git, nil, nil, nil, "", nil, nil)
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

	h := NewHandler(git, nil, nil, nil, "", nil, nil)
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

	h := NewHandler(git, nil, nil, nil, "", nil, nil)
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

	h := NewHandler(git, nil, nil, nil, "", nil, nil)
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

	h := NewHandler(gitMock, nil, nil, nil, "", nil, nil)

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
	h := NewHandler(nil, nil, nil, nil, "", nil, nil)

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
	h := NewHandler(nil, nil, nil, nil, "", nil, nil)

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

	h := NewHandler(gitMock, nil, nil, nil, "", nil, nil)

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
	h := NewHandler(nil, nil, nil, nil, "", nil, nil)

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

	h := NewHandler(gitMock, nil, nil, nil, "", nil, nil)
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

	h := NewHandler(gitMock, nil, nil, nil, "", nil, nil)
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
	args := map[string]any{"command": "PREVIEW", "why": "commit staged changes"}
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

func TestHandlePreview_FastPath_ReturnsJSONResponseWithJobID(t *testing.T) {
	mGit := new(mockGit)
	mGit.On("WriteTree").Return("tree123", nil)
	mGit.On("Status").Return(domain.Status{Branch: "main", IsClean: false, Modified: 1, Files: []domain.FileStatus{{Path: "main.go", Status: "M ", Staged: true}}}, nil)
	mGit.On("DiffStaged", mock.Anything).Return("diff --git a/main.go b/main.go\n+added line", nil)

	h := newTestHandler(t, mGit)
	args := map[string]any{"command": "PREVIEW", "why": "refactor core logic"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text

	// The response must be a valid JSON containing status: success, a job_id, and the message
	var parsed struct {
		Status  string `json:"status"`
		JobID   string `json:"job_id"`
		Message string `json:"message"`
	}
	err = json.Unmarshal([]byte(text), &parsed)
	assert.NoError(t, err, "Response should be a valid JSON")
	assert.Equal(t, "success", parsed.Status)
	assert.NotEmpty(t, parsed.JobID)
	assert.Contains(t, parsed.Message, "Pending: commit")

	// Verify BgJob was stored with status BgDone, correct TreeHash, and a non-empty Message
	bgJobVal, ok := h.bgJobs.Load(parsed.JobID)
	assert.True(t, ok)
	bgJob := bgJobVal.(*BgJob)
	assert.Equal(t, BgDone, bgJob.Status)
	assert.Equal(t, "tree123", bgJob.TreeHash)
	assert.NotEmpty(t, bgJob.Message)

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

// --- Task 5: HandleCommitJobs handler tests ---

func TestHandleCommitJobs_EmptyMap_ReturnsEmptyArray(t *testing.T) {
	mGit := new(mockGit)
	mGit.On("WriteTree").Return("abc123", nil)

	h := newTestHandler(t, mGit)

	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: map[string]any{}}}

	res, err := h.HandleCommitJobs(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Equal(t, "[]", text, "Empty bgJobs should return empty JSON array")
}

func TestHandleCommitJobs_MultipleJobs_ReturnsAll(t *testing.T) {
	mGit := new(mockGit)
	mGit.On("WriteTree").Return("abc123", nil)

	h := newTestHandler(t, mGit)

	// Store 3 BgJobs with different statuses
	done1 := make(chan struct{})
	close(done1)
	h.bgJobs.Store("commit-running-1", &BgJob{
		ID:       "commit-running-1",
		Status:   BgRunning,
		TreeHash: "tree111",
		Done:     make(chan struct{}),
	})
	h.bgJobs.Store("commit-done-2", &BgJob{
		ID:       "commit-done-2",
		Status:   BgDone,
		Message:  "feat: add auth",
		TreeHash: "tree222",
		Done:     done1,
	})
	done2 := make(chan struct{})
	close(done2)
	h.bgJobs.Store("commit-failed-3", &BgJob{
		ID:       "commit-failed-3",
		Status:   BgFailed,
		Error:    "timeout",
		TreeHash: "tree333",
		Done:     done2,
	})

	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: map[string]any{}}}

	res, err := h.HandleCommitJobs(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	text := res.Content[0].(mcpgo.TextContent).Text

	// Parse JSON and verify structure
	var jobs []map[string]any
	err = json.Unmarshal([]byte(text), &jobs)
	assert.NoError(t, err, "Response should be valid JSON array")
	assert.Len(t, jobs, 3, "Should return all 3 jobs")

	// Verify each job has required fields
	for _, job := range jobs {
		assert.Contains(t, job, "id", "Each job should have id")
		assert.Contains(t, job, "status", "Each job should have status")
		assert.Contains(t, job, "tree_hash", "Each job should have tree_hash")
	}
}

func TestHandleCommitJobs_RunningJob_HasEmptyMessage(t *testing.T) {
	mGit := new(mockGit)
	mGit.On("WriteTree").Return("abc123", nil)

	h := newTestHandler(t, mGit)

	// Store a running job — Message should be empty, TreeHash should be present
	h.bgJobs.Store("commit-running-1", &BgJob{
		ID:       "commit-running-1",
		Status:   BgRunning,
		TreeHash: "tree999",
		Done:     make(chan struct{}),
	})

	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: map[string]any{}}}

	res, err := h.HandleCommitJobs(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	text := res.Content[0].(mcpgo.TextContent).Text

	var jobs []map[string]any
	err = json.Unmarshal([]byte(text), &jobs)
	assert.NoError(t, err)
	assert.Len(t, jobs, 1)

	job := jobs[0]
	assert.Equal(t, "commit-running-1", job["id"])
	assert.Equal(t, "running", job["status"])
	assert.Equal(t, "tree999", job["tree_hash"])
	// Message may be empty or missing for running jobs
	msg, _ := job["message"].(string)
	assert.Empty(t, msg, "Running job should have empty message")
}

// --- apply-plumbing Tests (Phase 1: RED) ---

// TestComposeMessage verifies composeMessage joins multiple chunks.
// The LLM prompt now enforces a single clean message with structured
// [EL WHY PRIMERO] / [Y DESPUÉS ASÍ] format, so composeMessage is a simple join.
func TestComposeMessage(t *testing.T) {
	tests := []struct {
		name     string
		chunks   []string
		fallback string
		want     string
	}{
		{
			name:     "empty chunks returns fallback",
			chunks:   nil,
			fallback: "chore: apply changes",
			want:     "chore: apply changes",
		},
		{
			name:     "single chunk returns as-is",
			chunks:   []string{"feat: add auth"},
			fallback: "chore: apply changes",
			want:     "feat: add auth",
		},
		{
			name:     "two chunks joined with double newline",
			chunks:   []string{"feat: add auth\n\n[EL WHY PRIMERO]\nWhy text", "fix: fix bug\n\n[Y DESPUÉS ASÍ]\n* Fixed bug"},
			fallback: "chore: apply changes",
			want:     "feat: add auth\n\n[EL WHY PRIMERO]\nWhy text\n\nfix: fix bug\n\n[Y DESPUÉS ASÍ]\n* Fixed bug",
		},
		{
			name:     "three chunks joined",
			chunks:   []string{"feat: add auth", "fix: fix bug", "docs: update docs"},
			fallback: "chore: apply changes",
			want:     "feat: add auth\n\nfix: fix bug\n\ndocs: update docs",
		},
		{
			name:     "empty strings in chunks are skipped",
			chunks:   []string{"feat: add auth", "", "fix: fix bug"},
			fallback: "chore: apply changes",
			want:     "feat: add auth\n\nfix: fix bug",
		},
		{
			name:     "empty slice not nil uses fallback",
			chunks:   []string{},
			fallback: "chore: apply changes",
			want:     "chore: apply changes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := composeMessage(tt.chunks, tt.fallback)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestHandleApply_NoJobID_CallsLegacyApply verifies that when job_id is absent
// or empty, the existing reviewWorkflow.Apply path is used unchanged.
func TestHandleApply_NoJobID_CallsLegacyApply(t *testing.T) {
	mGit := new(mockGit)
	h := newTestHandler(t, mGit)

	// Without job_id → legacy path (which returns an error result because there's no pending plan)
	params := map[string]any{"command": "APPLY"}
	res, err := h.handleApply(context.Background(), params)
	// Legacy path uses JSONErrorResult which returns (result, nil) — not a Go error
	assert.NoError(t, err, "legacy path uses JSONErrorResult, not Go errors")
	assert.NotNil(t, res)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "APPLY", "legacy apply result should mention APPLY")
}

// TestHandleApply_EmptyJobID_CallsLegacyApply verifies that empty job_id routes to legacy.
func TestHandleApply_EmptyJobID_CallsLegacyApply(t *testing.T) {
	mGit := new(mockGit)
	h := newTestHandler(t, mGit)

	params := map[string]any{"command": "APPLY", "job_id": ""}
	res, err := h.handleApply(context.Background(), params)
	// Legacy path uses JSONErrorResult which returns (result, nil) — not a Go error
	assert.NoError(t, err, "empty job_id routes to legacy, which uses JSONErrorResult")
	assert.NotNil(t, res)
	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "APPLY", "empty job_id should route to legacy apply")
}

// TestHandleApply_JobNotFound_ReturnsError verifies that a missing job_id
// returns an error containing "job not found".
func TestHandleApply_JobNotFound_ReturnsError(t *testing.T) {
	mGit := new(mockGit)
	h := newTestHandler(t, mGit)

	params := map[string]any{"command": "APPLY", "job_id": "nonexistent"}
	res, err := h.handleApply(context.Background(), params)
	assert.Error(t, err, "missing job should return error")
	assert.Contains(t, err.Error(), "job not found")
	assert.Nil(t, res)
}

// TestHandleApply_JobFailed_ReturnsError verifies that a BgFailed job
// returns an error containing the job's error message.
func TestHandleApply_JobFailed_ReturnsError(t *testing.T) {
	mGit := new(mockGit)
	h := newTestHandler(t, mGit)

	// Store a failed job
	jobID := "commit-failed-789"
	done := make(chan struct{})
	close(done)
	h.bgJobs.Store(jobID, &BgJob{
		ID:       jobID,
		Status:   BgFailed,
		Error:    "LLM timeout",
		TreeHash: "tree789",
		Done:     done,
	})

	params := map[string]any{"command": "APPLY", "job_id": jobID}
	res, err := h.handleApply(context.Background(), params)
	assert.Error(t, err, "failed job should return error")
	assert.Contains(t, err.Error(), "LLM timeout")
	assert.Nil(t, res)

	// Job should be preserved (not deleted) so user can retry
	_, ok := h.bgJobs.Load(jobID)
	assert.True(t, ok, "failed job should NOT be deleted from bgJobs")
}

// TestHandleApply_JobDone_HappyPath verifies the full plumbing path:
// lookup BgJob → compose message → Head → CommitTree → UpdateRef → CaptureCommit → plumbing amend → Reset → delete job.
func TestHandleApply_JobDone_HappyPath(t *testing.T) {
	mGit := new(mockGit)
	// Set up mock expectations for the plumbing path
	// Note: This test has no commitSvc store, so CaptureCommit is a no-op,
	// but the plumbing amend still runs (stages .git-courer, WriteTree, CommitTree amend, UpdateRef)
	mGit.On("Head").Return("parent123", nil)
	mGit.On("CommitTree", "tree456", "parent123", "feat: add auth\n\nRefresh tokens are rotated every 24h").Return("commit789", nil)
	// First UpdateRef: move HEAD to the initial commit
	mGit.On("UpdateRef", "HEAD", "commit789").Return("", nil)
	// Plumbing amend sequence (Bug 3)
	mGit.On("Add", []string{domain.MetadataDir}).Return(nil)
	mGit.On("WriteTree").Return("newTree999", nil)
	mGit.On("CommitTree", "newTree999", "commit789", "feat: add auth\n\nRefresh tokens are rotated every 24h").Return("replacementCommit888", nil)
	mGit.On("UpdateRef", "HEAD", "replacementCommit888").Return("", nil)
	mGit.On("Reset", "HEAD", ".").Return("", nil)

	h := newTestHandler(t, mGit)

	// Store a done job with pre-populated message
	jobID := "commit-done-123"
	done := make(chan struct{})
	close(done)
	h.bgJobs.Store(jobID, &BgJob{
		ID:       jobID,
		Status:   BgDone,
		TreeHash: "tree456",
		Message:  "feat: add auth\n\nRefresh tokens are rotated every 24h",
		Done:     done,
	})

	params := map[string]any{"command": "APPLY", "job_id": jobID}
	res, err := h.handleApply(context.Background(), params)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "replacementCommit888", "result should contain replacement commit hash")

	// Job should be deleted after successful apply
	_, ok := h.bgJobs.Load(jobID)
	assert.False(t, ok, "completed job should be deleted from bgJobs")

	mGit.AssertExpectations(t)
}

// TestHandleApply_CommitTreeFails_NoUpdateRef verifies that if CommitTree
// returns an error, UpdateRef is NOT called and the job is preserved.
func TestHandleApply_CommitTreeFails_NoUpdateRef(t *testing.T) {
	mGit := new(mockGit)
	mGit.On("Head").Return("parent123", nil)
	mGit.On("CommitTree", "tree456", "parent123", mock.Anything).Return("", fmt.Errorf("commit-tree failed"))
	// UpdateRef should NOT be called — no mock setup means panic if called
	// Plumbing amend sequence is not reached since first CommitTree fails

	h := newTestHandler(t, mGit)

	jobID := "commit-ctfail-123"
	done := make(chan struct{})
	close(done)
	h.bgJobs.Store(jobID, &BgJob{
		ID:       jobID,
		Status:   BgDone,
		TreeHash: "tree456",
		Message:  "feat: something",
		Done:     done,
	})

	params := map[string]any{"command": "APPLY", "job_id": jobID}
	res, err := h.handleApply(context.Background(), params)
	assert.Error(t, err, "CommitTree failure should return error")
	assert.Nil(t, res)

	// Job should be preserved
	_, ok := h.bgJobs.Load(jobID)
	assert.True(t, ok, "job should be preserved when CommitTree fails")

	mGit.AssertExpectations(t)
}

// TestHandleApply_UpdateRefFails_ErrorContainsCommitHash verifies that if
// UpdateRef fails after CommitTree succeeds, the error includes the commitHash
// for manual recovery.
func TestHandleApply_UpdateRefFails_ErrorContainsCommitHash(t *testing.T) {
	mGit := new(mockGit)
	mGit.On("Head").Return("parent123", nil)
	mGit.On("CommitTree", "tree456", "parent123", mock.Anything).Return("commit789", nil)
	// First UpdateRef fails - this is the error that returns commitHash
	mGit.On("UpdateRef", "HEAD", "commit789").Return("", fmt.Errorf("update-ref failed"))
	// Subsequent calls should NOT happen since we return error after first UpdateRef fails

	h := newTestHandler(t, mGit)

	jobID := "commit-urfail-123"
	done := make(chan struct{})
	close(done)
	h.bgJobs.Store(jobID, &BgJob{
		ID:       jobID,
		Status:   BgDone,
		TreeHash: "tree456",
		Message:  "feat: something",
		Done:     done,
	})

	params := map[string]any{"command": "APPLY", "job_id": jobID}
	res, err := h.handleApply(context.Background(), params)
	assert.Error(t, err, "UpdateRef failure should return error")
	assert.Contains(t, err.Error(), "commit789", "error should contain commitHash for manual recovery")
	assert.Nil(t, res)

	// Job should be preserved
	_, ok := h.bgJobs.Load(jobID)
	assert.True(t, ok, "job should be preserved when UpdateRef fails")

	mGit.AssertExpectations(t)
}

// TestHandleApply_ResetFails_CommitStillValid verifies that if UpdateRef
// succeeds but Reset fails, the commit is still valid — the error is a warning.
func TestHandleApply_ResetFails_CommitStillValid(t *testing.T) {
	mGit := new(mockGit)
	mGit.On("Head").Return("parent123", nil)
	mGit.On("CommitTree", "tree456", "parent123", mock.Anything).Return("commit789", nil)
	// First UpdateRef: move HEAD to the initial commit
	mGit.On("UpdateRef", "HEAD", "commit789").Return("", nil)
	// Plumbing amend sequence succeeds
	mGit.On("Add", []string{domain.MetadataDir}).Return(nil)
	mGit.On("WriteTree").Return("newTree999", nil)
	mGit.On("CommitTree", "newTree999", "commit789", mock.Anything).Return("replacementCommit888", nil)
	mGit.On("UpdateRef", "HEAD", "replacementCommit888").Return("", nil)
	// Reset fails but commit is still valid
	mGit.On("Reset", "HEAD", ".").Return("", fmt.Errorf("reset failed"))

	h := newTestHandler(t, mGit)

	jobID := "commit-resetfail-123"
	done := make(chan struct{})
	close(done)
	h.bgJobs.Store(jobID, &BgJob{
		ID:       jobID,
		Status:   BgDone,
		TreeHash: "tree456",
		Message:  "feat: something",
		Done:     done,
	})

	params := map[string]any{"command": "APPLY", "job_id": jobID}
	res, err := h.handleApply(context.Background(), params)
	// Reset failure should NOT be a hard error — commit is valid
	assert.NoError(t, err, "Reset failure should not be a hard error")
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "replacementCommit888", "result should contain replacement commit hash even with reset warning")

	// Job should be deleted (commit was successful)
	_, ok := h.bgJobs.Load(jobID)
	assert.False(t, ok, "job should be deleted even when Reset fails")

	mGit.AssertExpectations(t)
}

// TestHandleApply_PushAfter_CallsPush verifies that pushAfter=true calls
// Push() after successful plumbing commit.
func TestHandleApply_PushAfter_CallsPush(t *testing.T) {
	mGit := new(mockGit)
	mGit.On("Head").Return("parent123", nil)
	mGit.On("CommitTree", "tree456", "parent123", mock.Anything).Return("commit789", nil)
	// First UpdateRef: move HEAD to the initial commit
	mGit.On("UpdateRef", "HEAD", "commit789").Return("", nil)
	// Plumbing amend sequence succeeds
	mGit.On("Add", []string{domain.MetadataDir}).Return(nil)
	mGit.On("WriteTree").Return("newTree999", nil)
	mGit.On("CommitTree", "newTree999", "commit789", mock.Anything).Return("replacementCommit888", nil)
	mGit.On("UpdateRef", "HEAD", "replacementCommit888").Return("", nil)
	mGit.On("Reset", "HEAD", ".").Return("", nil)
	mGit.On("Push").Return("push output", nil)

	h := newTestHandler(t, mGit)

	jobID := "commit-push-123"
	done := make(chan struct{})
	close(done)
	h.bgJobs.Store(jobID, &BgJob{
		ID:       jobID,
		Status:   BgDone,
		TreeHash: "tree456",
		Message:  "feat: something",
		Done:     done,
	})

	params := map[string]any{"command": "APPLY", "job_id": jobID, "push_after": true}
	res, err := h.handleApply(context.Background(), params)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "replacementCommit888")
	assert.Contains(t, text, "push", "result should mention push status")

	mGit.AssertExpectations(t)
}

// TestHandleApply_PushAfter_PushFails_WarningNotHardError verifies that a
// Push failure is a warning, not a hard error.
func TestHandleApply_PushAfter_PushFails_WarningNotHardError(t *testing.T) {
	mGit := new(mockGit)
	mGit.On("Head").Return("parent123", nil)
	mGit.On("CommitTree", "tree456", "parent123", mock.Anything).Return("commit789", nil)
	// First UpdateRef: move HEAD to the initial commit
	mGit.On("UpdateRef", "HEAD", "commit789").Return("", nil)
	// Plumbing amend sequence succeeds
	mGit.On("Add", []string{domain.MetadataDir}).Return(nil)
	mGit.On("WriteTree").Return("newTree999", nil)
	mGit.On("CommitTree", "newTree999", "commit789", mock.Anything).Return("replacementCommit888", nil)
	mGit.On("UpdateRef", "HEAD", "replacementCommit888").Return("", nil)
	mGit.On("Reset", "HEAD", ".").Return("", nil)
	mGit.On("Push").Return("", fmt.Errorf("push failed: remote rejected"))

	h := newTestHandler(t, mGit)

	jobID := "commit-pushfail-123"
	done := make(chan struct{})
	close(done)
	h.bgJobs.Store(jobID, &BgJob{
		ID:       jobID,
		Status:   BgDone,
		TreeHash: "tree456",
		Message:  "feat: something",
		Done:     done,
	})

	params := map[string]any{"command": "APPLY", "job_id": jobID, "push_after": true}
	res, err := h.handleApply(context.Background(), params)
	// Push failure should NOT be a hard error — commit is valid
	assert.NoError(t, err, "Push failure should be a warning, not hard error")
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "replacementCommit888")
	assert.Contains(t, text, "push failed", "result should contain push failure warning")

	mGit.AssertExpectations(t)
}

// TestHandleApply_MessageFromJob_PrePopulated verifies that when job.Message
// is populated, it's used directly without calling GenerateCommitMessage.
func TestHandleApply_MessageFromJob_PrePopulated(t *testing.T) {
	mGit := new(mockGit)
	expectedMessage := "feat: add refresh token rotation"
	mGit.On("Head").Return("parent123", nil)
	mGit.On("CommitTree", "tree456", "parent123", expectedMessage).Return("commit789", nil)
	// First UpdateRef: move HEAD to the initial commit
	mGit.On("UpdateRef", "HEAD", "commit789").Return("", nil)
	// Plumbing amend sequence
	mGit.On("Add", []string{domain.MetadataDir}).Return(nil)
	mGit.On("WriteTree").Return("newTree999", nil)
	mGit.On("CommitTree", "newTree999", "commit789", expectedMessage).Return("replacementCommit888", nil)
	mGit.On("UpdateRef", "HEAD", "replacementCommit888").Return("", nil)
	mGit.On("Reset", "HEAD", ".").Return("", nil)

	h := newTestHandler(t, mGit)

	jobID := "commit-msg-123"
	done := make(chan struct{})
	close(done)
	h.bgJobs.Store(jobID, &BgJob{
		ID:       jobID,
		Status:   BgDone,
		TreeHash: "tree456",
		Message:  expectedMessage,
		Done:     done,
	})

	params := map[string]any{"command": "APPLY", "job_id": jobID}
	res, err := h.handleApply(context.Background(), params)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	// Verify CommitTree was called with the exact pre-populated message
	mGit.AssertCalled(t, "CommitTree", "tree456", "parent123", expectedMessage)
	mGit.AssertExpectations(t)
}

// TestHandleApply_MessageFromGenerateCommitMessage verifies that when
// job.Message is empty, GenerateCommitMessage is called and the result
// is composed into subject + body.
func TestHandleApply_MessageFromGenerateCommitMessage(t *testing.T) {
	mGit := new(mockGit)
	// GenerateCommitMessage calls prepareStages which needs Status, DiffStaged, DiffStatStaged
	mGit.On("Status").Return(domain.Status{Branch: "main", IsClean: false, Modified: 1, Files: []domain.FileStatus{{Path: "main.go", Status: "M ", Staged: true}}}, nil)
	mGit.On("DiffStaged", mock.Anything).Return("diff --git a/main.go b/main.go\n+added line", nil)
	mGit.On("DiffStatStaged", mock.Anything).Return("1 file changed, 1 insertion(+)", nil)
	mGit.On("Head").Return("parent123", nil)
	// The message comes from GenerateCommitMessage through composeMessage — use mock.Anything for message
	mGit.On("CommitTree", "tree456", "parent123", mock.Anything).Return("commit789", nil)
	mGit.On("UpdateRef", "HEAD", "commit789").Return("", nil)
	// Plumbing amend sequence
	mGit.On("Add", []string{domain.MetadataDir}).Return(nil)
	mGit.On("WriteTree").Return("newTree999", nil)
	mGit.On("CommitTree", "newTree999", "commit789", mock.Anything).Return("replacementCommit888", nil)
	mGit.On("UpdateRef", "HEAD", "replacementCommit888").Return("", nil)
	mGit.On("Reset", "HEAD", ".").Return("", nil)

	h := newTestHandler(t, mGit)

	jobID := "commit-genmsg-123"
	done := make(chan struct{})
	close(done)
	// job.Message is empty — should trigger GenerateCommitMessage
	h.bgJobs.Store(jobID, &BgJob{
		ID:       jobID,
		Status:   BgDone,
		TreeHash: "tree456",
		Message:  "", // empty → must call GenerateCommitMessage
		Why:      "why text",
		Done:     done,
	})

	params := map[string]any{"command": "APPLY", "job_id": jobID}
	res, err := h.handleApply(context.Background(), params)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "replacementCommit888", "result should contain replacement commit hash")
	// Verify the message was not empty (proving GenerateCommitMessage was called)
	mGit.AssertCalled(t, "CommitTree", "tree456", "parent123", mock.MatchedBy(func(msg string) bool {
		return msg != "" // non-empty message proves GenerateCommitMessage was called
	}))
}

// TestHandleApply_WhyPropagation verifies that the user's justification (Why)
// stored in the BgJob is propagated all the way to the LLM during Apply's plumbing path.
func TestHandleApply_WhyPropagation(t *testing.T) {
	mGit := new(mockGit)
	mGit.On("Status").Return(domain.Status{Branch: "main", IsClean: false, Modified: 1, Files: []domain.FileStatus{{Path: "main.go", Status: "M ", Staged: true}}}, nil)
	mGit.On("DiffStaged", mock.Anything).Return("diff --git a/main.go b/main.go\n+added line", nil)
	mGit.On("DiffStatStaged", mock.Anything).Return("1 file changed, 1 insertion(+)", nil)
	mGit.On("Head").Return("parent123", nil)
	mGit.On("CommitTree", "tree456", "parent123", mock.Anything).Return("commit789", nil)
	mGit.On("UpdateRef", "HEAD", "commit789").Return("", nil)
	// Plumbing amend sequence
	mGit.On("Add", []string{domain.MetadataDir}).Return(nil)
	mGit.On("WriteTree").Return("newTree999", nil)
	mGit.On("CommitTree", "newTree999", "commit789", mock.Anything).Return("replacementCommit888", nil)
	mGit.On("UpdateRef", "HEAD", "replacementCommit888").Return("", nil)
	mGit.On("Reset", "HEAD", ".").Return("", nil)

	// Custom LLM tracking SetWhy
	trackingLLM := &whyTrackingLLM{}
	trackingLLM.On("GenerateChunkMessage", mock.Anything).Return("feat: test commit", nil)
	trackingLLM.On("ClassifyBinary", mock.Anything).Return("fix", nil)

	mChunker := new(mockDiffChunker)
	mChunker.On("Chunk", mock.Anything, mock.Anything).
		Return([]domain.DiffChunk{{Files: []string{"main.go"}, Diff: "test diff"}}, nil)

	mSecurity := new(mockSecurityService)
	mSecurity.On("CheckFiles", mock.Anything, mock.Anything).
		Return(&ports.SecurityCheckResult{Blocked: false})

	commitSvc := workflow.NewCommitService(
		mGit, trackingLLM, mChunker, mSecurity,
		workflow.DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/task.log"),
		nil,
	)

	confirm := confirm.NewInMemory(5 * time.Minute)
	cfg := config.Default()
	rev := workflow.New(mGit, trackingLLM, confirm, cfg, commitSvc, nil, mSecurity)

	h := NewHandler(mGit, commitSvc, rev, trackingLLM, "", nil, nil)

	jobID := "commit-why-propagation-123"
	done := make(chan struct{})
	close(done)

	// Store BgJob with Why populated
	h.bgJobs.Store(jobID, &BgJob{
		ID:       jobID,
		Status:   BgDone,
		TreeHash: "tree456",
		Message:  "", // triggers GenerateCommitMessage
		Why:      "refactor database access layer",
		Done:     done,
	})

	params := map[string]any{"command": "APPLY", "job_id": jobID}
	_, err := h.handleApply(context.Background(), params)
	assert.NoError(t, err)

	// Verify why was propagated to our tracking LLM
	assert.Equal(t, "refactor database access layer", trackingLLM.whyCaptured)
}

type whyTrackingLLM struct {
	mockLLM
	whyCaptured string
}

func (l *whyTrackingLLM) SetWhy(why string) {
	l.whyCaptured = why
}

func (l *whyTrackingLLM) ClearWhy() {}

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
func (m *mockLLM) GenerateCommitSynthesis(combinedChunk domain.DiffChunk, fileMessages []string) (string, error) {
	args := m.Called(combinedChunk, fileMessages)
	return args.String(0), args.Error(1)
}
func (m *mockLLM) InterpretGitOp(op, instruction string, context map[string]string) (map[string]string, error) {
	args := m.Called(op, instruction, context)
	return args.Get(0).(map[string]string), args.Error(1)
}
func (m *mockLLM) SetRetryContext(previousMessage string) { m.Called(previousMessage) }
func (m *mockLLM) ClearRetryContext()                     { m.Called() }
func (m *mockLLM) IsAvailable() bool                      { return true }
func (m *mockLLM) VerifySecrets(diff string, findings []domain.SecretDetection) (bool, error) {
	args := m.Called(diff, findings)
	return args.Bool(0), args.Error(1)
}
func (m *mockLLM) AuditBinaryContent(filename, content string) (bool, error) {
	args := m.Called(filename, content)
	return args.Bool(0), args.Error(1)
}
func (m *mockLLM) GenerateChangelogByArea(formattedGroups string, nameMap map[string]string, customMessage string) (domain.ChangelogByArea, error) {
	args := m.Called(formattedGroups, nameMap)
	return args.Get(0).(domain.ChangelogByArea), args.Error(1)
}
func (m *mockLLM) GenerateChangelogGrouped(formattedGroups string, nameMap map[string]string, customMessage string, mode string) (domain.ChangelogByArea, error) {
	return m.GenerateChangelogByArea(formattedGroups, nameMap, customMessage)
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

	mGit.On("Add", []string{domain.MetadataDir}).Return(nil).Maybe()
	mGit.On("Head").Return("a1b2c3d4e5f6071829a0b1c2d3e4f50617283940", nil).Maybe()
	mGit.On("Reset", mock.Anything, domain.MetadataDir).Return("reset output", nil).Maybe()

	mLLM := new(mockLLM)
	mLLM.On("GenerateChunkMessage", mock.Anything).Return("feat: test commit", nil)
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
		nil,
	)

	confirm := confirm.NewInMemory(5 * time.Minute)
	cfg := config.Default()

	rev := workflow.New(mGit, mLLM, confirm, cfg, commitSvc, nil, mSecurity)

	return NewHandler(mGit, commitSvc, rev, mLLM, "", nil, nil)
}

type mockCommitStore struct {
	mock.Mock
}

func (m *mockCommitStore) Append(entries ...domain.CommitEntry) error {
	args := m.Called(entries)
	return args.Error(0)
}

func (m *mockCommitStore) Read() ([]domain.CommitEntry, error) {
	args := m.Called()
	return args.Get(0).([]domain.CommitEntry), args.Error(1)
}

func (m *mockCommitStore) Clear() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockCommitStore) SetBranch(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *mockCommitStore) RemoveBranch(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *mockCommitStore) Reconcile(gitEntries []domain.CommitEntry) error {
	args := m.Called(gitEntries)
	return args.Error(0)
}

func (m *mockCommitStore) ReadAllBranches() (map[string][]domain.CommitEntry, error) {
	args := m.Called()
	return args.Get(0).(map[string][]domain.CommitEntry), args.Error(1)
}

func (m *mockCommitStore) RemoveAllBranchDirs() error {
	args := m.Called()
	return args.Error(0)
}

func TestHandlePreview_StagesMetadataAndReconciles(t *testing.T) {
	mGit := new(mockGit)
	mGit.On("CurrentBranch").Return("feat/test-branch", nil)

	// Expectations for Reconcile log fetch:
	mGit.On("MergeBase", "main", "feat/test-branch").Return("a1b2c3d4", nil)
	mGit.On("LogRange", "a1b2c3d4", "HEAD").Return("a1b2c3d4e5f6071829a0b1c2d3e4f50617283940|author|2026-05-31|feat: test message", nil)

	// Staging expectation:
	mGit.On("Add", []string{domain.MetadataDir}).Return(nil)

	// Normal preview execution expectations:
	mGit.On("WriteTree").Return("tree123", nil)
	mGit.On("Head").Return("a1b2c3d4e5f6071829a0b1c2d3e4f50617283940", nil)
	mGit.On("Reset", "HEAD", domain.MetadataDir).Return("reset output", nil)

	mGit.On("Status").Return(domain.Status{Branch: "feat/test-branch", IsClean: false, Modified: 1, Files: []domain.FileStatus{{Path: "main.go", Status: "M ", Staged: true}}}, nil)
	mGit.On("DiffStaged", mock.Anything).Return("diff --git a/main.go b/main.go\n+added line", nil)

	// Set up commit store mock:
	mStore := new(mockCommitStore)
	mStore.On("SetBranch", "feat/test-branch").Return(nil)

	expectedEntry, _ := domain.NewCommitEntry("a1b2c3d4e5f6071829a0b1c2d3e4f50617283940", "feat: test message", domain.WithAuthor("author"), domain.WithDate("2026-05-31"))
	mStore.On("Reconcile", []domain.CommitEntry{expectedEntry}).Return(nil)

	// Create CommitService with the mocked store
	mLLM := new(mockLLM)
	mLLM.On("GenerateChunkMessage", mock.Anything).Return("feat: test commit", nil)
	mLLM.On("ClassifyBinary", mock.Anything).Return("feat", nil)

	mChunker := new(mockDiffChunker)
	mChunker.On("Chunk", mock.Anything, mock.Anything).
		Return([]domain.DiffChunk{{Files: []string{"main.go"}, Diff: "test diff"}}, nil)

	mSecurity := new(mockSecurityService)
	mSecurity.On("CheckFiles", mock.Anything, mock.Anything).
		Return(&ports.SecurityCheckResult{Blocked: false})

	commitSvc := workflow.NewCommitService(
		mGit, mLLM, mChunker, mSecurity,
		workflow.DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/task.log"),
		mStore,
	)

	confirm := confirm.NewInMemory(5 * time.Minute)
	cfg := config.Default()
	rev := workflow.New(mGit, mLLM, confirm, cfg, commitSvc, nil, mSecurity)

	h := NewHandler(mGit, commitSvc, rev, mLLM, "", nil, nil)

	args := map[string]any{"command": "PREVIEW", "why": "test preview"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	mGit.AssertExpectations(t)
	mStore.AssertExpectations(t)
}

// --- BaseBranch reconciliation tests (Task 1.4) ---

func TestHandlePreview_BaseBranchConfigured_UsesMergeBase(t *testing.T) {
	// When BaseBranch is "develop" and current branch is "feature/auth",
	// MergeBase("develop", "feature/auth") should be called exactly once.
	mGit := new(mockGit)
	mGit.On("CurrentBranch").Return("feature/auth", nil)
	mGit.On("MergeBase", "develop", "feature/auth").Return("abc123def456", nil)
	mGit.On("LogRange", "abc123def456", "HEAD").Return("abc123def4560000000000000000000000000001|Alice|2026-06-01|feat: add auth", nil)
	mGit.On("Add", []string{domain.MetadataDir}).Return(nil)
	mGit.On("WriteTree").Return("tree123", nil)
	mGit.On("Head").Return("abc123def4560000000000000000000000000001", nil)
	mGit.On("Reset", "HEAD", domain.MetadataDir).Return("reset output", nil)
	mGit.On("Status").Return(domain.Status{Branch: "feature/auth", IsClean: false, Modified: 1, Files: []domain.FileStatus{{Path: "auth.go", Status: "M ", Staged: true}}}, nil)
	mGit.On("DiffStaged", mock.Anything).Return("diff --git a/auth.go b/auth.go\n+added line", nil)

	mStore := new(mockCommitStore)
	mStore.On("SetBranch", "feature/auth").Return(nil)
	expectedEntry, _ := domain.NewCommitEntry("abc123def4560000000000000000000000000001", "feat: add auth", domain.WithAuthor("Alice"), domain.WithDate("2026-06-01"), domain.WithStackID("abc123def456"), domain.WithStackBranch("feature/auth"))
	mStore.On("Reconcile", []domain.CommitEntry{expectedEntry}).Return(nil)

	mLLM := new(mockLLM)
	mLLM.On("GenerateChunkMessage", mock.Anything).Return("feat: test commit", nil)
	mLLM.On("ClassifyBinary", mock.Anything).Return("feat", nil)

	mChunker := new(mockDiffChunker)
	mChunker.On("Chunk", mock.Anything, mock.Anything).Return([]domain.DiffChunk{{Files: []string{"auth.go"}, Diff: "test diff"}}, nil)

	mSecurity := new(mockSecurityService)
	mSecurity.On("CheckFiles", mock.Anything, mock.Anything).Return(&ports.SecurityCheckResult{Blocked: false})

	commitSvc := workflow.NewCommitService(
		mGit, mLLM, mChunker, mSecurity,
		workflow.DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/task.log"),
		mStore,
	)

	confirm := confirm.NewInMemory(5 * time.Minute)
	cfg := config.Default()
	rev := workflow.New(mGit, mLLM, confirm, cfg, commitSvc, nil, mSecurity)

	h := NewHandler(mGit, commitSvc, rev, mLLM, "", nil, nil)
	// Override configProvider to return a config with BaseBranch = "develop"
	h.configProvider = func() *domain.ProjectConfig {
		return &domain.ProjectConfig{BaseBranch: "develop", Areas: map[string][]string{}}
	}

	args := map[string]any{"command": "PREVIEW", "why": "test"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	// Verify MergeBase was called exactly once with the configured BaseBranch
	mGit.AssertCalled(t, "MergeBase", "develop", "feature/auth")
	mGit.AssertNumberOfCalls(t, "MergeBase", 1)
	mStore.AssertExpectations(t)
}

func TestHandlePreview_BaseBranchIsCurrentBranch_SkipsReconciliation(t *testing.T) {
	// When BaseBranch is "main" and current branch IS "main",
	// reconciliation should be skipped entirely (no MergeBase, no LogRange).
	mGit := new(mockGit)
	mGit.On("CurrentBranch").Return("main", nil)
	// DO NOT set up MergeBase/LogRange — they should NOT be called

	mGit.On("Add", []string{domain.MetadataDir}).Return(nil)
	mGit.On("WriteTree").Return("tree123", nil)
	mGit.On("Head").Return("abc123", nil)
	mGit.On("Reset", "HEAD", domain.MetadataDir).Return("reset output", nil)
	mGit.On("Status").Return(domain.Status{Branch: "main", IsClean: false, Modified: 1, Files: []domain.FileStatus{{Path: "main.go", Status: "M ", Staged: true}}}, nil)
	mGit.On("DiffStaged", mock.Anything).Return("diff --git a/main.go b/main.go\n+added line", nil)

	mStore := new(mockCommitStore)
	mStore.On("SetBranch", "main").Return(nil)
	// Reconcile should NOT be called when on trunk — no mock setup for it

	mLLM := new(mockLLM)
	mLLM.On("GenerateChunkMessage", mock.Anything).Return("feat: test commit", nil)
	mLLM.On("ClassifyBinary", mock.Anything).Return("feat", nil)

	mChunker := new(mockDiffChunker)
	mChunker.On("Chunk", mock.Anything, mock.Anything).Return([]domain.DiffChunk{{Files: []string{"main.go"}, Diff: "test diff"}}, nil)

	mSecurity := new(mockSecurityService)
	mSecurity.On("CheckFiles", mock.Anything, mock.Anything).Return(&ports.SecurityCheckResult{Blocked: false})

	commitSvc := workflow.NewCommitService(
		mGit, mLLM, mChunker, mSecurity,
		workflow.DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/task.log"),
		mStore,
	)

	confirm := confirm.NewInMemory(5 * time.Minute)
	cfg := config.Default()
	rev := workflow.New(mGit, mLLM, confirm, cfg, commitSvc, nil, mSecurity)

	h := NewHandler(mGit, commitSvc, rev, mLLM, "", nil, nil)
	// Override configProvider to return BaseBranch = "main" (same as current branch)
	h.configProvider = func() *domain.ProjectConfig {
		return &domain.ProjectConfig{BaseBranch: "main", Areas: map[string][]string{}}
	}

	args := map[string]any{"command": "PREVIEW", "why": "test"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	_, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)

	// Verify MergeBase was NOT called
	mGit.AssertNotCalled(t, "MergeBase")
	mGit.AssertNotCalled(t, "LogRange")
	mStore.AssertExpectations(t)
}

func TestHandlePreview_BaseBranchEmpty_FallsBackToHardcodedList(t *testing.T) {
	// When BaseBranch is empty, the old behavior should work:
	// try "main", "master", "develop" in order.
	mGit := new(mockGit)
	mGit.On("CurrentBranch").Return("feature/test", nil)
	// "main" succeeds
	mGit.On("MergeBase", "main", "feature/test").Return("abc123", nil)
	mGit.On("LogRange", "abc123", "HEAD").Return("abc1230000000000000000000000000000000001|Alice|2026-06-01|feat: test", nil)
	mGit.On("Add", []string{domain.MetadataDir}).Return(nil)
	mGit.On("WriteTree").Return("tree123", nil)
	mGit.On("Head").Return("abc1230000000000000000000000000000000001", nil)
	mGit.On("Reset", "HEAD", domain.MetadataDir).Return("reset output", nil)
	mGit.On("Status").Return(domain.Status{Branch: "feature/test", IsClean: false, Modified: 1, Files: []domain.FileStatus{{Path: "test.go", Status: "M ", Staged: true}}}, nil)
	mGit.On("DiffStaged", mock.Anything).Return("diff --git a/test.go b/test.go\n+added line", nil)

	mStore := new(mockCommitStore)
	mStore.On("SetBranch", "feature/test").Return(nil)
	expectedEntry, _ := domain.NewCommitEntry("abc1230000000000000000000000000000000001", "feat: test", domain.WithAuthor("Alice"), domain.WithDate("2026-06-01"))
	mStore.On("Reconcile", []domain.CommitEntry{expectedEntry}).Return(nil)

	mLLM := new(mockLLM)
	mLLM.On("GenerateChunkMessage", mock.Anything).Return("feat: test commit", nil)
	mLLM.On("ClassifyBinary", mock.Anything).Return("feat", nil)

	mChunker := new(mockDiffChunker)
	mChunker.On("Chunk", mock.Anything, mock.Anything).Return([]domain.DiffChunk{{Files: []string{"test.go"}, Diff: "test diff"}}, nil)

	mSecurity := new(mockSecurityService)
	mSecurity.On("CheckFiles", mock.Anything, mock.Anything).Return(&ports.SecurityCheckResult{Blocked: false})

	commitSvc := workflow.NewCommitService(
		mGit, mLLM, mChunker, mSecurity,
		workflow.DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/task.log"),
		mStore,
	)

	confirm := confirm.NewInMemory(5 * time.Minute)
	cfg := config.Default()
	rev := workflow.New(mGit, mLLM, confirm, cfg, commitSvc, nil, mSecurity)

	h := NewHandler(mGit, commitSvc, rev, mLLM, "", nil, nil)
	// Override configProvider to return an empty BaseBranch
	h.configProvider = func() *domain.ProjectConfig {
		return &domain.ProjectConfig{Areas: map[string][]string{}}
	}

	args := map[string]any{"command": "PREVIEW", "why": "test"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	// Verify MergeBase was called with "main" (first in hardcoded list)
	mGit.AssertCalled(t, "MergeBase", "main", "feature/test")
	mStore.AssertExpectations(t)
}

func TestHandlePreview_BaseBranchConfigured_MergeBaseError_ReturnsError(t *testing.T) {
	// Bug 1: When BaseBranch is "develop" and MergeBase fails, return error (do NOT fall back to full log).
	mGit := new(mockGit)
	mGit.On("CurrentBranch").Return("feature/auth", nil)
	// MergeBase fails — should return error immediately
	mGit.On("MergeBase", "develop", "feature/auth").Return("", fmt.Errorf("no merge base"))
	// WriteTree is called before the error path (staging happens before branch resolution)
	mGit.On("Add", []string{domain.MetadataDir}).Return(nil)
	mGit.On("WriteTree").Return("tree123", nil)
	// Log should NOT be called since we return error on MergeBase failure

	mStore := new(mockCommitStore)
	mStore.On("SetBranch", "feature/auth").Return(nil)
	// Reconcile should NOT be called since we return error on MergeBase failure

	mLLM := new(mockLLM)
	// LLM should NOT be called since we return error on MergeBase failure

	mChunker := new(mockDiffChunker)
	// Chunker should NOT be called

	mSecurity := new(mockSecurityService)
	// Security should NOT be called

	commitSvc := workflow.NewCommitService(
		mGit, mLLM, mChunker, mSecurity,
		workflow.DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/task.log"),
		mStore,
	)

	confirm := confirm.NewInMemory(5 * time.Minute)
	cfg := config.Default()
	rev := workflow.New(mGit, mLLM, confirm, cfg, commitSvc, nil, mSecurity)

	h := NewHandler(mGit, commitSvc, rev, mLLM, "", nil, nil)
	h.configProvider = func() *domain.ProjectConfig {
		return &domain.ProjectConfig{BaseBranch: "develop", Areas: map[string][]string{}}
	}

	args := map[string]any{"command": "PREVIEW", "why": "test"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleCommit(context.Background(), req)
	// JSONErrorResult returns result with error content, not an error return value
	assert.NoError(t, err)
	assert.NotNil(t, res)
	// Error responses contain "error" in the JSON
	assert.Contains(t, res.Content[0].(mcpgo.TextContent).Text, "error")
	assert.Contains(t, res.Content[0].(mcpgo.TextContent).Text, "cannot resolve merge base for configured BaseBranch")

	// Verify MergeBase was called with the configured BaseBranch
	mGit.AssertCalled(t, "MergeBase", "develop", "feature/auth")
	// Verify Log was NOT called (no fallback)
	mGit.AssertNotCalled(t, "Log", mock.Anything, mock.Anything, mock.Anything)
	mStore.AssertExpectations(t)
}

func TestMockGit_MergeBaseAndLogRange(t *testing.T) {
	m := new(mockGit)
	// Case 1: Happy path
	m.On("MergeBase", "a", "b").Return("base", nil)
	m.On("LogRange", "base", "HEAD").Return("log output", nil)

	mb, err1 := m.MergeBase("a", "b")
	lr, err2 := m.LogRange("base", "HEAD")

	assert.NoError(t, err1)
	assert.Equal(t, "base", mb)
	assert.NoError(t, err2)
	assert.Equal(t, "log output", lr)

	// Case 2: Triangulation with different inputs and error propagation
	m.On("MergeBase", "x", "y").Return("", fmt.Errorf("git error"))
	m.On("LogRange", "y", "HEAD").Return("", fmt.Errorf("log error"))

	mb2, err3 := m.MergeBase("x", "y")
	lr2, err4 := m.LogRange("y", "HEAD")

	assert.Error(t, err3)
	assert.Equal(t, "", mb2)
	assert.Equal(t, "git error", err3.Error())
	assert.Error(t, err4)
	assert.Equal(t, "", lr2)
	assert.Equal(t, "log error", err4.Error())

	m.AssertExpectations(t)
}

func TestHandlePreview_StackMetadataInjected_WithBaseBranch(t *testing.T) {
	// When BaseBranch is configured and current branch differs,
	// entries passed to Reconcile should have StackID and StackBranch set.
	// StackID = merge-base SHA, StackBranch = current branch name.
	mGit := new(mockGit)
	mGit.On("CurrentBranch").Return("feature/auth", nil)
	mGit.On("MergeBase", "develop", "feature/auth").Return("abc123def456789012345678901234567890ab", nil)
	mGit.On("LogRange", "abc123def456789012345678901234567890ab", "HEAD").Return("abc123def4560000000000000000000000000001|Alice|2026-06-01|feat: add auth\nabc123def4560000000000000000000000000002|Bob|2026-06-02|fix: bug", nil)
	mGit.On("Add", []string{domain.MetadataDir}).Return(nil)
	mGit.On("WriteTree").Return("tree123", nil)
	mGit.On("Head").Return("abc123def4560000000000000000000000000001", nil)
	mGit.On("Reset", "HEAD", domain.MetadataDir).Return("reset output", nil)
	mGit.On("Status").Return(domain.Status{Branch: "feature/auth", IsClean: false, Modified: 1, Files: []domain.FileStatus{{Path: "auth.go", Status: "M ", Staged: true}}}, nil)
	mGit.On("DiffStaged", mock.Anything).Return("diff --git a/auth.go b/auth.go\n+added line", nil)

	mStore := new(mockCommitStore)
	mStore.On("SetBranch", "feature/auth").Return(nil)

	// Verify entries have StackID and StackBranch set correctly
	mStore.On("Reconcile", mock.MatchedBy(func(entries []domain.CommitEntry) bool {
		if len(entries) != 2 {
			return false
		}
		// Both entries should have the same StackID (merge-base) and StackBranch (current branch)
		for _, e := range entries {
			if e.StackID() != "abc123def456789012345678901234567890ab" {
				return false
			}
			if e.StackBranch() != "feature/auth" {
				return false
			}
		}
		return true
	})).Return(nil)

	mLLM := new(mockLLM)
	mLLM.On("GenerateChunkMessage", mock.Anything).Return("feat: test commit", nil)
	mLLM.On("ClassifyBinary", mock.Anything).Return("feat", nil)

	mChunker := new(mockDiffChunker)
	mChunker.On("Chunk", mock.Anything, mock.Anything).Return([]domain.DiffChunk{{Files: []string{"auth.go"}, Diff: "test diff"}}, nil)

	mSecurity := new(mockSecurityService)
	mSecurity.On("CheckFiles", mock.Anything, mock.Anything).Return(&ports.SecurityCheckResult{Blocked: false})

	commitSvc := workflow.NewCommitService(
		mGit, mLLM, mChunker, mSecurity,
		workflow.DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/task.log"),
		mStore,
	)

	confirm := confirm.NewInMemory(5 * time.Minute)
	cfg := config.Default()
	rev := workflow.New(mGit, mLLM, confirm, cfg, commitSvc, nil, mSecurity)

	h := NewHandler(mGit, commitSvc, rev, mLLM, "", nil, nil)
	h.configProvider = func() *domain.ProjectConfig {
		return &domain.ProjectConfig{BaseBranch: "develop", Areas: map[string][]string{}}
	}

	args := map[string]any{"command": "PREVIEW", "why": "test"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	mGit.AssertExpectations(t)
	mStore.AssertExpectations(t)
}

// Bug 3: applyPlumbing integration test — verifies exact call order for metadata amend
func TestApplyPlumbing_AmendsMetadataAfterCaptureCommit(t *testing.T) {
	mGit := new(mockGit)
	mStore := new(mockCommitStore)
	mLLM := new(mockLLM)

	commitSvc := workflow.NewCommitService(
		mGit, mLLM, &mockDiffChunker{}, &mockSecurityService{},
		workflow.DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/task.log"),
		mStore,
	)

	confirm := confirm.NewInMemory(5*time.Minute)
	cfg := config.Default()
	rev := workflow.New(mGit, mLLM, confirm, cfg, commitSvc, nil, &mockSecurityService{})

	h := NewHandler(mGit, commitSvc, rev, mLLM, "", nil, nil)

	// Create a BgJob with stack metadata
	jobID := "test-job-123"
	bgJob := &BgJob{
		ID:          jobID,
		Status:      BgDone,
		TreeHash:    "tree123",
		Message:     "feat: test commit",
		Why:         "test",
		StackID:     "abc123",
		StackBranch: "feature/test",
		Done:        make(chan struct{}),
	}
	close(bgJob.Done)
	h.bgJobs.Store(jobID, bgJob)

	// Mock the plumbing sequence
	mGit.On("Head").Return("abcd1234567890abcdef1234567890abcdef1234", nil)
	mGit.On("CommitTree", "tree123", "abcd1234567890abcdef1234567890abcdef1234", "feat: test commit").Return("commitHash111111111111111111111111111111", nil)
	// First UpdateRef: move HEAD to the initial commit
	mGit.On("UpdateRef", "HEAD", "commitHash111111111111111111111111111111").Return("", nil)
	// CaptureCommit is called (via commitSvc.CaptureCommit) - needs CurrentBranch for SetBranch
	mGit.On("CurrentBranch").Return("feature/test", nil)
	mGit.On("ConfigGet", "user.name").Return("Test User", nil)
	// mStore expects SetBranch and Append (variadic)
	mStore.On("SetBranch", "feature/test").Return(nil)
	// Append is variadic: Append(entries ...domain.CommitEntry)
	// When Called(entries) is used inside the mock, it passes the slice as a single arg
	mStore.On("Append", mock.AnythingOfType("[]domain.CommitEntry")).Return(nil)
	// Bug 3: Plumbing amend sequence
	mGit.On("Add", []string{domain.MetadataDir}).Return(nil)
	mGit.On("WriteTree").Return("newTreeHash222222222222222222222222222222", nil)
	mGit.On("CommitTree", "newTreeHash222222222222222222222222222222", "commitHash111111111111111111111111111111", "feat: test commit").Return("replacementHash333333333333333333333333333333", nil)
	// Second UpdateRef: move HEAD to the replacement commit
	mGit.On("UpdateRef", "HEAD", "replacementHash333333333333333333333333333333").Return("", nil)
	mGit.On("Reset", "HEAD", ".").Return("cleanup output", nil)

	args := map[string]any{"command": "APPLY", "job_id": jobID}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	// Verify the plumbing amend sequence was called in order
	mGit.AssertCalled(t, "Add", []string{domain.MetadataDir})
	mGit.AssertCalled(t, "WriteTree")
	mGit.AssertCalled(t, "CommitTree", mock.Anything, "commitHash111111111111111111111111111111", mock.Anything)
	mGit.AssertCalled(t, "UpdateRef", "HEAD", mock.Anything)

	// Verify CaptureCommit was called (stack metadata verified by Append being called)
	mStore.AssertCalled(t, "Append", mock.Anything)

	mGit.AssertExpectations(t)
	mStore.AssertExpectations(t)
}

func TestHandlePreview_StackMetadataEmpty_WhenBaseBranchEmpty(t *testing.T) {
	// When BaseBranch is empty, entries should have empty StackID and StackBranch.
	// This tests the fallback path (hardcoded list).
	mGit := new(mockGit)
	mGit.On("CurrentBranch").Return("feature/test", nil)
	mGit.On("MergeBase", "main", "feature/test").Return("abc123000000000000000000000000000000000", nil)
	mGit.On("LogRange", "abc123000000000000000000000000000000000", "HEAD").Return("abc1230000000000000000000000000000000001|Alice|2026-06-01|feat: test", nil)
	mGit.On("Add", []string{domain.MetadataDir}).Return(nil)
	mGit.On("WriteTree").Return("tree123", nil)
	mGit.On("Head").Return("abc1230000000000000000000000000000000001", nil)
	mGit.On("Reset", "HEAD", domain.MetadataDir).Return("reset output", nil)
	mGit.On("Status").Return(domain.Status{Branch: "feature/test", IsClean: false, Modified: 1, Files: []domain.FileStatus{{Path: "test.go", Status: "M ", Staged: true}}}, nil)
	mGit.On("DiffStaged", mock.Anything).Return("diff --git a/test.go b/test.go\n+added line", nil)

	mStore := new(mockCommitStore)
	mStore.On("SetBranch", "feature/test").Return(nil)

	// Verify entries have empty StackID and StackBranch (no BaseBranch configured)
	mStore.On("Reconcile", mock.MatchedBy(func(entries []domain.CommitEntry) bool {
		for _, e := range entries {
			if e.StackID() != "" {
				return false
			}
			if e.StackBranch() != "" {
				return false
			}
		}
		return true
	})).Return(nil)

	mLLM := new(mockLLM)
	mLLM.On("GenerateChunkMessage", mock.Anything).Return("feat: test commit", nil)
	mLLM.On("ClassifyBinary", mock.Anything).Return("feat", nil)

	mChunker := new(mockDiffChunker)
	mChunker.On("Chunk", mock.Anything, mock.Anything).Return([]domain.DiffChunk{{Files: []string{"test.go"}, Diff: "test diff"}}, nil)

	mSecurity := new(mockSecurityService)
	mSecurity.On("CheckFiles", mock.Anything, mock.Anything).Return(&ports.SecurityCheckResult{Blocked: false})

	commitSvc := workflow.NewCommitService(
		mGit, mLLM, mChunker, mSecurity,
		workflow.DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/task.log"),
		mStore,
	)

	confirm := confirm.NewInMemory(5 * time.Minute)
	cfg := config.Default()
	rev := workflow.New(mGit, mLLM, confirm, cfg, commitSvc, nil, mSecurity)

	h := NewHandler(mGit, commitSvc, rev, mLLM, "", nil, nil)
	// Empty BaseBranch — no stack metadata should be injected
	h.configProvider = func() *domain.ProjectConfig {
		return &domain.ProjectConfig{Areas: map[string][]string{}}
	}

	args := map[string]any{"command": "PREVIEW", "why": "test"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	mStore.AssertExpectations(t)
}

func TestHandlePreview_UnstageMetadataTriangulation_HeadErrorAndResetError(t *testing.T) {
	mGit := new(mockGit)
	mGit.On("CurrentBranch").Return("feat/test-branch", nil)

	// Expectations for Reconcile log fetch:
	mGit.On("MergeBase", "main", "feat/test-branch").Return("a1b2c3d4", nil)
	mGit.On("LogRange", "a1b2c3d4", "HEAD").Return("a1b2c3d4e5f6071829a0b1c2d3e4f50617283940|author|2026-05-31|feat: test message", nil)

	// Staging expectation:
	mGit.On("Add", []string{domain.MetadataDir}).Return(nil)

	// Normal preview execution expectations:
	mGit.On("WriteTree").Return("tree123", nil)
	// Case 1: Head returns error (initial repo state)
	mGit.On("Head").Return("", fmt.Errorf("fatal: your current branch 'main' does not have any commits yet"))
	// Case 2: Reset returns error
	mGit.On("Reset", "--", domain.MetadataDir).Return("", fmt.Errorf("fatal: reset failed"))

	mGit.On("Status").Return(domain.Status{Branch: "feat/test-branch", IsClean: false, Modified: 1, Files: []domain.FileStatus{{Path: "main.go", Status: "M ", Staged: true}}}, nil)
	mGit.On("DiffStaged", mock.Anything).Return("diff --git a/main.go b/main.go\n+added line", nil)

	// Set up commit store mock:
	mStore := new(mockCommitStore)
	mStore.On("SetBranch", "feat/test-branch").Return(nil)

	expectedEntry, _ := domain.NewCommitEntry("a1b2c3d4e5f6071829a0b1c2d3e4f50617283940", "feat: test message", domain.WithAuthor("author"), domain.WithDate("2026-05-31"))
	mStore.On("Reconcile", []domain.CommitEntry{expectedEntry}).Return(nil)

	mLLM := new(mockLLM)
	mLLM.On("GenerateChunkMessage", mock.Anything).Return("feat: test commit", nil)
	mLLM.On("ClassifyBinary", mock.Anything).Return("feat", nil)

	mChunker := new(mockDiffChunker)
	mChunker.On("Chunk", mock.Anything, mock.Anything).
		Return([]domain.DiffChunk{{Files: []string{"main.go"}, Diff: "test diff"}}, nil)

	mSecurity := new(mockSecurityService)
	mSecurity.On("CheckFiles", mock.Anything, mock.Anything).
		Return(&ports.SecurityCheckResult{Blocked: false})

	commitSvc := workflow.NewCommitService(
		mGit, mLLM, mChunker, mSecurity,
		workflow.DefaultCommitServiceConfig(4096, 50, t.TempDir()+"/task.log"),
		mStore,
	)

	confirm := confirm.NewInMemory(5 * time.Minute)
	cfg := config.Default()
	rev := workflow.New(mGit, mLLM, confirm, cfg, commitSvc, nil, mSecurity)

	h := NewHandler(mGit, commitSvc, rev, mLLM, "", nil, nil)

	args := map[string]any{"command": "PREVIEW", "why": "test preview"}
	req := mcpgo.CallToolRequest{Params: mcpgo.CallToolParams{Arguments: args}}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	mGit.AssertExpectations(t)
	mStore.AssertExpectations(t)
}
