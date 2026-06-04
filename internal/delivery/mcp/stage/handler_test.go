package stage

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

// mockGitForStage is a testify-based mock implementing ports.Git for stage tests.
type mockGitForStage struct {
	mock.Mock
}

func (m *mockGitForStage) Add(paths []string) error {
	args := m.Called(paths)
	return args.Error(0)
}
func (m *mockGitForStage) Remove(paths []string) error {
	args := m.Called(paths)
	return args.Error(0)
}
func (m *mockGitForStage) Restore(paths []string) error {
	args := m.Called(paths)
	return args.Error(0)
}
func (m *mockGitForStage) ResetSoft(ref string) error {
	args := m.Called(ref)
	return args.Error(0)
}
func (m *mockGitForStage) Reset(mode, commit string) (string, error) {
	args := m.Called(mode, commit)
	return args.String(0), args.Error(1)
}
func (m *mockGitForStage) Clean() error {
	args := m.Called()
	return args.Error(0)
}
func (m *mockGitForStage) CreateBackup(operation string, mode domain.StashMode) (domain.Backup, error) {
	args := m.Called(operation, mode)
	return args.Get(0).(domain.Backup), args.Error(1)
}
func (m *mockGitForStage) Stash(message ...string) (string, error) {
	args := m.Called(message)
	return args.String(0), args.Error(1)
}
func (m *mockGitForStage) StashPop() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *mockGitForStage) StashApply(index string) (string, error) {
	args := m.Called(index)
	return args.String(0), args.Error(1)
}
func (m *mockGitForStage) StashDrop(index string) (string, error) {
	args := m.Called(index)
	return args.String(0), args.Error(1)
}
func (m *mockGitForStage) StashClear() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *mockGitForStage) StashShow() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *mockGitForStage) StashWithUntracked(message string) (string, error) {
	args := m.Called(message)
	return args.String(0), args.Error(1)
}
func (m *mockGitForStage) Checkout(ref string) error {
	args := m.Called(ref)
	return args.Error(0)
}

// --- Remaining ports.Git methods (stubs for interface satisfaction, not used in stage tests) ---

