package mcp

import (
	"time"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/stretchr/testify/mock"
)

type MockGit struct {
	mock.Mock
}

func (m *MockGit) Status() (domain.Status, error) {
	args := m.Called()
	return args.Get(0).(domain.Status), args.Error(1)
}

func (m *MockGit) Diff(paths ...string) (string, error) {
	args := m.Called(paths)
	return args.String(0), args.Error(1)
}

func (m *MockGit) DiffStat(paths ...string) (string, error) {
	args := m.Called(paths)
	return args.String(0), args.Error(1)
}

func (m *MockGit) DiffStatStaged(paths ...string) (string, error) {
	args := m.Called(paths)
	return args.String(0), args.Error(1)
}

func (m *MockGit) DiffAll(paths ...string) (string, error) {
	args := m.Called(paths)
	return args.String(0), args.Error(1)
}

func (m *MockGit) DiffRange(base, target, mode string, paths ...string) (string, error) {
	args := m.Called(base, target, mode, paths)
	return args.String(0), args.Error(1)
}

func (m *MockGit) DiffStaged(paths ...string) (string, error) {
	args := m.Called(paths)
	return args.String(0), args.Error(1)
}

func (m *MockGit) ListUntracked() ([]string, error) {
	args := m.Called()
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockGit) Log(limit int, pattern string, paths ...string) (string, error) {
	args := m.Called(limit, pattern, paths)
	return args.String(0), args.Error(1)
}

func (m *MockGit) LogFull(limit int) (string, error) {
	args := m.Called(limit)
	return args.String(0), args.Error(1)
}

func (m *MockGit) CurrentBranch() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockGit) ListBranches(pattern ...string) (string, error) {
	args := m.Called(pattern)
	return args.String(0), args.Error(1)
}

func (m *MockGit) ListTags(pattern ...string) ([]string, error) {
	args := m.Called(pattern)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockGit) IsRepo() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockGit) RemoteURL() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockGit) RemoteInfo() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockGit) Search(pattern string, context, before, after int, paths ...string) (string, error) {
	args := m.Called(pattern, context, before, after, paths)
	return args.String(0), args.Error(1)
}

func (m *MockGit) CatFile(revision, path string) (string, error) {
	args := m.Called(revision, path)
	return args.String(0), args.Error(1)
}

func (m *MockGit) ListTree(revision, path string, recursive bool) ([]string, error) {
	args := m.Called(revision, path, recursive)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockGit) LatestTag() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockGit) CommitsFromTag(sinceTag string) (string, error) {
	args := m.Called(sinceTag)
	return args.String(0), args.Error(1)
}

func (m *MockGit) TagExists(name string) (bool, error) {
	args := m.Called(name)
	return args.Bool(0), args.Error(1)
}

func (m *MockGit) IsGHAuthenticated() (bool, error) {
	args := m.Called()
	return args.Bool(0), args.Error(1)
}

func (m *MockGit) CreateRelease(tagName, changelog string) (string, error) {
	args := m.Called(tagName, changelog)
	return args.String(0), args.Error(1)
}

func (m *MockGit) Blame(filepath string) ([]domain.BlameLine, error) {
	args := m.Called(filepath)
	return args.Get(0).([]domain.BlameLine), args.Error(1)
}

func (m *MockGit) Show(hash string) (domain.ShowResult, error) {
	args := m.Called(hash)
	return args.Get(0).(domain.ShowResult), args.Error(1)
}

func (m *MockGit) Reflog() ([]domain.ReflogEntry, error) {
	args := m.Called()
	return args.Get(0).([]domain.ReflogEntry), args.Error(1)
}

func (m *MockGit) StashList() ([]domain.StashEntry, error) {
	args := m.Called()
	return args.Get(0).([]domain.StashEntry), args.Error(1)
}

func (m *MockGit) StashDiff(index string) (string, error) {
	args := m.Called(index)
	return args.String(0), args.Error(1)
}

func (m *MockGit) StashShow() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockGit) MergeBase(a, b string) (string, error) {
	args := m.Called(a, b)
	return args.String(0), args.Error(1)
}

func (m *MockGit) LogRange(from, to string) (string, error) {
	args := m.Called(from, to)
	return args.String(0), args.Error(1)
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

func (m *MockGit) ListBackups() ([]domain.Backup, error) {
	args := m.Called()
	return args.Get(0).([]domain.Backup), args.Error(1)
}

func (m *MockGit) PruneBackups(olderThan time.Duration) error {
	args := m.Called(olderThan)
	return args.Error(0)
}

func (m *MockGit) Add(paths []string) error {
	args := m.Called(paths)
	return args.Error(0)
}

func (m *MockGit) Remove(paths []string) error {
	args := m.Called(paths)
	return args.Error(0)
}

func (m *MockGit) Commit(message string) (string, error) {
	args := m.Called(message)
	return args.String(0), args.Error(1)
}

func (m *MockGit) Push() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockGit) PushTo(remoteBranch string) (string, error) {
	args := m.Called(remoteBranch)
	return args.String(0), args.Error(1)
}

