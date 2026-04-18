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
	Diff(paths ...string) (string, error)
	DiffStaged(paths ...string) (string, error)
	ListUntracked() ([]string, error)
	Log(limit int, paths ...string) (string, error)
	LogFull(limit int) (string, error)
	CurrentBranch() (string, error)
	ListBranches(pattern ...string) (string, error)
	ListTags(pattern ...string) ([]string, error)
	IsRepo() bool

	// --- GitHub CLI Integration ---
	LatestTag() (string, error)
	CommitsFromTag(sinceTag string) (string, error)
	TagExists(name string) (bool, error)
	IsGHAuthenticated() (bool, error)
	CreateRelease(name, changelog string) (string, error)

	// --- Backup ---
	CreateBackup(operation string, keepIndex bool) (domain.Backup, error)
	RestoreBackup(backup domain.Backup) error
	DeleteBackup(backup domain.Backup) error

	// --- Write · Direct (no LLM needed) ---
	Add(paths []string) error
	Remove(paths []string) error
	Checkout(name string) (string, error)
	Switch(name string) error
	Push() (string, error)
	Pull() (string, error)
	Fetch() (string, error)
	Stash() (string, error)
	StashPop() (string, error)

	// --- Write · Workflow (LLM + optional confirm) ---
	Commit(message string) (string, error)
	Branch(name string) (string, error)
	DeleteBranch(name string) (string, error)
	Reset(mode string, commit string) (string, error)
	Merge(branch string) (string, error)
	Tag(name string) (string, error)
}
