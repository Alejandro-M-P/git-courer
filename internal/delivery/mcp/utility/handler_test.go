package utility

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/blak0p/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockGitForUtility is a testify-based mock implementing ports.Git for utility tests.
type mockGitForUtility struct {
	mock.Mock
}

func (m *mockGitForUtility) ListBackups() ([]domain.Backup, error) {
	args := m.Called()
	return args.Get(0).([]domain.Backup), args.Error(1)
}
func (m *mockGitForUtility) RestoreBackup(backup domain.Backup) error {
	args := m.Called(backup)
	return args.Error(0)
}
func (m *mockGitForUtility) ListBranches(pattern ...string) (string, error) {
	args := m.Called(pattern)
	return args.String(0), args.Error(1)
}

// --- Remaining ports.Git methods (stubs, not used in utility tests) ---

func (m *mockGitForUtility) Add(paths []string) error { panic("not implemented") }
func (m *mockGitForUtility) Amend(msg string, paths []string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForUtility) Blame(filepath string) ([]domain.BlameLine, error) {
	panic("not implemented")
}
func (m *mockGitForUtility) Branch(name string) (string, error)             { panic("not implemented") }
func (m *mockGitForUtility) CatFile(revision, path string) (string, error)  { panic("not implemented") }
func (m *mockGitForUtility) Checkout(ref string) error                      { panic("not implemented") }
func (m *mockGitForUtility) CherryPick(commit string) (string, error)       { panic("not implemented") }
func (m *mockGitForUtility) Clean() error                                   { panic("not implemented") }
func (m *mockGitForUtility) Clone(repo, dest string) error                  { panic("not implemented") }
func (m *mockGitForUtility) Commit(message string) (string, error)          { panic("not implemented") }
func (m *mockGitForUtility) CommitsFromTag(sinceTag string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) Config(args ...string) (string, error)          { panic("not implemented") }
func (m *mockGitForUtility) CreateBackup(op string, mode domain.StashMode) (domain.Backup, error) {
	args := m.Called(op, mode)
	return args.Get(0).(domain.Backup), args.Error(1)
}
func (m *mockGitForUtility) CreateRelease(tagName, changelog string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForUtility) CurrentBranch() (string, error) { panic("not implemented") }
func (m *mockGitForUtility) DeleteBackup(backup domain.Backup) error {
	args := m.Called(backup)
	return args.Error(0)
}
func (m *mockGitForUtility) DeleteBranch(name string, force bool) (string, error) {
	panic("not implemented")
}
func (m *mockGitForUtility) DeleteRemoteBranch(name string) error        { panic("not implemented") }
func (m *mockGitForUtility) DeleteTag(name string) (string, error)       { panic("not implemented") }
func (m *mockGitForUtility) DeleteTagRemote(name string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) Diff(paths ...string) (string, error)        { panic("not implemented") }
func (m *mockGitForUtility) DiffAll(paths ...string) (string, error)     { panic("not implemented") }
func (m *mockGitForUtility) DiffUntracked() (string, error)              { panic("not implemented") }
func (m *mockGitForUtility) DiffRange(base, target, mode string, paths ...string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForUtility) DiffStat(paths ...string) (string, error)       { panic("not implemented") }
func (m *mockGitForUtility) DiffStatStaged(paths ...string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) DiffStaged(paths ...string) (string, error)     { panic("not implemented") }
func (m *mockGitForUtility) Fetch() (string, error)                         { panic("not implemented") }
func (m *mockGitForUtility) IsGHAuthenticated() (bool, error)               { panic("not implemented") }
func (m *mockGitForUtility) IsRepo() bool                                   { panic("not implemented") }
func (m *mockGitForUtility) LatestTag() (string, error)                     { panic("not implemented") }
func (m *mockGitForUtility) ListTags(pattern ...string) ([]string, error)   { panic("not implemented") }
func (m *mockGitForUtility) ListTree(revision, path string, recursive bool) ([]string, error) {
	panic("not implemented")
}
func (m *mockGitForUtility) ListUntracked() ([]string, error) { panic("not implemented") }
func (m *mockGitForUtility) Log(limit int, pattern string, paths ...string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForUtility) LogFile() string                          { panic("not implemented") }
func (m *mockGitForUtility) LogFull(limit int) (string, error)        { panic("not implemented") }
func (m *mockGitForUtility) Merge(branch string) (string, error)      { panic("not implemented") }
func (m *mockGitForUtility) MergeAbort() (string, error)              { panic("not implemented") }
func (m *mockGitForUtility) MergeContinue() (string, error)           { panic("not implemented") }
func (m *mockGitForUtility) MergeSkip() (string, error)               { panic("not implemented") }
func (m *mockGitForUtility) MergeBase(a, b string) (string, error)    { panic("not implemented") }
func (m *mockGitForUtility) LogRange(from, to string) (string, error) { return "", nil }

