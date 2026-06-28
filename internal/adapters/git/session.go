package git

import (
	"sync/atomic"
	"time"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
)

// Compile-time assertion that sessionGit satisfies ports.Git.
var _ ports.Git = (*sessionGit)(nil)

// sessionGit wraps a base ports.Git adapter and redirects ordinary git
// operations to the active session's worktree when one is selected. Operations
// that manage worktrees themselves (AddWorktree, RemoveWorktree, CreateRef)
// always target the main repo via mainGit, regardless of the active session.
//
// The active session is read from a shared *atomic.Value that may hold either
// nil (no active session) or a *domain.Session. Reads are lock-free and safe
// for concurrent MCP connections.
type sessionGit struct {
	base          ports.Git
	mainGit       ports.Git
	activeSession *atomic.Value
}

// newSessionGit constructs a session-aware wrapper. base receives all
// non-worktree-management operations; mainGit is used exclusively for
// AddWorktree/RemoveWorktree/CreateRef.
func newSessionGit(base, mainGit ports.Git, activeSession *atomic.Value) *sessionGit {
	return &sessionGit{
		base:          base,
		mainGit:       mainGit,
		activeSession: activeSession,
	}
}

// NewSessionWrapper is the exported constructor used by wiring code outside
// the git package (e.g. internal/delivery/mcp/handlers.go). It returns a
// ports.Git that redirects to the active session's worktree.
func NewSessionWrapper(base, mainGit ports.Git, activeSession *atomic.Value) ports.Git {
	return newSessionGit(base, mainGit, activeSession)
}

// --- Read ---

func (s *sessionGit) Status() (domain.Status, error)           { return s.base.Status() }
func (s *sessionGit) Diff(paths ...string) (string, error)     { return s.base.Diff(paths...) }
func (s *sessionGit) DiffStat(paths ...string) (string, error) { return s.base.DiffStat(paths...) }
func (s *sessionGit) DiffStatStaged(paths ...string) (string, error) {
	return s.base.DiffStatStaged(paths...)
}
func (s *sessionGit) DiffAll(paths ...string) (string, error) { return s.base.DiffAll(paths...) }
func (s *sessionGit) DiffRange(base, target, mode string, paths ...string) (string, error) {
	return s.base.DiffRange(base, target, mode, paths...)
}
func (s *sessionGit) DiffUntracked() (string, error) { return s.base.DiffUntracked() }
func (s *sessionGit) DiffStaged(paths ...string) (string, error) {
	return s.base.DiffStaged(paths...)
}
func (s *sessionGit) ListUntracked() ([]string, error) { return s.base.ListUntracked() }
func (s *sessionGit) Log(limit int, pattern string, paths ...string) (string, error) {
	return s.base.Log(limit, pattern, paths...)
}
func (s *sessionGit) LogRange(from, to string) (string, error) { return s.base.LogRange(from, to) }
func (s *sessionGit) LogFull(limit int) (string, error)        { return s.base.LogFull(limit) }
func (s *sessionGit) CurrentBranch() (string, error)           { return s.base.CurrentBranch() }
func (s *sessionGit) ListBranches(pattern ...string) (string, error) {
	return s.base.ListBranches(pattern...)
}
func (s *sessionGit) ListTags(pattern ...string) ([]string, error) {
	return s.base.ListTags(pattern...)
}
func (s *sessionGit) IsRepo() bool                { return s.base.IsRepo() }
func (s *sessionGit) RemoteURL() (string, error)  { return s.base.RemoteURL() }
func (s *sessionGit) RemoteInfo() (string, error) { return s.base.RemoteInfo() }
func (s *sessionGit) Search(pattern string, context, before, after int, paths ...string) (string, error) {
	return s.base.Search(pattern, context, before, after, paths...)
}
func (s *sessionGit) CatFile(revision, path string) (string, error) {
	return s.base.CatFile(revision, path)
}
func (s *sessionGit) ListTree(revision, path string, recursive bool) ([]string, error) {
	return s.base.ListTree(revision, path, recursive)
}

// --- GitHub CLI Integration ---