func (m *MockGit) Pull() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockGit) PullFrom(remoteBranch string) (string, error) {
	args := m.Called(remoteBranch)
	return args.String(0), args.Error(1)
}

func (m *MockGit) Fetch() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockGit) Stash(message ...string) (string, error) {
	args := m.Called(message)
	return args.String(0), args.Error(1)
}

func (m *MockGit) StashWithUntracked(message string) (string, error) {
	args := m.Called(message)
	return args.String(0), args.Error(1)
}

func (m *MockGit) StashPop() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockGit) StashApply(index string) (string, error) {
	args := m.Called(index)
	return args.String(0), args.Error(1)
}

func (m *MockGit) StashDrop(index string) (string, error) {
	args := m.Called(index)
	return args.String(0), args.Error(1)
}

func (m *MockGit) StashClear() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockGit) Switch(branch string) error {
	args := m.Called(branch)
	return args.Error(0)
}

func (m *MockGit) Branch(name string) (string, error) {
	args := m.Called(name)
	return args.String(0), args.Error(1)
}

func (m *MockGit) DeleteBranch(name string, force bool) (string, error) {
	args := m.Called(name, force)
	return args.String(0), args.Error(1)
}

func (m *MockGit) RenameBranch(oldName, newName string) (string, error) {
	args := m.Called(oldName, newName)
	return args.String(0), args.Error(1)
}

func (m *MockGit) DeleteRemoteBranch(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockGit) Tag(name, message string) (string, error) {
	args := m.Called(name, message)
	return args.String(0), args.Error(1)
}

func (m *MockGit) PushTag(name string) (string, error) {
	args := m.Called(name)
	return args.String(0), args.Error(1)
}

func (m *MockGit) DeleteTag(name string) (string, error) {
	args := m.Called(name)
	return args.String(0), args.Error(1)
}

func (m *MockGit) DeleteTagRemote(name string) (string, error) {
	args := m.Called(name)
	return args.String(0), args.Error(1)
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

func (m *MockGit) Reset(mode string, commit string) (string, error) {
	args := m.Called(mode, commit)
	return args.String(0), args.Error(1)
}

func (m *MockGit) ResetSoft(ref string) error {
	args := m.Called(ref)
	return args.Error(0)
}

func (m *MockGit) Restore(paths []string) error {
	args := m.Called(paths)
	return args.Error(0)
}

func (m *MockGit) Clean() error {
	args := m.Called()
	return args.Error(0)
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

func (m *MockGit) PushToBranch(remote, branch string) (string, error) {
	args := m.Called(remote, branch)
	return args.String(0), args.Error(1)
}

func (m *MockGit) PullFromBranch(remote, branch string) (string, error) {
	args := m.Called(remote, branch)
	return args.String(0), args.Error(1)
}

func (m *MockGit) CherryPick(commit string) (string, error) {
	args := m.Called(commit)
	return args.String(0), args.Error(1)
}

func (m *MockGit) SetUpstream(branch, remote string) (string, error) {
	args := m.Called(branch, remote)
	return args.String(0), args.Error(1)
}

func (m *MockGit) UnsetUpstream(branch string) (string, error) {
	args := m.Called(branch)
	return args.String(0), args.Error(1)
}

func (m *MockGit) Revert(commit string) (string, error) {
	args := m.Called(commit)
	return args.String(0), args.Error(1)
}

func (m *MockGit) Amend(message string, paths []string) (string, error) {
	args := m.Called(message, paths)
	return args.String(0), args.Error(1)
}

func (m *MockGit) ShowCommit(commit string) (string, error) {
	args := m.Called(commit)
	return args.String(0), args.Error(1)
}

func (m *MockGit) RemoteAdd(name, url string) (string, error) {
	args := m.Called(name, url)
	return args.String(0), args.Error(1)
}

func (m *MockGit) RemoteRemove(name string) (string, error) {
	args := m.Called(name)
	return args.String(0), args.Error(1)
}

func (m *MockGit) ConfigGet(key string) (string, error) {
	args := m.Called(key)
	return args.String(0), args.Error(1)
}

func (m *MockGit) ConfigSet(key, value string) (string, error) {
	args := m.Called(key, value)
	return args.String(0), args.Error(1)
}

func (m *MockGit) SymbolicRef(ref string) (string, error) {
	args := m.Called(ref)
	return args.String(0), args.Error(1)
}

func (m *MockGit) WriteTree() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockGit) CommitTree(treeHash, parentHash, message string) (string, error) {
	args := m.Called(treeHash, parentHash, message)
	return args.String(0), args.Error(1)
}

func (m *MockGit) UpdateRef(ref, commitHash string) (string, error) {
	args := m.Called(ref, commitHash)
	return args.String(0), args.Error(1)
}

func (m *MockGit) Head() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