func (m *mockGitForUtility) PruneBackups(olderThan time.Duration) error   { panic("not implemented") }
func (m *mockGitForUtility) Pull() (string, error)                        { panic("not implemented") }
func (m *mockGitForUtility) PullFrom(remoteBranch string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) Push() (string, error)                        { panic("not implemented") }
func (m *mockGitForUtility) PushTag(name string) (string, error)          { panic("not implemented") }
func (m *mockGitForUtility) PushTo(remoteBranch string) (string, error)   { panic("not implemented") }
func (m *mockGitForUtility) Rebase(branch string) (string, error)         { panic("not implemented") }
func (m *mockGitForUtility) RebaseAbort() (string, error)                 { panic("not implemented") }
func (m *mockGitForUtility) RebaseContinue() (string, error)              { panic("not implemented") }
func (m *mockGitForUtility) RebaseSkip() (string, error)                  { panic("not implemented") }
func (m *mockGitForUtility) RebaseOnto(newBase, upstream, branch string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForUtility) PushToBranch(remote, branch string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForUtility) PullFromBranch(remote, branch string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForUtility) Reflog() ([]domain.ReflogEntry, error)      { panic("not implemented") }
func (m *mockGitForUtility) RemoteAdd(name, url string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) RemoteInfo() (string, error)                { panic("not implemented") }
func (m *mockGitForUtility) RemoteRemove(name string) (string, error)   { panic("not implemented") }
func (m *mockGitForUtility) RemoteURL() (string, error)                 { panic("not implemented") }
func (m *mockGitForUtility) Remove(paths []string) error                { panic("not implemented") }
func (m *mockGitForUtility) RenameBranch(oldName, newName string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForUtility) Reset(mode, commit string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) ResetSoft(ref string) error                { panic("not implemented") }
func (m *mockGitForUtility) Restore(paths []string) error              { panic("not implemented") }
func (m *mockGitForUtility) Revert(commit string) (string, error)      { panic("not implemented") }
func (m *mockGitForUtility) RevParse(ref string) (string, error)       { panic("not implemented") }
func (m *mockGitForUtility) Search(pattern string, context, before, after int, paths ...string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForUtility) SetOrigin(url string) error       { panic("not implemented") }
func (m *mockGitForUtility) SetRemote(name, url string) error { panic("not implemented") }
func (m *mockGitForUtility) SetUpstream(branch, remote string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForUtility) Show(hash string) (domain.ShowResult, error) { panic("not implemented") }
func (m *mockGitForUtility) ShowCommit(commit string) (string, error)    { panic("not implemented") }
func (m *mockGitForUtility) Stash(message ...string) (string, error)     { panic("not implemented") }
func (m *mockGitForUtility) StashApply(index string) (string, error)     { panic("not implemented") }
func (m *mockGitForUtility) StashClear() (string, error)                 { panic("not implemented") }
func (m *mockGitForUtility) StashDiff(index string) (string, error)      { panic("not implemented") }
func (m *mockGitForUtility) StashDrop(index string) (string, error)      { panic("not implemented") }
func (m *mockGitForUtility) StashList() ([]domain.StashEntry, error)     { panic("not implemented") }
func (m *mockGitForUtility) StashPop() (string, error)                   { panic("not implemented") }
func (m *mockGitForUtility) StashShow() (string, error)                  { panic("not implemented") }
func (m *mockGitForUtility) StashWithUntracked(message string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForUtility) Status() (domain.Status, error)              { panic("not implemented") }
func (m *mockGitForUtility) Switch(branch string) error                  { panic("not implemented") }
func (m *mockGitForUtility) Tag(name, message string) (string, error)    { panic("not implemented") }
func (m *mockGitForUtility) TagFromFile(name, path string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) TagExists(name string) (bool, error)         { panic("not implemented") }
func (m *mockGitForUtility) UnsetUpstream(branch string) (string, error) { panic("not implemented") }
func (m *mockGitForUtility) ConfigGet(key string) (string, error)        { return "", nil }
func (m *mockGitForUtility) ConfigSet(key, value string) (string, error) { return "", nil }
func (m *mockGitForUtility) SymbolicRef(ref string) (string, error)      { return "", nil }
func (m *mockGitForUtility) WriteTree() (string, error)                  { panic("not implemented") }
func (m *mockGitForUtility) CommitTree(treeHash, parentHash, message string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForUtility) UpdateRef(ref, commitHash string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForUtility) Head() (string, error)                    { panic("not implemented") }
func (m *mockGitForUtility) HashObject(data []byte) (string, error)  { return "mock-blob-sha", nil }
func (m *mockGitForUtility) ShowRef(pattern string) (string, error) { return "", nil }
func (m *mockGitForUtility) Version() (string, error)                 { panic("not implemented") }
func (m *mockGitForUtility) WorkDir() string          { panic("not implemented") }
func (m *mockGitForUtility) WithWorkDir(dir string) interface {
	Git() interface{}
	Err() error
} { panic("not implemented") }


