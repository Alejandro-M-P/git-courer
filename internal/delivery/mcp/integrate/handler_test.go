package integrate

import (
	"context"
	"testing"
	"time"

	"github.com/blak0p/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockGit struct {
	mock.Mock
}

func (m *MockGit) Status() (domain.Status, error) {
	args := m.Called()
	return args.Get(0).(domain.Status), args.Error(1)
}
func (m *MockGit) CreateBackup(operation string, mode domain.StashMode) (domain.Backup, error) {
	args := m.Called(operation, mode)
	return args.Get(0).(domain.Backup), args.Error(1)
}
func (m *MockGit) RestoreBackup(backup domain.Backup) error {
	args := m.Called(backup)
	return args.Error(0)
}
func (m *MockGit) DeleteBackup(backup domain.Backup) error {
	args := m.Called(backup)
	return args.Error(0)
}
func (m *MockGit) Switch(branch string) error {
	args := m.Called(branch)
	return args.Error(0)
}
func (m *MockGit) Merge(branch string) (string, error) {
	args := m.Called(branch)
	return args.String(0), args.Error(1)
}
func (m *MockGit) MergeAbort() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *MockGit) MergeContinue() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *MockGit) MergeSkip() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *MockGit) Rebase(branch string) (string, error) {
	args := m.Called(branch)
	return args.String(0), args.Error(1)
}
func (m *MockGit) RebaseAbort() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *MockGit) RebaseContinue() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *MockGit) RebaseSkip() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *MockGit) RebaseOnto(newBase, upstream, branch string) (string, error) {
	args := m.Called(newBase, upstream, branch)
	return args.String(0), args.Error(1)
}
func (m *MockGit) CherryPick(commit string) (string, error) {
	args := m.Called(commit)
	return args.String(0), args.Error(1)
}

