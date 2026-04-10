// Package ports defines the interfaces (inward-facing) that the core depends on.
// Adapters implement these interfaces; the core never imports adapters directly.
package ports

import "github.com/Alejandro-M-P/git-courer/internal/core/domain"

// Git is the unified interface for all git operations.
// Direct ops (add, push, pull…) are called by handlers without LLM.
// Workflow ops (commit, merge, branch…) are routed through the workflow engine.
type Git interface {
	// --- Read ---
	Status() (domain.Status, error)
	Diff() (string, error)
	DiffStaged() (string, error)
	ListUntracked() ([]string, error)
	Log(limit int) (string, error)
	Show(commit string) (string, error)
	Blame(file string) (string, error)
	Reflog(limit int) (string, error)
	CurrentBranch() (string, error)
	ListBranches() (string, error)
	ListTags() (string, error)
	IsRepo() bool

	// --- Write · Direct (no LLM needed) ---
	Add(paths []string) error
	Remove(paths []string) error
	Checkout(name string) (string, error)
	Switch(name string) error
	Push() (string, error)
	PushWithUpstream(branch string) (string, error)
	Pull() (string, error)
	PullRebase() (string, error)
	Fetch() (string, error)
	Stash() (string, error)
	StashPop() (string, error)
	ResetSoft(commits int) error

	// --- Write · Workflow (LLM + optional confirm) ---
	Commit(message string) (string, error)
	Branch(name string) (string, error)
	DeleteBranch(name string) (string, error)
	RenameBranch(oldName, newName string) (string, error)
	Merge(branch string) (string, error)
	Rebase(branch string) (string, error)
	RebaseContinue() (string, error)
	RebaseAbort() (string, error)
	Reset(mode string, commit string) (string, error)
	CherryPick(commit string) (string, error)
	Revert(commit string) (string, error)
	Clean(directories bool) (string, error)
	Tag(name string) (string, error)
	DeleteTag(name string) (string, error)
	AddRemote(name, url string) (string, error)
	RemoveRemote(name string) (string, error)
	Init() (string, error)
	Clone(url string) (string, error)
}
