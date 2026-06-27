package history

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

type mockGitForHistory struct {
	mock.Mock
}

func (m *mockGitForHistory) Status() (domain.Status, error) { panic("unexpected") }
func (m *mockGitForHistory) Diff(paths ...string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) DiffStat(paths ...string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) DiffStatStaged(paths ...string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) DiffAll(paths ...string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) DiffUntracked() (string, error)          { panic("unexpected") }
func (m *mockGitForHistory) DiffRange(base, target, mode string, paths ...string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) DiffStaged(paths ...string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) ListUntracked() ([]string, error) { panic("unexpected") }
func (m *mockGitForHistory) Log(limit int, pattern string, paths ...string) (string, error) {
	args := m.Called(limit, pattern, paths)
	return args.String(0), args.Error(1)
}
func (m *mockGitForHistory) LogRange(from, to string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) LogFull(limit int) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) CurrentBranch() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) ListBranches(pattern ...string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) ListTags(pattern ...string) ([]string, error) { panic("unexpected") }
func (m *mockGitForHistory) IsRepo() bool { panic("unexpected") }
func (m *mockGitForHistory) RemoteURL() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) RemoteInfo() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) Search(pattern string, context, before, after int, paths ...string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) CatFile(revision, path string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) ListTree(revision, path string, recursive bool) ([]string, error) { panic("unexpected") }
func (m *mockGitForHistory) LatestTag() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) CommitsFromTag(sinceTag string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) TagExists(name string) (bool, error) { panic("unexpected") }
func (m *mockGitForHistory) IsGHAuthenticated() (bool, error) { panic("unexpected") }
func (m *mockGitForHistory) CreateRelease(tagName, changelog string) (string, error) { panic("unexpected") }

func (m *mockGitForHistory) Blame(filepath string) ([]domain.BlameLine, error) {
	args := m.Called(filepath)
	return args.Get(0).([]domain.BlameLine), args.Error(1)
}
func (m *mockGitForHistory) Show(hash string) (domain.ShowResult, error) { panic("unexpected") }
func (m *mockGitForHistory) Reflog() ([]domain.ReflogEntry, error) {
	args := m.Called()
	return args.Get(0).([]domain.ReflogEntry), args.Error(1)
}
func (m *mockGitForHistory) StashList() ([]domain.StashEntry, error) { panic("unexpected") }
func (m *mockGitForHistory) StashDiff(index string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) StashShow() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) MergeBase(a, b string) (string, error) { panic("unexpected") }

func (m *mockGitForHistory) CreateBackup(operation string, mode domain.StashMode) (domain.Backup, error) { panic("unexpected") }
func (m *mockGitForHistory) RestoreBackup(backup domain.Backup) error { panic("unexpected") }
func (m *mockGitForHistory) DeleteBackup(backup domain.Backup) error { panic("unexpected") }
func (m *mockGitForHistory) ListBackups() ([]domain.Backup, error) { panic("unexpected") }
func (m *mockGitForHistory) PruneBackups(olderThan time.Duration) error { panic("unexpected") }

func (m *mockGitForHistory) Add(paths []string) error { panic("unexpected") }
func (m *mockGitForHistory) Remove(paths []string) error { panic("unexpected") }
func (m *mockGitForHistory) Commit(message string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) Push() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) PushTo(remoteBranch string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) Pull() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) PullFrom(remoteBranch string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) Fetch() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) Stash(message ...string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) StashPop() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) StashApply(index string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) StashDrop(index string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) StashClear() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) Switch(branch string) error { panic("unexpected") }
func (m *mockGitForHistory) Branch(name string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) DeleteBranch(name string, force bool) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) RenameBranch(oldName, newName string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) DeleteRemoteBranch(name string) error { panic("unexpected") }
func (m *mockGitForHistory) Tag(name, message string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) TagFromFile(name, path string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) PushTag(name string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) DeleteTag(name string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) DeleteTagRemote(name string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) Merge(branch string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) MergeAbort() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) MergeContinue() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) MergeSkip() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) Reset(mode, commit string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) ResetSoft(ref string) error { panic("unexpected") }
func (m *mockGitForHistory) Restore(paths []string) error { panic("unexpected") }
func (m *mockGitForHistory) Clean() error { panic("unexpected") }
func (m *mockGitForHistory) Rebase(branch string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) RebaseAbort() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) RebaseContinue() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) RebaseSkip() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) RebaseOnto(newBase, upstream, branch string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) PushToBranch(remote, branch string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) PullFromBranch(remote, branch string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) CherryPick(commit string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) Revert(commit string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) Amend(message string, paths []string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) ShowCommit(commit string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) RemoteAdd(name, url string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) RemoteRemove(name string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) SetUpstream(branch, remote string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) UnsetUpstream(branch string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) StashWithUntracked(message string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) ConfigGet(key string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) ConfigSet(key, value string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) SymbolicRef(ref string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) WriteTree() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) CommitTree(treeHash, parentHash, message string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) UpdateRef(ref, commitHash string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) Head() (string, error) { panic("unexpected") }
func (m *mockGitForHistory) HashObject(data []byte) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) ShowRef(pattern string) (string, error) { panic("unexpected") }
// Worktree & ref methods — unused in history domain
func (m *mockGitForHistory) AddWorktree(path, branch string) (string, error) { panic("unexpected") }
func (m *mockGitForHistory) RemoveWorktree(path string) error               { panic("unexpected") }
func (m *mockGitForHistory) CreateRef(ref, commitHash string) error         { panic("unexpected") }

func TestHandleHistory_Blame(t *testing.T) {
	gitMock := new(mockGitForHistory)
	lines := []domain.BlameLine{
		{Line: 1, Author: "John", Hash: "abc"},
	}
	gitMock.On("Blame", "main.go").Return(lines, nil)

	h := NewHandler(gitMock)
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{"command": "BLAME", "target_paths": "main.go"},
		},
	}

	res, err := h.HandleHistory(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Equal(t, "main.go", parsed["file"])
	gitMock.AssertExpectations(t)
}