func (m *mockGitForStage) Amend(msg string, paths []string) (string, error) { panic("not implemented") }
func (m *mockGitForStage) Blame(filepath string) ([]domain.BlameLine, error) {
	panic("not implemented")
}
func (m *mockGitForStage) Branch(name string) (string, error)             { panic("not implemented") }
func (m *mockGitForStage) CatFile(revision, path string) (string, error)  { panic("not implemented") }
func (m *mockGitForStage) CherryPick(commit string) (string, error)       { panic("not implemented") }
func (m *mockGitForStage) Clone(repo, dest string) error                  { panic("not implemented") }
func (m *mockGitForStage) Commit(message string) (string, error)          { panic("not implemented") }
func (m *mockGitForStage) CommitsFromTag(sinceTag string) (string, error) { panic("not implemented") }
func (m *mockGitForStage) Config(args ...string) (string, error)          { panic("not implemented") }
func (m *mockGitForStage) CreateRelease(tagName, changelog string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForStage) CurrentBranch() (string, error)          { panic("not implemented") }
func (m *mockGitForStage) DeleteBackup(backup domain.Backup) error { panic("not implemented") }
func (m *mockGitForStage) DeleteBranch(name string, force bool) (string, error) {
	panic("not implemented")
}
func (m *mockGitForStage) DeleteRemoteBranch(name string) error        { panic("not implemented") }
func (m *mockGitForStage) DeleteTag(name string) (string, error)       { panic("not implemented") }
func (m *mockGitForStage) DeleteTagRemote(name string) (string, error) { panic("not implemented") }
func (m *mockGitForStage) Diff(paths ...string) (string, error)        { panic("not implemented") }
func (m *mockGitForStage) DiffAll(paths ...string) (string, error)     { panic("not implemented") }
func (m *mockGitForStage) DiffRange(base, target, mode string, paths ...string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForStage) DiffStat(paths ...string) (string, error)       { panic("not implemented") }
func (m *mockGitForStage) DiffStatStaged(paths ...string) (string, error) { panic("not implemented") }
func (m *mockGitForStage) DiffStaged(paths ...string) (string, error)     { panic("not implemented") }
func (m *mockGitForStage) Fetch() (string, error)                         { panic("not implemented") }
func (m *mockGitForStage) IsGHAuthenticated() (bool, error)               { panic("not implemented") }
func (m *mockGitForStage) IsRepo() bool                                   { panic("not implemented") }
func (m *mockGitForStage) LatestTag() (string, error)                     { panic("not implemented") }
func (m *mockGitForStage) ListBackups() ([]domain.Backup, error)          { panic("not implemented") }
func (m *mockGitForStage) ListBranches(pattern ...string) (string, error) { panic("not implemented") }
func (m *mockGitForStage) ListTags(pattern ...string) ([]string, error)   { panic("not implemented") }
func (m *mockGitForStage) ListTree(revision, path string, recursive bool) ([]string, error) {
	panic("not implemented")
}
func (m *mockGitForStage) ListUntracked() ([]string, error) { panic("not implemented") }
func (m *mockGitForStage) Log(limit int, pattern string, paths ...string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForStage) LogFull(limit int) (string, error)        { panic("not implemented") }
func (m *mockGitForStage) LogFile() string                          { panic("not implemented") }
func (m *mockGitForStage) Merge(branch string) (string, error)      { panic("not implemented") }
func (m *mockGitForStage) MergeAbort() (string, error)              { panic("not implemented") }
func (m *mockGitForStage) MergeContinue() (string, error)           { panic("not implemented") }
func (m *mockGitForStage) MergeSkip() (string, error)               { panic("not implemented") }
func (m *mockGitForStage) MergeBase(a, b string) (string, error)    { panic("not implemented") }
func (m *mockGitForStage) LogRange(from, to string) (string, error) { return "", nil }