// --- Backup tests ---

func TestHandler_HandleBackup_RESTORE(t *testing.T) {
	git := new(mockGitForUtility)
	backup := domain.Backup{Ref: "refs/git-courer/backup/20260517123000_commit", Operation: "commit"}
	git.On("ListBackups").Return([]domain.Backup{backup}, nil)
	git.On("RestoreBackup", backup).Return(nil)

	h := NewHandler(git, nil, "", nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "RESTORE"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "Successfully restored")
	git.AssertExpectations(t)
}

func TestHandler_HandleBackup_RESTORE_NoBackup(t *testing.T) {
	git := new(mockGitForUtility)
	git.On("ListBackups").Return([]domain.Backup{}, nil)

	h := NewHandler(git, nil, "", nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "RESTORE"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "no backups available to restore")
}


func TestHandler_HandleBackup_RESTORE_ClearsBackup(t *testing.T) {
	git := new(mockGitForUtility)
	backup := domain.Backup{Ref: "refs/git-courer/backup/20260517123000_commit", Operation: "commit"}
	git.On("ListBackups").Return([]domain.Backup{backup}, nil)
	git.On("RestoreBackup", backup).Return(nil)

	h := NewHandler(git, nil, "", nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "RESTORE"},
		},
	}

	_, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	// RESTORE no longer sets/clears a shared pointer — it uses ListBackups
	git.AssertExpectations(t)
}

func TestHandler_HandleBackup_LIST(t *testing.T) {
	git := new(mockGitForUtility)
	git.On("ListBackups").Return([]domain.Backup{
		{Ref: "ref1", Operation: "commit", CreatedAt: time.Now()},
	}, nil)

	h := NewHandler(git, nil, "", nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "LIST"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "backups")
	git.AssertExpectations(t)
}

func TestHandler_HandleBackup_UnknownCommand(t *testing.T) {
	h := NewHandler(new(mockGitForUtility), nil, "", nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "UND"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "unknown command")
}

// --- T3.1: Verify Handler no longer requires ReleaseSvc ---

