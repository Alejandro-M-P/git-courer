// Package ports defines the interfaces (inward-facing) that the core depends on.
// Adapters implement these interfaces; the core never imports adapters directly.
package ports

import (
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// Git is the unified interface for all git operations.
// Direct ops (add, push, pull…) are called by handlers without LLM.
// Workflow ops (commit, merge, branch…) are routed through the workflow engine.
type Git interface {
	// --- Read ---
	Status() (domain.Status, error)
	Diff(paths ...string) (string, error)
	DiffStat(paths ...string) (string, error)
	DiffStatStaged(paths ...string) (string, error)
	DiffAll(paths ...string) (string, error)
	DiffRange(base, target, mode string, paths ...string) (string, error)
	DiffStaged(paths ...string) (string, error)
	ListUntracked() ([]string, error)
	Log(limit int, pattern string, paths ...string) (string, error)
	LogFull(limit int) (string, error)
	CurrentBranch() (string, error)
	ListBranches(pattern ...string) (string, error)
	ListTags(pattern ...string) ([]string, error)
	IsRepo() bool
	RemoteURL() (string, error)
	RemoteInfo() (string, error)
	Search(pattern string, context, before, after int, paths ...string) (string, error)
	CatFile(revision, path string) (string, error)
	ListTree(revision, path string, recursive bool) ([]string, error)

	// --- GitHub CLI Integration ---
	LatestTag() (string, error)
	CommitsFromTag(sinceTag string) (string, error)
	TagExists(name string) (bool, error)
	IsGHAuthenticated() (bool, error)
	CreateRelease(tagName, changelog string) (string, error)

	// --- Read · Advanced ---
	Blame(filepath string) ([]domain.BlameLine, error)
	Show(hash string) (domain.ShowResult, error)
	Reflog() ([]domain.ReflogEntry, error)
	StashList() ([]domain.StashEntry, error)
	StashDiff(index string) (string, error)
	StashShow() (string, error)
	MergeBase(a, b string) (string, error)

	// --- Backup ---
	CreateBackup(operation string, mode domain.StashMode) (domain.Backup, error)
	RestoreBackup(backup domain.Backup) error
	DeleteBackup(backup domain.Backup) error
	ListBackups() ([]domain.Backup, error)
	PruneBackups(olderThan time.Duration) error

	// --- Write ---
	Add(paths []string) error
	Remove(paths []string) error
	Commit(message string) (string, error)
	Push() (string, error)
	PushTo(remoteBranch string) (string, error)
	Pull() (string, error)
	PullFrom(remoteBranch string) (string, error)
	Fetch() (string, error)
	Stash(message ...string) (string, error)
	StashPop() (string, error)
	StashApply(index string) (string, error)
	StashDrop(index string) (string, error)
	StashClear() (string, error)
	Switch(branch string) error
	Branch(name string) (string, error)
	DeleteBranch(name string, force bool) (string, error)
	RenameBranch(oldName, newName string) (string, error)
	DeleteRemoteBranch(name string) error
	Tag(name, message string) (string, error)
	PushTag(name string) (string, error)
	DeleteTag(name string) (string, error)
	DeleteTagRemote(name string) (string, error)
	Merge(branch string) (string, error)
	Reset(mode string, commit string) (string, error)
	ResetSoft(ref string) error
}
