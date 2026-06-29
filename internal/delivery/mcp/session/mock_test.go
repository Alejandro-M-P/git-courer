package session

import (
	"time"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/stretchr/testify/mock"
)

// MockGit is a testify-based mock implementing ports.Git for session tests.
// It mirrors internal/delivery/mcp/branch/mock_test.go: methods relevant to
// the session domain use m.Called so tests can set expectations; the rest are
// no-op stubs that satisfy the interface without interfering with session logic.
type MockGit struct {
	mock.Mock
}

// --- Session-relevant methods (testify-mocked) ---

func (m *MockGit) Head() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockGit) CreateRef(ref, commitHash string) error {
	args := m.Called(ref, commitHash)
	return args.Error(0)
}

func (m *MockGit) AddWorktree(path, branch string) (string, error) {
	args := m.Called(path, branch)
	return args.String(0), args.Error(1)
}

func (m *MockGit) RemoveWorktree(path string) error {
	args := m.Called(path)
	return args.Error(0)
}

func (m *MockGit) UpdateRef(ref, commitHash string) (string, error) {
	args := m.Called(ref, commitHash)
	return args.String(0), args.Error(1)
}

func (m *MockGit) ShowRef(pattern string) (string, error) {
	args := m.Called(pattern)
	return args.String(0), args.Error(1)
}

func (m *MockGit) GitCommonDir() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

// --- Read ---

func (m *MockGit) Status() (domain.Status, error) {
	args := m.Called()
	return args.Get(0).(domain.Status), args.Error(1)
}

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
func (m *MockGit) LogRange(from, to string) (string, error)                      { return "", nil }
func (m *MockGit) CurrentBranch() (string, error)                                 { return "", nil }
func (m *MockGit) ListBranches(pattern ...string) (string, error) {
	if len(pattern) == 0 {
		args := m.Called()
		return args.String(0), args.Error(1)
	}
	args := m.Called(pattern[0])
	return args.String(0), args.Error(1)
}
func (m *MockGit) ListTags(pattern ...string) ([]string, error) { return nil, nil }
func (m *MockGit) IsRepo() bool                                 { return true }
func (m *MockGit) RemoteURL() (string, error)                   { return "", nil }
func (m *MockGit) RemoteInfo() (string, error)                  { return "", nil }
func (m *MockGit) Search(pattern string, context, before, after int, paths ...string) (string, error) {
	return "", nil
}
func (m *MockGit) CatFile(revision, path string) (string, error)                    { return "", nil }
func (m *MockGit) ListTree(revision, path string, recursive bool) ([]string, error) { return nil, nil }
func (m *MockGit) LatestTag() (string, error)                                       { return "", nil }
func (m *MockGit) CommitsFromTag(sinceTag string) (string, error)                   { return "", nil }
func (m *MockGit) TagExists(name string) (bool, error)                              { return false, nil }
func (m *MockGit) IsGHAuthenticated() (bool, error)                                 { return false, nil }
func (m *MockGit) CreateRelease(tagName, changelog string) (string, error)         { return "", nil }
func (m *MockGit) Blame(filepath string) ([]domain.BlameLine, error)               { return nil, nil }
func (m *MockGit) Show(hash string) (domain.ShowResult, error)                     { return domain.ShowResult{}, nil }
func (m *MockGit) Reflog() ([]domain.ReflogEntry, error)                            { return nil, nil }
func (m *MockGit) StashList() ([]domain.StashEntry, error)                          { return nil, nil }
func (m *MockGit) StashDiff(index string) (string, error)                           { return "", nil }
func (m *MockGit) StashShow() (string, error)                                       { return "", nil }
func (m *MockGit) MergeBase(a, b string) (string, error) {
	args := m.Called(a, b)
	return args.String(0), args.Error(1)
}

// --- Backup ---

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
func (m *MockGit) ListBackups() ([]domain.Backup, error)      { return nil, nil }
func (m *MockGit) PruneBackups(olderThan time.Duration) error { return nil }

// --- Write ---