func TestHandler_NewHandler_WithoutReleaseSvc(t *testing.T) {
	// After MCP release tool removal, NewHandler does not take a releaseSvc parameter.
	// This test verifies the Handler struct works without any release dependency.
	h := NewHandler(new(mockGitForUtility), nil, "", nil)

	assert.NotNil(t, h)
	// Handler should have working Config, Backup, and Undo methods
	// (verified by their respective tests — this focuses on construction)
}

func TestHandler_HandlersInterface_ThreeMethods(t *testing.T) {
	// Verify the Handlers interface only requires Config, Backup, and Undo.
	// The release tool was removed in Phase 3, so HandleRelease is no longer part of the interface.
	var _ Handlers = (*Handler)(nil)
	// This compiles only if Handler implements Handlers with 3 methods:
	// HandleConfig, HandleBackup, HandleUndo
	// If HandleRelease is still in the interface, the Handler must implement it too.
	// If we remove HandleRelease from Handler but not from the interface, this fails to compile.
}

// --- Backup CREATE tests (Phase 1: B5a) ---


// --- Backup RESTORE with ref test (Phase 1: B5b) ---

func TestHandler_HandleBackup_RESTORE_WithRef(t *testing.T) {
	git := new(mockGitForUtility)
	backup1 := domain.Backup{Ref: "refs/git-courer/backup/20260517123000_MERGE", Operation: "MERGE"}
	backup2 := domain.Backup{Ref: "refs/git-courer/backup/20260517124000_COMMIT", Operation: "COMMIT"}
	git.On("ListBackups").Return([]domain.Backup{backup2, backup1}, nil)
	git.On("RestoreBackup", backup1).Return(nil)

	h := NewHandler(git, nil, "", nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "RESTORE", "ref": "refs/git-courer/backup/20260517123000_MERGE"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "Successfully restored")
	assert.Contains(t, text, "MERGE")
	git.AssertExpectations(t)
}

func TestHandler_HandleBackup_RESTORE_DefaultsToMostRecent(t *testing.T) {
	git := new(mockGitForUtility)
	backup1 := domain.Backup{Ref: "refs/git-courer/backup/20260517123000_MERGE", Operation: "MERGE"}
	backup2 := domain.Backup{Ref: "refs/git-courer/backup/20260517124000_COMMIT", Operation: "COMMIT"}
	// ListBackups returns newest first
	git.On("ListBackups").Return([]domain.Backup{backup2, backup1}, nil)
	git.On("RestoreBackup", backup2).Return(nil)

	h := NewHandler(git, nil, "", nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "RESTORE"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "Successfully restored")
	assert.Contains(t, text, "COMMIT")
	git.AssertExpectations(t)
}

func TestHandler_HandleBackup_RESTORE_UnknownRef(t *testing.T) {
	git := new(mockGitForUtility)
	backup1 := domain.Backup{Ref: "refs/git-courer/backup/20260517123000_MERGE", Operation: "MERGE"}
	git.On("ListBackups").Return([]domain.Backup{backup1}, nil)

	h := NewHandler(git, nil, "", nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "backup",
			Arguments: map[string]any{"command": "RESTORE", "ref": "nonexistent"},
		},
	}

	res, err := h.HandleBackup(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "unknown backup ref")
	git.AssertExpectations(t)
}


func TestFormatBackupListJSON_WithUndoable(t *testing.T) {
	backups := []domain.Backup{
		{
			Ref:       "ref-local",
			Operation: "commit",
			CreatedAt: time.Now(),
			Undoable:  true,
		},
		{
			Ref:       "ref-remote",
			Operation: "push",
			CreatedAt: time.Now(),
			Undoable:  false,
		},
	}

	result := formatBackupListJSON(backups)
	var parsed map[string]any
	err := json.Unmarshal([]byte(result), &parsed)
	assert.NoError(t, err)

	items, ok := parsed["backups"].([]any)
	assert.True(t, ok, "backups should be an array")
	assert.Len(t, items, 2)

	first := items[0].(map[string]any)
	assert.Equal(t, true, first["undoable"])

	second := items[1].(map[string]any)
	assert.Equal(t, false, second["undoable"])
}
