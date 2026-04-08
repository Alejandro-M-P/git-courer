package git

import (
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// GitWriteReviewAdapter implements ports.GitWriteReviewPort for operations requiring confirmation
type GitWriteReviewAdapter struct {
	exec *ExecAdapter
}

// NewGitWriteReviewAdapter creates a new GitWriteReviewAdapter
func NewGitWriteReviewAdapter(workDir string) *GitWriteReviewAdapter {
	return &GitWriteReviewAdapter{exec: NewExecAdapter(workDir)}
}

// CreateBranch creates a new branch
func (a *GitWriteReviewAdapter) CreateBranch(name string) (string, error) {
	return a.exec.Branch(name)
}

// DeleteBranch deletes a branch
func (a *GitWriteReviewAdapter) DeleteBranch(name string) (string, error) {
	return a.exec.DeleteBranch(name)
}

// RenameBranch renames a branch
func (a *GitWriteReviewAdapter) RenameBranch(oldName, newName string) (string, error) {
	return a.exec.RenameBranch(oldName, newName)
}

// CreateTag creates a tag
func (a *GitWriteReviewAdapter) CreateTag(name string) (string, error) {
	return a.exec.Tag(name)
}

// DeleteTag deletes a tag
func (a *GitWriteReviewAdapter) DeleteTag(name string) (string, error) {
	return a.exec.DeleteTag(name)
}

// Merge merges a branch
func (a *GitWriteReviewAdapter) Merge(branch string) (string, error) {
	return a.exec.Merge(branch)
}

// Rebase rebases onto a branch
func (a *GitWriteReviewAdapter) Rebase(branch string) (string, error) {
	return a.exec.Rebase(branch)
}

// RebaseContinue continues a rebase
func (a *GitWriteReviewAdapter) RebaseContinue() (string, error) {
	return a.exec.RebaseContinue()
}

// RebaseAbort aborts a rebase
func (a *GitWriteReviewAdapter) RebaseAbort() (string, error) {
	return a.exec.RebaseAbort()
}

// ResetSoft resets soft (undo commits, keeps changes staged)
func (a *GitWriteReviewAdapter) ResetSoft(commits int) error {
	return a.exec.ResetSoft(commits)
}

// ResetHard resets hard
func (a *GitWriteReviewAdapter) ResetHard(target string) (string, error) {
	return a.exec.Reset("hard", target)
}

// Clean removes untracked files
func (a *GitWriteReviewAdapter) Clean() (string, error) {
	return a.exec.Clean(true)
}

// AddRemote adds a remote
func (a *GitWriteReviewAdapter) AddRemote(name, url string) (string, error) {
	return a.exec.AddRemote(name, url)
}

// RemoveRemote removes a remote
func (a *GitWriteReviewAdapter) RemoveRemote(name string) (string, error) {
	return a.exec.RemoveRemote(name)
}

// CherryPick cherry-picks a commit
func (a *GitWriteReviewAdapter) CherryPick(commit string) (string, error) {
	return a.exec.CherryPick(commit)
}

// Revert reverts a commit
func (a *GitWriteReviewAdapter) Revert(commit string) (string, error) {
	return a.exec.Revert(commit)
}

// Init initializes a git repository
func (a *GitWriteReviewAdapter) Init() (string, error) {
	return a.exec.Init()
}

// Clone clones a repository
func (a *GitWriteReviewAdapter) Clone(url string) (string, error) {
	return a.exec.Clone(url)
}

// Ensure GitWriteReviewAdapter implements ports.GitWriteReviewPort
var _ ports.GitWriteReviewPort = (*GitWriteReviewAdapter)(nil)
