package sync

import (
	"time"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/stretchr/testify/mock"
)

type MockGit struct {
	mock.Mock
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

func (m *MockGit) Fetch() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockGit) PullFrom(remoteBranch string) (string, error) {
	args := m.Called(remoteBranch)
	return args.String(0), args.Error(1)
}

func (m *MockGit) PushTo(remoteBranch string) (string, error) {
	args := m.Called(remoteBranch)
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

// Required to satisfy ports.Git interface but unused in sync domain
func (m *MockGit) Status() (domain.Status, error)                 { return domain.Status{}, nil }
func (m *MockGit) Diff(paths ...string) (string, error)           { return "", nil }
func (m *MockGit) DiffStat(paths ...string) (string, error)       { return "", nil }
func (m *MockGit) DiffStatStaged(paths ...string) (string, error) { return "", nil }
func (m *MockGit) DiffAll(paths ...string) (string, error)        { return "", nil }
func (m *MockGit) DiffUntracked() (string, error)                 { return "", nil }
func (m *MockGit) DiffStaged(paths ...string) (string, error)     { return "", nil }
func (m *MockGit) DiffRange(base, target, mode string, paths ...string) (string, error) {
	return "", nil
}
func (m *MockGit) ListUntracked() ([]string, error)                               { return nil, nil }
func (m *MockGit) Log(limit int, pattern string, paths ...string) (string, error) { return "", nil }
func (m *MockGit) LogFull(limit int) (string, error)                              { return "", nil }
func (m *MockGit) CurrentBranch() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *MockGit) ListBranches(pattern ...string) (string, error)                 { return "", nil }
func (m *MockGit) ListTags(pattern ...string) ([]string, error)                   { return nil, nil }
func (m *MockGit) IsRepo() bool                                                   { return true }
func (m *MockGit) RemoteURL() (string, error)                                     { return "", nil }
func (m *MockGit) RemoteInfo() (string, error)                                    { return "", nil }
func (m *MockGit) Search(pattern string, context, before, after int, paths ...string) (string, error) {
	return "", nil
}
func (m *MockGit) CatFile(revision, path string) (string, error)                    { return "", nil }
func (m *MockGit) ListTree(revision, path string, recursive bool) ([]string, error) { return nil, nil }
func (m *MockGit) LatestTag() (string, error)                                       { return "", nil }
func (m *MockGit) CommitsFromTag(sinceTag string) (string, error)                   { return "", nil }
func (m *MockGit) TagExists(name string) (bool, error)                              { return false, nil }
func (m *MockGit) IsGHAuthenticated() (bool, error)                                 { return false, nil }
func (m *MockGit) CreateRelease(tagName, changelog string) (string, error)          { return "", nil }
func (m *MockGit) Blame(filepath string) ([]domain.BlameLine, error)                { return nil, nil }
func (m *MockGit) Show(hash string) (domain.ShowResult, error)                      { return domain.ShowResult{}, nil }
func (m *MockGit) Reflog() ([]domain.ReflogEntry, error)                            { return nil, nil }
func (m *MockGit) StashList() ([]domain.StashEntry, error)                          { return nil, nil }
func (m *MockGit) StashDiff(index string) (string, error)                           { return "", nil }
func (m *MockGit) StashShow() (string, error)                                       { return "", nil }
func (m *MockGit) MergeBase(a, b string) (string, error)                            { return "", nil }
func (m *MockGit) LogRange(from, to string) (string, error)                         { return "", nil }

