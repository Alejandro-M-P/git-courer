package git

import "github.com/Alejandro-M-P/git-courer/internal/domain/models"

// Port defines what git operations the core can do
// The core uses this interface, the adapter implements it
type Port interface {
	// Status returns the current repository status
	Status() (models.Status, error)

	// Diff returns the diff for all changes
	Diff() (string, error)

	// DiffStaged returns the diff for staged files only
	DiffStaged() (string, error)

	// Add stages files
	Add(paths []string) error

	// Commit creates a commit with the given message
	Commit(message string) (string, error)

	// Push pushes to the remote
	Push() (string, error)

	// Pull pulls from the remote
	Pull() (string, error)

	// PullRebase pulls from the remote with rebase
	PullRebase() (string, error)

	// Fetch fetches from the remote
	Fetch() (string, error)

	// Branch creates a new branch
	Branch(name string) (string, error)

	// Checkout switches to a branch
	Checkout(name string) (string, error)

	// CurrentBranch returns the current branch name
	CurrentBranch() (string, error)

	// Merge merges a branch into current
	Merge(branch string) (string, error)

	// Rebase rebases onto another branch
	Rebase(branch string) (string, error)

	// Stash saves current changes
	Stash() (string, error)

	// StashPop restores stashed changes
	StashPop() (string, error)

	// Reset resets to a commit
	Reset(mode string, commit string) (string, error)

	// Log returns commit history
	Log(limit int) (string, error)

	// Show returns details about a commit
	Show(commit string) (string, error)

	// Blame returns blame information
	Blame(file string) (string, error)

	// Clean removes untracked files
	Clean(directories bool) (string, error)

	// Revert creates a revert commit
	Revert(commit string) (string, error)

	// IsRepo checks if directory is a git repository
	IsRepo() bool
}
