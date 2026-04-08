package git

import (
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// GitWriteAdapter implements ports.GitWritePort for direct write operations
type GitWriteAdapter struct {
	exec *ExecAdapter
}

// NewGitWriteAdapter creates a new GitWriteAdapter
func NewGitWriteAdapter(workDir string) *GitWriteAdapter {
	return &GitWriteAdapter{exec: NewExecAdapter(workDir)}
}

// Add stages files
func (a *GitWriteAdapter) Add(paths []string) error {
	return a.exec.Add(paths)
}

// Checkout switches to a branch
func (a *GitWriteAdapter) Checkout(branch string) error {
	_, err := a.exec.Checkout(branch)
	return err
}

// Switch switches to a branch
func (a *GitWriteAdapter) Switch(branch string) error {
	return a.exec.Switch(branch)
}

// Stash saves current changes
func (a *GitWriteAdapter) Stash() error {
	_, err := a.exec.Stash()
	return err
}

// StashPop restores stashed changes
func (a *GitWriteAdapter) StashPop() error {
	_, err := a.exec.StashPop()
	return err
}

// Push pushes to the remote
func (a *GitWriteAdapter) Push() (string, error) {
	return a.exec.Push()
}

// Pull pulls from the remote
func (a *GitWriteAdapter) Pull() (string, error) {
	return a.exec.Pull()
}

// Fetch fetches from the remote
func (a *GitWriteAdapter) Fetch() (string, error) {
	return a.exec.Fetch()
}

// Remove removes files from the repository
func (a *GitWriteAdapter) Remove(paths []string) error {
	return a.exec.Remove(paths)
}

// Ensure GitWriteAdapter implements ports.GitWritePort
var _ ports.GitWritePort = (*GitWriteAdapter)(nil)