func (m *mockGitForStage) PruneBackups(olderThan time.Duration) error   { panic("not implemented") }
func (m *mockGitForStage) Pull() (string, error)                        { panic("not implemented") }
func (m *mockGitForStage) PullFrom(remoteBranch string) (string, error) { panic("not implemented") }
func (m *mockGitForStage) Push() (string, error)                        { panic("not implemented") }
func (m *mockGitForStage) PushTag(name string) (string, error)          { panic("not implemented") }
func (m *mockGitForStage) PushTo(remoteBranch string) (string, error)   { panic("not implemented") }
func (m *mockGitForStage) Rebase(branch string) (string, error)         { panic("not implemented") }
func (m *mockGitForStage) RebaseAbort() (string, error)                 { panic("not implemented") }
func (m *mockGitForStage) RebaseContinue() (string, error)              { panic("not implemented") }
func (m *mockGitForStage) RebaseSkip() (string, error)                  { panic("not implemented") }
func (m *mockGitForStage) RebaseOnto(newBase, upstream, branch string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForStage) PushToBranch(remote, branch string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForStage) PullFromBranch(remote, branch string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForStage) Reflog() ([]domain.ReflogEntry, error)      { panic("not implemented") }
func (m *mockGitForStage) RemoteAdd(name, url string) (string, error) { panic("not implemented") }
func (m *mockGitForStage) RemoteInfo() (string, error)                { panic("not implemented") }
func (m *mockGitForStage) RemoteRemove(name string) (string, error)   { panic("not implemented") }
func (m *mockGitForStage) RemoteURL() (string, error)                 { panic("not implemented") }
func (m *mockGitForStage) RenameBranch(oldName, newName string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForStage) RestoreBackup(backup domain.Backup) error { panic("not implemented") }
func (m *mockGitForStage) Revert(commit string) (string, error)     { panic("not implemented") }
func (m *mockGitForStage) RevParse(ref string) (string, error)      { panic("not implemented") }
func (m *mockGitForStage) Search(pattern string, context, before, after int, paths ...string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForStage) SetOrigin(url string) error       { panic("not implemented") }
func (m *mockGitForStage) SetRemote(name, url string) error { panic("not implemented") }
func (m *mockGitForStage) SetUpstream(branch, remote string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForStage) Show(hash string) (domain.ShowResult, error) { panic("not implemented") }
func (m *mockGitForStage) ShowCommit(commit string) (string, error)    { panic("not implemented") }
func (m *mockGitForStage) Status() (domain.Status, error)              { panic("not implemented") }
func (m *mockGitForStage) StashDiff(index string) (string, error)      { panic("not implemented") }
func (m *mockGitForStage) StashList() ([]domain.StashEntry, error)     { panic("not implemented") }
func (m *mockGitForStage) Switch(branch string) error                  { panic("not implemented") }
func (m *mockGitForStage) Tag(name, message string) (string, error)    { panic("not implemented") }
func (m *mockGitForStage) TagExists(name string) (bool, error)         { panic("not implemented") }
func (m *mockGitForStage) UnsetUpstream(branch string) (string, error) { panic("not implemented") }
func (m *mockGitForStage) ConfigGet(key string) (string, error) {
	args := m.Called(key)
	return args.String(0), args.Error(1)
}
func (m *mockGitForStage) ConfigSet(key, value string) (string, error) {
	args := m.Called(key, value)
	return args.String(0), args.Error(1)
}
func (m *mockGitForStage) SymbolicRef(ref string) (string, error) {
	args := m.Called(ref)
	return args.String(0), args.Error(1)
}
func (m *mockGitForStage) WriteTree() (string, error) { panic("not implemented") }
func (m *mockGitForStage) CommitTree(treeHash, parentHash, message string) (string, error) {
	panic("not implemented")
}
func (m *mockGitForStage) UpdateRef(ref, commitHash string) (string, error) { panic("not implemented") }
func (m *mockGitForStage) Head() (string, error)                            { panic("not implemented") }
func (m *mockGitForStage) Version() (string, error)                         { panic("not implemented") }
func (m *mockGitForStage) WorkDir() string                                  { panic("not implemented") }
func (m *mockGitForStage) WithWorkDir(dir string) interface {
	Git() interface{}
	Err() error
} { panic("not implemented") }

func TestHandler_HandleStage(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		args     map[string]any
		setup    func(*mockGitForStage)
		expected string
	}{
		{
			name:    "ADD with target_paths",
			command: "ADD",
			args:    map[string]any{"target_paths": "a.go b.go"},
			setup: func(m *mockGitForStage) {
				m.On("CreateBackup", "ADD", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("Add", []string{"a.go", "b.go"}).Return(nil)
			},
			expected: "2 files staged",
		},
		{
			name:    "RM with target_paths",
			command: "RM",
			args:    map[string]any{"target_paths": "old.go"},
			setup: func(m *mockGitForStage) {
				m.On("CreateBackup", "RM", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("Remove", []string{"old.go"}).Return(nil)
			},
			expected: "1 files removed",
		},
		{
			name:    "RESTORE with target_paths",
			command: "RESTORE",
			args:    map[string]any{"target_paths": "file.go"},
			setup: func(m *mockGitForStage) {
				m.On("CreateBackup", "RESTORE", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("Restore", []string{"file.go"}).Return(nil)
			},
			expected: "1 files restored",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := new(mockGitForStage)
			tt.setup(git)

			h := NewHandler(git, nil)

			args := map[string]any{"command": tt.command}
			for k, v := range tt.args {
				args[k] = v
			}
			req := mcpgo.CallToolRequest{
				Params: mcpgo.CallToolParams{
					Name:      "stage",
					Arguments: args,
				},
			}

			res, err := h.HandleStage(context.Background(), req)
			assert.NoError(t, err)
			assert.NotNil(t, res)

			text := res.Content[0].(mcpgo.TextContent).Text
			var parsed map[string]any
			assert.NoError(t, json.Unmarshal([]byte(text), &parsed))
			assert.Contains(t, parsed["message"], tt.expected)
			git.AssertExpectations(t)
		})
	}
}

func TestHandler_HandleStage_MissingRequiredParams(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    map[string]any
		wantErr string
	}{
		{
			name:    "ADD without target_paths",
			command: "ADD",
			args:    map[string]any{},
			wantErr: "target_paths is required for ADD",
		},
		{
			name:    "RM without target_paths",
			command: "RM",
			args:    map[string]any{},
			wantErr: "target_paths is required for RM",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			git := new(mockGitForStage)

			h := NewHandler(git, nil)

			args := map[string]any{"command": tt.command}
			for k, v := range tt.args {
				args[k] = v
			}
			req := mcpgo.CallToolRequest{
				Params: mcpgo.CallToolParams{
					Name:      "stage",
					Arguments: args,
				},
			}

			res, err := h.HandleStage(context.Background(), req)
			assert.NoError(t, err)
			assert.NotNil(t, res)

			var result map[string]any
			err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
			assert.NoError(t, err)
			assert.Contains(t, result["error"], tt.wantErr)
		})
	}
}