// Satisfy other methods of ports.Git with dummy implementations
func (m *MockGit) PushToBranch(remote, branch string) (string, error)      { return "", nil }
func (m *MockGit) PullFromBranch(remote, branch string) (string, error)    { return "", nil }
func (m *MockGit) Branch(name string) (string, error)                      { return "", nil }
func (m *MockGit) DeleteBranch(name string, force bool) (string, error)    { return "", nil }
func (m *MockGit) RenameBranch(oldName, newName string) (string, error)    { return "", nil }
func (m *MockGit) DeleteRemoteBranch(name string) error                    { return nil }
func (m *MockGit) Tag(name, message string) (string, error)                { return "", nil }
func (m *MockGit) TagFromFile(name, path string) (string, error)           { return "", nil }
func (m *MockGit) PushTag(name string) (string, error)                     { return "", nil }
func (m *MockGit) DeleteTag(name string) (string, error)                   { return "", nil }
func (m *MockGit) DeleteTagRemote(name string) (string, error)             { return "", nil }
func (m *MockGit) SetUpstream(branch, remote string) (string, error)       { return "", nil }
func (m *MockGit) UnsetUpstream(branch string) (string, error)             { return "", nil }
func (m *MockGit) Diff(paths ...string) (string, error)                    { return "", nil }
func (m *MockGit) DiffStat(paths ...string) (string, error)                { return "", nil }
func (m *MockGit) DiffStatStaged(paths ...string) (string, error)          { return "", nil }
func (m *MockGit) DiffAll(paths ...string) (string, error)                 { return "", nil }
func (m *MockGit) DiffStaged(paths ...string) (string, error)              { return "", nil }
func (m *MockGit) DiffRange(base, target, mode string, paths ...string) (string, error) { return "", nil }
func (m *MockGit) ListUntracked() ([]string, error)                        { return nil, nil }
func (m *MockGit) Log(limit int, pattern string, paths ...string) (string, error) { return "", nil }
func (m *MockGit) LogFull(limit int) (string, error)                       { return "", nil }
func (m *MockGit) CurrentBranch() (string, error)                          { return "", nil }
func (m *MockGit) ListBranches(pattern ...string) (string, error)           { return "", nil }
func (m *MockGit) ListTags(pattern ...string) ([]string, error)            { return nil, nil }
func (m *MockGit) IsRepo() bool                                          { return true }
func (m *MockGit) RemoteURL() (string, error)                            { return "", nil }
func (m *MockGit) RemoteInfo() (string, error)                           { return "", nil }
func (m *MockGit) Search(pattern string, context, before, after int, paths ...string) (string, error) { return "", nil }
func (m *MockGit) CatFile(revision, path string) (string, error)             { return "", nil }
func (m *MockGit) ListTree(revision, path string, recursive bool) ([]string, error) { return nil, nil }
func (m *MockGit) LatestTag() (string, error)                                { return "", nil }
func (m *MockGit) CommitsFromTag(sinceTag string) (string, error)            { return "", nil }
func (m *MockGit) TagExists(name string) (bool, error)                       { return false, nil }
func (m *MockGit) IsGHAuthenticated() (bool, error)                          { return false, nil }
func (m *MockGit) CreateRelease(tagName, changelog string) (string, error)   { return "", nil }
func (m *MockGit) Blame(filepath string) ([]domain.BlameLine, error)         { return nil, nil }
func (m *MockGit) Show(hash string) (domain.ShowResult, error)               { return domain.ShowResult{}, nil }
func (m *MockGit) Reflog() ([]domain.ReflogEntry, error)                     { return nil, nil }
func (m *MockGit) StashList() ([]domain.StashEntry, error)                   { return nil, nil }
func (m *MockGit) StashDiff(index string) (string, error)                    { return "", nil }
func (m *MockGit) StashShow() (string, error)                                { return "", nil }
func (m *MockGit) MergeBase(a, b string) (string, error)                     { return "", nil }
func (m *MockGit) LogRange(from, to string) (string, error)                  { return "", nil }
func (m *MockGit) ListBackups() ([]domain.Backup, error)                     { return nil, nil }
func (m *MockGit) PruneBackups(olderThan time.Duration) error                { return nil }
func (m *MockGit) Add(paths []string) error                                  { return nil }
func (m *MockGit) Remove(paths []string) error                               { return nil }
func (m *MockGit) Commit(message string) (string, error)                     { return "", nil }
func (m *MockGit) Push() (string, error)                                     { return "", nil }
func (m *MockGit) PushTo(remoteBranch string) (string, error)                { return "", nil }
func (m *MockGit) Pull() (string, error)                                     { return "", nil }
func (m *MockGit) PullFrom(remoteBranch string) (string, error)              { return "", nil }
func (m *MockGit) Fetch() (string, error)                                    { return "", nil }
func (m *MockGit) Stash(message ...string) (string, error)                   { return "", nil }
func (m *MockGit) StashWithUntracked(message string) (string, error)         { return "", nil }
func (m *MockGit) StashPop() (string, error)                                 { return "", nil }
func (m *MockGit) StashApply(index string) (string, error)                   { return "", nil }
func (m *MockGit) StashDrop(index string) (string, error)                    { return "", nil }
func (m *MockGit) StashClear() (string, error)                               { return "", nil }
func (m *MockGit) Restore(paths []string) error                              { return nil }
func (m *MockGit) Clean() error                                              { return nil }
func (m *MockGit) Reset(mode string, commit string) (string, error)          { return "", nil }
func (m *MockGit) ResetSoft(ref string) error                                { return nil }
func (m *MockGit) Revert(commit string) (string, error)                      { return "", nil }
func (m *MockGit) Amend(message string, paths []string) (string, error)      { return "", nil }
func (m *MockGit) ShowCommit(commit string) (string, error)                  { return "", nil }
func (m *MockGit) RemoteAdd(name, url string) (string, error)                { return "", nil }
func (m *MockGit) RemoteRemove(name string) (string, error)                  { return "", nil }
func (m *MockGit) ConfigGet(key string) (string, error)                      { return "", nil }
func (m *MockGit) ConfigSet(key, value string) (string, error)               { return "", nil }
func (m *MockGit) SymbolicRef(ref string) (string, error)                   { return "", nil }
func (m *MockGit) WriteTree() (string, error)                                { return "", nil }
func (m *MockGit) CommitTree(treeHash, parentHash, message string) (string, error) { return "", nil }
func (m *MockGit) UpdateRef(ref, commitHash string) (string, error)          { return "", nil }
func (m *MockGit) Head() (string, error)                                     { return "", nil }
func (m *MockGit) HashObject(data []byte) (string, error)                    { return "", nil }
func (m *MockGit) ShowRef(pattern string) (string, error)                    { return "", nil }