func (s *sessionGit) LatestTag() (string, error) { return s.base.LatestTag() }
func (s *sessionGit) CommitsFromTag(sinceTag string) (string, error) {
	return s.base.CommitsFromTag(sinceTag)
}
func (s *sessionGit) TagExists(name string) (bool, error) { return s.base.TagExists(name) }
func (s *sessionGit) IsGHAuthenticated() (bool, error)    { return s.base.IsGHAuthenticated() }
func (s *sessionGit) CreateRelease(tagName, changelog string) (string, error) {
	return s.base.CreateRelease(tagName, changelog)
}

// --- Read · Advanced ---

func (s *sessionGit) Blame(filepath string) ([]domain.BlameLine, error) {
	return s.base.Blame(filepath)
}
func (s *sessionGit) Show(hash string) (domain.ShowResult, error) { return s.base.Show(hash) }
func (s *sessionGit) Reflog() ([]domain.ReflogEntry, error)       { return s.base.Reflog() }
func (s *sessionGit) StashList() ([]domain.StashEntry, error)     { return s.base.StashList() }
func (s *sessionGit) StashDiff(index string) (string, error)      { return s.base.StashDiff(index) }
func (s *sessionGit) StashShow() (string, error)                  { return s.base.StashShow() }
func (s *sessionGit) MergeBase(a, b string) (string, error)       { return s.base.MergeBase(a, b) }

// --- Backup ---

func (s *sessionGit) CreateBackup(operation string, mode domain.StashMode) (domain.Backup, error) {
	return s.base.CreateBackup(operation, mode)
}
func (s *sessionGit) RestoreBackup(backup domain.Backup) error { return s.base.RestoreBackup(backup) }
func (s *sessionGit) DeleteBackup(backup domain.Backup) error  { return s.base.DeleteBackup(backup) }
func (s *sessionGit) ListBackups() ([]domain.Backup, error)    { return s.base.ListBackups() }
func (s *sessionGit) PruneBackups(olderThan time.Duration) error {
	return s.base.PruneBackups(olderThan)
}

// --- Write ---