func TestHandler_HandleStage_UnknownCommand(t *testing.T) {
	git := new(mockGitForStage)

	h := NewHandler(git, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "stage",
			Arguments: map[string]any{"command": "COMMIT"},
		},
	}

	res, err := h.HandleStage(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "unknown command")
}

func TestHandler_HandleStage_CleanBlockedWithoutConfirmed(t *testing.T) {
	git := new(mockGitForStage)

	h := NewHandler(git, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "stage",
			Arguments: map[string]any{"command": "CLEAN", "confirmed": false},
		},
	}

	res, err := h.HandleStage(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "blocked")
}

func TestHandler_HandleStage_CleanWithConfirmed(t *testing.T) {
	git := new(mockGitForStage)
	git.On("CreateBackup", "CLEAN", domain.StashNone).Return(domain.Backup{}, nil)
	git.On("Clean").Return(nil)

	h := NewHandler(git, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "stage",
			Arguments: map[string]any{"command": "CLEAN", "confirmed": true},
		},
	}

	res, err := h.HandleStage(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["message"], "Untracked files cleaned")
	git.AssertExpectations(t)
}

func TestHandler_HandleStage_CleanDryRun(t *testing.T) {
	git := new(mockGitForStage)

	h := NewHandler(git, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "stage",
			Arguments: map[string]any{"command": "CLEAN", "dry_run": true},
		},
	}

	res, err := h.HandleStage(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "affected_files")
}

// --- Reset handler tests ---

func TestHandler_HandleReset_Soft(t *testing.T) {
	git := new(mockGitForStage)
	git.On("CreateBackup", "SOFT", domain.StashNone).Return(domain.Backup{}, nil)
	git.On("ResetSoft", "abc123").Return(nil)

	h := NewHandler(git, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "reset",
			Arguments: map[string]any{"command": "SOFT", "target_commit": "abc123"},
		},
	}

	res, err := h.HandleReset(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["message"], "Soft reset to abc123")
	git.AssertExpectations(t)
}

func TestHandler_HandleReset_Mixed(t *testing.T) {
	git := new(mockGitForStage)
	git.On("CreateBackup", "MIXED", domain.StashNone).Return(domain.Backup{}, nil)
	git.On("Reset", "--mixed", "def456").Return("reset output", nil)

	h := NewHandler(git, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "reset",
			Arguments: map[string]any{"command": "MIXED", "target_commit": "def456"},
		},
	}

	res, err := h.HandleReset(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["message"], "Mixed reset to def456")
	git.AssertExpectations(t)
}

func TestHandler_HandleReset_HardBlockedWithoutConfirmed(t *testing.T) {
	git := new(mockGitForStage)

	h := NewHandler(git, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "reset",
			Arguments: map[string]any{"command": "HARD", "target_commit": "abc", "confirmed": false},
		},
	}

	res, err := h.HandleReset(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "blocked")
}