func (m *MockGit) ListBackups() ([]domain.Backup, error)                       { return nil, nil }
func (m *MockGit) PruneBackups(olderThan time.Duration) error                  { return nil }
func (m *MockGit) Add(paths []string) error                                    { return nil }
func (m *MockGit) Remove(paths []string) error                                 { return nil }
func (m *MockGit) Commit(message string) (string, error)                       { return "", nil }
func (m *MockGit) Push() (string, error)                                       { return "", nil }
func (m *MockGit) Pull() (string, error)                                       { return "", nil }
func (m *MockGit) Stash(message ...string) (string, error)                     { return "", nil }
func (m *MockGit) StashWithUntracked(message string) (string, error)           { return "", nil }
func (m *MockGit) StashPop() (string, error)                                   { return "", nil }
func (m *MockGit) StashApply(index string) (string, error)                     { return "", nil }
func (m *MockGit) StashDrop(index string) (string, error)                      { return "", nil }
func (m *MockGit) StashClear() (string, error)                                 { return "", nil }
func (m *MockGit) Reset(mode string, commit string) (string, error)            { return "", nil }
func (m *MockGit) ResetSoft(ref string) error                                  { return nil }
func (m *MockGit) Restore(paths []string) error                                { return nil }
func (m *MockGit) Clean() error                                                { return nil }
func (m *MockGit) Switch(branch string) error                                  { return nil }
func (m *MockGit) Merge(branch string) (string, error)                         { return "", nil }
func (m *MockGit) MergeAbort() (string, error)                                 { return "", nil }
func (m *MockGit) MergeContinue() (string, error)                              { return "", nil }
func (m *MockGit) MergeSkip() (string, error)                                  { return "", nil }
func (m *MockGit) Rebase(branch string) (string, error)                        { return "", nil }
func (m *MockGit) RebaseAbort() (string, error)                                { return "", nil }
func (m *MockGit) RebaseContinue() (string, error)                             { return "", nil }
func (m *MockGit) RebaseSkip() (string, error)                                 { return "", nil }
func (m *MockGit) RebaseOnto(newBase, upstream, branch string) (string, error) { return "", nil }
func (m *MockGit) PushToBranch(remote, branch string) (string, error) {
	args := m.Called(remote, branch)
	return args.String(0), args.Error(1)
}

func (m *MockGit) PullFromBranch(remote, branch string) (string, error) {
	args := m.Called(remote, branch)
	return args.String(0), args.Error(1)
}
func (m *MockGit) Branch(name string) (string, error)                   { return "", nil }
func (m *MockGit) DeleteBranch(name string, force bool) (string, error) { return "", nil }
func (m *MockGit) RenameBranch(oldName, newName string) (string, error) { return "", nil }
func (m *MockGit) DeleteRemoteBranch(name string) error                 { return nil }
func (m *MockGit) Tag(name, message string) (string, error)             { return "", nil }
func (m *MockGit) TagFromFile(name, path string) (string, error)       { return "", nil }
func (m *MockGit) PushTag(name string) (string, error)                  { return "", nil }
func (m *MockGit) DeleteTag(name string) (string, error)                { return "", nil }
func (m *MockGit) DeleteTagRemote(name string) (string, error)          { return "", nil }
func (m *MockGit) CherryPick(commit string) (string, error)             { return "", nil }
func (m *MockGit) SetUpstream(branch, remote string) (string, error)    { return "", nil }
func (m *MockGit) UnsetUpstream(branch string) (string, error)          { return "", nil }
func (m *MockGit) Revert(commit string) (string, error)                 { return "", nil }
func (m *MockGit) Amend(message string, paths []string) (string, error) { return "", nil }
func (m *MockGit) ShowCommit(commit string) (string, error)             { return "", nil }
func (m *MockGit) ConfigGet(key string) (string, error)                 { return "", nil }
func (m *MockGit) ConfigSet(key, value string) (string, error)          { return "", nil }
func (m *MockGit) SymbolicRef(ref string) (string, error)               { return "", nil }
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
func (m *MockGit) HashObject(data []byte) (string, error) {
	args := m.Called(data)
	return args.String(0), args.Error(1)
}
func (m *MockGit) ShowRef(pattern string) (string, error) {
	args := m.Called(pattern)
	return args.String(0), args.Error(1)
}
// Worktree & ref methods — unused in sync domain
func (m *MockGit) AddWorktree(path, branch string) (string, error) { return "", nil }
func (m *MockGit) RemoveWorktree(path string) error               { return nil }
func (m *MockGit) CreateRef(ref, commitHash string) error         { return nil }