func (s *sessionGit) Add(paths []string) error                   { return s.base.Add(paths) }
func (s *sessionGit) Remove(paths []string) error                { return s.base.Remove(paths) }
func (s *sessionGit) Commit(message string) (string, error)      { return s.base.Commit(message) }
func (s *sessionGit) Push() (string, error)                      { return s.base.Push() }
func (s *sessionGit) PushTo(remoteBranch string) (string, error) { return s.base.PushTo(remoteBranch) }
func (s *sessionGit) Pull() (string, error)                      { return s.base.Pull() }
func (s *sessionGit) PullFrom(remoteBranch string) (string, error) {
	return s.base.PullFrom(remoteBranch)
}
func (s *sessionGit) Fetch() (string, error)                  { return s.base.Fetch() }
func (s *sessionGit) Stash(message ...string) (string, error) { return s.base.Stash(message...) }
func (s *sessionGit) StashPop() (string, error)               { return s.base.StashPop() }
func (s *sessionGit) StashApply(index string) (string, error) { return s.base.StashApply(index) }
func (s *sessionGit) StashDrop(index string) (string, error)  { return s.base.StashDrop(index) }
func (s *sessionGit) StashClear() (string, error)             { return s.base.StashClear() }
func (s *sessionGit) Switch(branch string) error              { return s.base.Switch(branch) }
func (s *sessionGit) Branch(name string) (string, error)      { return s.base.Branch(name) }
func (s *sessionGit) DeleteBranch(name string, force bool) (string, error) {
	return s.base.DeleteBranch(name, force)
}
func (s *sessionGit) RenameBranch(oldName, newName string) (string, error) {
	return s.base.RenameBranch(oldName, newName)
}
func (s *sessionGit) DeleteRemoteBranch(name string) error { return s.base.DeleteRemoteBranch(name) }
func (s *sessionGit) Tag(name, message string) (string, error) {
	return s.base.Tag(name, message)
}
func (s *sessionGit) TagFromFile(name, path string) (string, error) {
	return s.base.TagFromFile(name, path)
}
func (s *sessionGit) PushTag(name string) (string, error) { return s.base.PushTag(name) }
func (s *sessionGit) DeleteTag(name string) (string, error) {
	return s.base.DeleteTag(name)
}
func (s *sessionGit) DeleteTagRemote(name string) (string, error) {
	return s.base.DeleteTagRemote(name)
}
func (s *sessionGit) Merge(branch string) (string, error) { return s.base.Merge(branch) }
func (s *sessionGit) MergeAbort() (string, error)         { return s.base.MergeAbort() }
func (s *sessionGit) MergeContinue() (string, error)      { return s.base.MergeContinue() }
func (s *sessionGit) MergeSkip() (string, error)          { return s.base.MergeSkip() }
func (s *sessionGit) Reset(mode string, commit string) (string, error) {
	return s.base.Reset(mode, commit)
}
func (s *sessionGit) ResetSoft(ref string) error           { return s.base.ResetSoft(ref) }
func (s *sessionGit) Restore(paths []string) error         { return s.base.Restore(paths) }
func (s *sessionGit) Clean() error                         { return s.base.Clean() }
func (s *sessionGit) Rebase(branch string) (string, error) { return s.base.Rebase(branch) }
func (s *sessionGit) RebaseAbort() (string, error)         { return s.base.RebaseAbort() }
func (s *sessionGit) RebaseContinue() (string, error)      { return s.base.RebaseContinue() }
func (s *sessionGit) RebaseSkip() (string, error)          { return s.base.RebaseSkip() }
func (s *sessionGit) RebaseOnto(newBase, upstream, branch string) (string, error) {
	return s.base.RebaseOnto(newBase, upstream, branch)
}
func (s *sessionGit) PushToBranch(remote, branch string) (string, error) {
	return s.base.PushToBranch(remote, branch)
}
func (s *sessionGit) PullFromBranch(remote, branch string) (string, error) {
	return s.base.PullFromBranch(remote, branch)
}
func (s *sessionGit) CherryPick(commit string) (string, error) {
	return s.base.CherryPick(commit)
}
func (s *sessionGit) Revert(commit string) (string, error) { return s.base.Revert(commit) }
func (s *sessionGit) Amend(message string, paths []string) (string, error) {
	return s.base.Amend(message, paths)
}
func (s *sessionGit) ShowCommit(commit string) (string, error) { return s.base.ShowCommit(commit) }
func (s *sessionGit) RemoteAdd(name, url string) (string, error) {
	return s.base.RemoteAdd(name, url)
}
func (s *sessionGit) RemoteRemove(name string) (string, error) { return s.base.RemoteRemove(name) }
func (s *sessionGit) SetUpstream(branch, remote string) (string, error) {
	return s.base.SetUpstream(branch, remote)
}
func (s *sessionGit) UnsetUpstream(branch string) (string, error) {
	return s.base.UnsetUpstream(branch)
}
func (s *sessionGit) StashWithUntracked(message string) (string, error) {
	return s.base.StashWithUntracked(message)
}
func (s *sessionGit) ConfigGet(key string) (string, error) { return s.base.ConfigGet(key) }
func (s *sessionGit) ConfigSet(key, value string) (string, error) {
	return s.base.ConfigSet(key, value)
}
func (s *sessionGit) SymbolicRef(ref string) (string, error) { return s.base.SymbolicRef(ref) }

// --- Write · Worktree & Refs ---
//
// Worktree-management operations ALWAYS target the main repo, never the active
// session's worktree. Creating/removing worktrees inside a session worktree
// would be nonsensical and would break session lifecycle.

func (s *sessionGit) AddWorktree(path, branch string) (string, error) {
	return s.mainGit.AddWorktree(path, branch)
}
func (s *sessionGit) RemoveWorktree(path string) error { return s.mainGit.RemoveWorktree(path) }
func (s *sessionGit) CreateRef(ref, commitHash string) error {
	return s.mainGit.CreateRef(ref, commitHash)
}

// --- Write · Plumbing ---

func (s *sessionGit) WriteTree() (string, error) { return s.base.WriteTree() }
func (s *sessionGit) CommitTree(treeHash, parentHash, message string) (string, error) {
	return s.base.CommitTree(treeHash, parentHash, message)
}
func (s *sessionGit) UpdateRef(ref, commitHash string) (string, error) {
	return s.base.UpdateRef(ref, commitHash)
}
func (s *sessionGit) Head() (string, error) { return s.base.Head() }

// --- Misc ---

func (s *sessionGit) HashObject(data []byte) (string, error) { return s.base.HashObject(data) }
func (s *sessionGit) ShowRef(pattern string) (string, error) { return s.base.ShowRef(pattern) }
func (s *sessionGit) GitCommonDir() (string, error)          { return s.base.GitCommonDir() }