func TestHandler_HandleReset_HardWithConfirmed(t *testing.T) {
	git := new(mockGitForStage)
	git.On("CreateBackup", "HARD", domain.StashNone).Return(domain.Backup{}, nil)
	git.On("Reset", "--hard", "abc").Return("", nil)

	h := NewHandler(git, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "reset",
			Arguments: map[string]any{"command": "HARD", "target_commit": "abc", "confirmed": true},
		},
	}

	res, err := h.HandleReset(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["message"], "Hard reset to abc")
	git.AssertExpectations(t)
}

// --- Stash handler tests ---

func TestHandler_HandleStash_Save(t *testing.T) {
	git := new(mockGitForStage)
	git.On("CreateBackup", "SAVE", domain.StashNone).Return(domain.Backup{}, nil)
	git.On("Stash", []string(nil)).Return("stashed", nil)

	h := NewHandler(git, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "stash",
			Arguments: map[string]any{"command": "SAVE"},
		},
	}

	res, err := h.HandleStash(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["message"], "Changes stashed")
	git.AssertExpectations(t)
}

func TestHandler_HandleStash_SaveWithMessage(t *testing.T) {
	git := new(mockGitForStage)
	git.On("CreateBackup", "SAVE", domain.StashNone).Return(domain.Backup{}, nil)
	git.On("Stash", []string{"my msg"}).Return("saved", nil)

	h := NewHandler(git, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "stash",
			Arguments: map[string]any{"command": "SAVE", "commit_message": "my msg"},
		},
	}

	res, err := h.HandleStash(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	git.AssertExpectations(t)
}

func TestHandler_HandleStash_Pop(t *testing.T) {
	git := new(mockGitForStage)
	git.On("CreateBackup", "POP", domain.StashNone).Return(domain.Backup{}, nil)
	git.On("StashPop").Return("restored", nil)

	h := NewHandler(git, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "stash",
			Arguments: map[string]any{"command": "POP"},
		},
	}

	res, err := h.HandleStash(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["message"], "Stash restored")
	git.AssertExpectations(t)
}

func TestHandler_HandleStash_Show(t *testing.T) {
	git := new(mockGitForStage)
	git.On("StashShow").Return("stash diff output", nil)

	h := NewHandler(git, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "stash",
			Arguments: map[string]any{"command": "SHOW"},
		},
	}

	res, err := h.HandleStash(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "stash diff output")
	git.AssertExpectations(t)
}

func TestHandler_HandleStash_UnknownCommand(t *testing.T) {
	git := new(mockGitForStage)

	h := NewHandler(git, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "stash",
			Arguments: map[string]any{"command": "APPLY"},
		},
	}

	res, err := h.HandleStash(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "unknown command")
}

// --- Stash POP with stash_index (Phase 1: B8a) ---

func TestHandler_HandleStash_PopWithStashIndex(t *testing.T) {
	git := new(mockGitForStage)
	git.On("CreateBackup", "POP", domain.StashNone).Return(domain.Backup{}, nil)
	git.On("StashApply", "2").Return("applied", nil)

	h := NewHandler(git, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "stash",
			Arguments: map[string]any{"command": "POP", "stash_index": "2"},
		},
	}

	res, err := h.HandleStash(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["message"], "Stash@{2} applied")
	git.AssertExpectations(t)
}

func TestHandler_HandleStash_PopWithoutStashIndex(t *testing.T) {
	git := new(mockGitForStage)
	git.On("CreateBackup", "POP", domain.StashNone).Return(domain.Backup{}, nil)
	git.On("StashPop").Return("restored", nil)

	h := NewHandler(git, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "stash",
			Arguments: map[string]any{"command": "POP"},
		},
	}

	res, err := h.HandleStash(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["message"], "Stash restored")
	git.AssertExpectations(t)
}