func (m *MockGit) Add(paths []string) error                             { return nil }
func (m *MockGit) Remove(paths []string) error                          { return nil }
func (m *MockGit) Commit(message string) (string, error)                { return "", nil }
func (m *MockGit) Push() (string, error)                                { return "", nil }
func (m *MockGit) PushTo(remoteBranch string) (string, error)           { return "", nil }
func (m *MockGit) Pull() (string, error)                                { return "", nil }
func (m *MockGit) PullFrom(remoteBranch string) (string, error)         { return "", nil }
func (m *MockGit) Fetch() (string, error)                               { return "", nil }
func (m *MockGit) Stash(message ...string) (string, error) {
	args := m.Called(message)
	return args.String(0), args.Error(1)
}
func (m *MockGit) StashWithUntracked(message string) (string, error) { return "", nil }
func (m *MockGit) StashPop() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *MockGit) StashApply(index string) (string, error)              { return "", nil }
func (m *MockGit) StashDrop(index string) (string, error)               { return "", nil }
func (m *MockGit) StashClear() (string, error)                          { return "", nil }
func (m *MockGit) Switch(branch string) error {
	args := m.Called(branch)
	return args.Error(0)
}
func (m *MockGit) Branch(name string) (string, error)                   { return "", nil }
func (m *MockGit) BranchFrom(name, from string) (string, error)         { return "", nil }
func (m *MockGit) DeleteBranch(name string, force bool) (string, error) {
	args := m.Called(name, force)
	return args.String(0), args.Error(1)
}
func (m *MockGit) RenameBranch(oldName, newName string) (string, error) { return "", nil }
func (m *MockGit) DeleteRemoteBranch(name string) error                 { return nil }
func (m *MockGit) Tag(name, message string) (string, error)             { return "", nil }
func (m *MockGit) TagFromFile(name, path string) (string, error)        { return "", nil }
func (m *MockGit) PushTag(name string) (string, error)                  { return "", nil }
func (m *MockGit) DeleteTag(name string) (string, error)               { return "", nil }
func (m *MockGit) DeleteTagRemote(name string) (string, error)          { return "", nil }
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
func (m *MockGit) ResetSoft(ref string) error                           { return nil }
func (m *MockGit) Restore(paths []string) error                         { return nil }
func (m *MockGit) Clean() error                                         { return nil }
func (m *MockGit) Rebase(branch string) (string, error)                 { return "", nil }
func (m *MockGit) RebaseAbort() (string, error)                         { return "", nil }
func (m *MockGit) RebaseContinue() (string, error)                      { return "", nil }
func (m *MockGit) RebaseSkip() (string, error)                          { return "", nil }
func (m *MockGit) RebaseOnto(newBase, upstream, branch string) (string, error) {
	return "", nil
}
func (m *MockGit) PushToBranch(remote, branch string) (string, error)   { return "", nil }
func (m *MockGit) PullFromBranch(remote, branch string) (string, error) { return "", nil }
func (m *MockGit) CherryPick(commit string) (string, error)             { return "", nil }
func (m *MockGit) Revert(commit string) (string, error)                 { return "", nil }
func (m *MockGit) Amend(message string, paths []string) (string, error) { return "", nil }
func (m *MockGit) ShowCommit(commit string) (string, error)             { return "", nil }
func (m *MockGit) RemoteAdd(name, url string) (string, error)          { return "", nil }
func (m *MockGit) RemoteRemove(name string) (string, error)            { return "", nil }
func (m *MockGit) SetUpstream(branch, remote string) (string, error)   { return "", nil }
func (m *MockGit) UnsetUpstream(branch string) (string, error)          { return "", nil }

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

// --- Plumbing ---

func (m *MockGit) WriteTree() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}
func (m *MockGit) CommitTree(treeHash, parentHash, message string) (string, error) {
	args := m.Called(treeHash, parentHash, message)
	return args.String(0), args.Error(1)
}
func (m *MockGit) HashObject(data []byte) (string, error) {
	args := m.Called(data)
	return args.String(0), args.Error(1)
}