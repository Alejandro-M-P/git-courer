package git

import (
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// GitReadAdapter implements ports.GitReadPort
type GitReadAdapter struct {
	exec *ExecAdapter
}

// NewGitReadAdapter creates a new GitReadAdapter
func NewGitReadAdapter(workDir string) *GitReadAdapter {
	return &GitReadAdapter{exec: NewExecAdapter(workDir)}
}

// Status returns the current repository status
func (a *GitReadAdapter) Status() (domain.Status, error) {
	return a.exec.Status()
}

// Diff returns the diff for all changes
func (a *GitReadAdapter) Diff() (string, error) {
	return a.exec.Diff()
}

// DiffStaged returns the diff for staged files only
func (a *GitReadAdapter) DiffStaged() (string, error) {
	return a.exec.DiffStaged()
}

// Log returns commit history
func (a *GitReadAdapter) Log(limit int) (string, error) {
	return a.exec.Log(limit)
}

// CurrentBranch returns the current branch name
func (a *GitReadAdapter) CurrentBranch() (string, error) {
	return a.exec.CurrentBranch()
}

// Ensure GitReadAdapter implements ports.GitReadPort
var _ ports.GitReadPort = (*GitReadAdapter)(nil)
