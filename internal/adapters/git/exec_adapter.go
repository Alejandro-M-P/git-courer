package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/domain/models"
	"github.com/Alejandro-M-P/git-courer/internal/ports/git"
)

// ExecAdapter implements the Port interface using os/exec
// This is the "git executor" - it runs actual git commands
type ExecAdapter struct {
	workDir string
}

// NewExecAdapter creates a new git executor adapter
func NewExecAdapter(workDir string) *ExecAdapter {
	if workDir == "" {
		workDir = "."
	}
	return &ExecAdapter{workDir: workDir}
}

// runGit executes a git command and returns the output
func (a *ExecAdapter) runGit(args ...string) (string, error) {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = a.workDir
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)

			// Detect push rejected (remote has changes)
			if args[0] == "push" && (strings.Contains(stderr, "push rejected") ||
				strings.Contains(stderr, "Updates were rejected") ||
				strings.Contains(stderr, "non-fast-forward")) {
				return "", fmt.Errorf("PUSH_REJECTED: remote has commits that aren't in your local branch. Pull the remote changes first with 'git_do pull', then push again. Details: %s", stderr)
			}

			// Detect merge conflicts
			if args[0] == "pull" && strings.Contains(stderr, "merge conflict") {
				return "", fmt.Errorf("MERGE_CONFLICT: there are merge conflicts that need to be resolved manually. Details: %s", stderr)
			}

			return "", fmt.Errorf("git error: %s", stderr)
		}
		return "", fmt.Errorf("git error: %w", err)
	}
	return string(out), nil
}

// Status returns the current repository status
func (a *ExecAdapter) Status() (models.Status, error) {
	out, err := a.runGit("status", "--porcelain")
	if err != nil {
		return models.Status{}, err
	}

	status := models.Status{
		Files: []models.FileStatus{},
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		if len(line) < 2 {
			continue
		}

		indexStatus := line[0]
		workTreeStatus := line[1]
		path := strings.TrimSpace(line[3:])

		fileStatus := models.FileStatus{
			Path:   path,
			Status: string(indexStatus) + string(workTreeStatus),
		}

		// Determine status flags
		switch indexStatus {
		case 'A':
			fileStatus.Staged = true
			fileStatus.IsNew = true
		case 'M':
			fileStatus.Staged = true
		case 'D':
			fileStatus.Staged = true
			fileStatus.IsDeleted = true
		case 'R':
			fileStatus.Staged = true
			fileStatus.IsRenamed = true
		case '?':
			fileStatus.IsNew = true
		}

		status.Files = append(status.Files, fileStatus)

		// Count by type
		if fileStatus.Staged {
			status.Staged++
		}
		if workTreeStatus == 'M' {
			status.Modified++
		}
		if indexStatus == '?' {
			status.Untracked++
		}
	}

	// Get current branch
	status.Branch, _ = a.CurrentBranch()
	status.IsClean = len(status.Files) == 0

	return status, nil
}

// Diff returns the diff for all changes
func (a *ExecAdapter) Diff() (string, error) {
	return a.runGit("diff")
}

// DiffStaged returns the diff for staged files only
func (a *ExecAdapter) DiffStaged() (string, error) {
	return a.runGit("diff", "--cached")
}

// Add stages files
func (a *ExecAdapter) Add(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"add"}, paths...)
	_, err := a.runGit(args...)
	return err
}

// Commit creates a commit with the given message
func (a *ExecAdapter) Commit(message string) (string, error) {
	return a.runGit("commit", "-m", message)
}

// Push pushes to the remote
func (a *ExecAdapter) Push() (string, error) {
	return a.runGit("push")
}

// Pull pulls from the remote
func (a *ExecAdapter) Pull() (string, error) {
	return a.runGit("pull")
}

// PullRebase pulls with rebase
func (a *ExecAdapter) PullRebase() (string, error) {
	return a.runGit("pull", "--rebase")
}

// Fetch fetches from the remote
func (a *ExecAdapter) Fetch() (string, error) {
	return a.runGit("fetch", "--all")
}

// Branch creates a new branch
func (a *ExecAdapter) Branch(name string) (string, error) {
	return a.runGit("checkout", "-b", name)
}

// Checkout switches to a branch
func (a *ExecAdapter) Checkout(name string) (string, error) {
	return a.runGit("checkout", name)
}

// CurrentBranch returns the current branch name
func (a *ExecAdapter) CurrentBranch() (string, error) {
	out, err := a.runGit("branch", "--show-current")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Merge merges a branch into current
func (a *ExecAdapter) Merge(branch string) (string, error) {
	return a.runGit("merge", branch)
}

// Rebase rebases onto another branch
func (a *ExecAdapter) Rebase(branch string) (string, error) {
	return a.runGit("rebase", branch)
}

// Stash saves current changes
func (a *ExecAdapter) Stash() (string, error) {
	return a.runGit("stash", "push", "-m", "git-courer stash")
}

// StashPop restores stashed changes
func (a *ExecAdapter) StashPop() (string, error) {
	return a.runGit("stash", "pop")
}

// Reset resets to a commit
func (a *ExecAdapter) Reset(mode string, commit string) (string, error) {
	return a.runGit("reset", mode, commit)
}

// Log returns commit history
func (a *ExecAdapter) Log(limit int) (string, error) {
	return a.runGit("log", fmt.Sprintf("-%d", limit), "--oneline")
}

// Show returns details about a commit
func (a *ExecAdapter) Show(commit string) (string, error) {
	return a.runGit("show", commit)
}

// Blame returns blame information
func (a *ExecAdapter) Blame(file string) (string, error) {
	return a.runGit("blame", file)
}

// Clean removes untracked files
func (a *ExecAdapter) Clean(directories bool) (string, error) {
	flags := "-f"
	if directories {
		flags = "-fd"
	}
	return a.runGit("clean", flags)
}

// Revert creates a revert commit
func (a *ExecAdapter) Revert(commit string) (string, error) {
	return a.runGit("revert", "--no-edit", commit)
}

// IsRepo checks if directory is a git repository
func (a *ExecAdapter) IsRepo() bool {
	_, err := os.Stat(fmt.Sprintf("%s/.git", a.workDir))
	return err == nil
}

// Ensure ExecAdapter implements Port
var _ git.Port = (*ExecAdapter)(nil)