func (m *MockGit) MergeConflicts() ([]string, error) {
	return nil, nil
}



func TestHandleIntegrate_Merge(t *testing.T) {
	t.Run("success merge", func(t *testing.T) {
		mockGit := new(MockGit)
		h := NewHandler(mockGit)

		mockGit.On("CreateBackup", "MERGE", domain.StashNone).Return(domain.Backup{}, nil)
		mockGit.On("Merge", "feature-branch").Return("Merged successfully", nil)
		mockGit.On("DeleteBackup", mock.Anything).Return(nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Arguments: map[string]any{
					"command":     "MERGE",
					"branch_name": "feature-branch",
				},
			},
		}

		res, err := h.HandleIntegrate(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Contains(t, res.Content[0].(mcpgo.TextContent).Text, "MERGE")
		mockGit.AssertExpectations(t)
	})
}

func TestHandleIntegrate_Update(t *testing.T) {
	t.Run("success rebase", func(t *testing.T) {
		mockGit := new(MockGit)
		h := NewHandler(mockGit)

		mockGit.On("CreateBackup", "REBASE", domain.StashNone).Return(domain.Backup{}, nil)
		mockGit.On("Rebase", "main").Return("Rebased successfully", nil)
		mockGit.On("DeleteBackup", mock.Anything).Return(nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Arguments: map[string]any{
					"command":     "UPDATE",
					"branch_name": "main",
				},
			},
		}

		res, err := h.HandleIntegrate(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Contains(t, res.Content[0].(mcpgo.TextContent).Text, "REBASE")
		mockGit.AssertExpectations(t)
	})
}

func TestHandleIntegrate_Pick(t *testing.T) {
	t.Run("success cherry-pick", func(t *testing.T) {
		mockGit := new(MockGit)
		h := NewHandler(mockGit)

		mockGit.On("CreateBackup", "CHERRY_PICK", domain.StashNone).Return(domain.Backup{}, nil)
		mockGit.On("CherryPick", "abc1234").Return("Cherry-picked successfully", nil)
		mockGit.On("DeleteBackup", mock.Anything).Return(nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Arguments: map[string]any{
					"command":       "PICK",
					"target_commit": "abc1234",
				},
			},
		}

		res, err := h.HandleIntegrate(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Contains(t, res.Content[0].(mcpgo.TextContent).Text, "CHERRY_PICK")
		mockGit.AssertExpectations(t)
	})
}

func TestHandleIntegrate_Continue(t *testing.T) {
	t.Run("success merge continue", func(t *testing.T) {
		mockGit := new(MockGit)
		h := NewHandler(mockGit)

		mockGit.On("MergeContinue").Return("Continued merge", nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Arguments: map[string]any{
					"command": "CONTINUE",
				},
			},
		}

		res, err := h.HandleIntegrate(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Contains(t, res.Content[0].(mcpgo.TextContent).Text, "merge conflict resolved")
		mockGit.AssertExpectations(t)
	})
}

func TestHandleIntegrate_Abort(t *testing.T) {
	t.Run("success merge abort", func(t *testing.T) {
		mockGit := new(MockGit)
		h := NewHandler(mockGit)

		mockGit.On("MergeAbort").Return("Aborted merge", nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Arguments: map[string]any{
					"command": "ABORT",
				},
			},
		}

		res, err := h.HandleIntegrate(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Contains(t, res.Content[0].(mcpgo.TextContent).Text, "Merge aborted")
		mockGit.AssertExpectations(t)
	})
}
