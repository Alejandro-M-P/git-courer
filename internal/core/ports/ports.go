// Package ports defines the interfaces (inward-facing) that the core depends on.
// Adapters implement these interfaces; the core never imports adapters directly.
package ports

import "github.com/Alejandro-M-P/git-courer/internal/core/domain"

// Git defines the interface for git operations.
type Git interface {
	// Status returns the current repository status
	Status() (domain.Status, error)

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

	// ResetSoft resets the last n commits soft (keeps changes staged)
	ResetSoft(commits int) error

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

	// CherryPick cherry-picks a commit
	CherryPick(commit string) (string, error)

	// Tag creates a tag
	Tag(name string) (string, error)

	// DeleteBranch deletes a branch
	DeleteBranch(name string) (string, error)

	// Reflog returns the reflog
	Reflog(limit int) (string, error)

	// PushWithUpstream pushes and sets upstream for a branch
	PushWithUpstream(branch string) (string, error)

	// IsRepo checks if directory is a git repository
	IsRepo() bool
}

// LLM defines the interface for AI/LLM operations.
type LLM interface {
	GenerateChunkMessage(chunk domain.DiffChunk) (string, error)
	DecideCommit(instruction, gitStatus, untracked, modified, deleted string) (domain.CommitIntent, error)
	SetRetryContext(previousMessage string)
	ClearRetryContext()
	IsAvailable() bool
}

// DiffChunker defines the interface for splitting a large diff
// into smaller, LLM-friendly chunks based on file structure.
type DiffChunker interface {
	// Chunk splits a unified diff into logical chunks.
	// Each chunk is small enough to fit in an LLM context window.
	// maxChunkSize is the maximum size in characters per chunk.
	Chunk(diff string, maxChunkSize int) ([]domain.DiffChunk, error)
}

// UI defines the interface for user interaction (confirmations, previews).
type UI interface {
	Confirm(message string) (bool, error)
	Edit(initial string) (string, error)
	ShowError(message string)
	ShowInfo(message string)
	Select(options []string) (int, error)
}
