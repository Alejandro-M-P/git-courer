package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
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

			// Improved error message when stderr is empty
			if stderr == "" {
				return "", fmt.Errorf("git error (empty stderr). Command: git %v. Stdout: %s", args, string(out))
			}
			return "", fmt.Errorf("git error: %s", stderr)
		}
		return "", fmt.Errorf("git error: %w", err)
	}
	return string(out), nil
}

// Status returns the current repository status
func (a *ExecAdapter) Status() (domain.Status, error) {
	out, err := a.runGit("status", "--porcelain")
	if err != nil {
		return domain.Status{}, err
	}

	status := domain.Status{
		Files: []domain.FileStatus{},
	}

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		// git status --porcelain format: "XY filename" where X=index, Y=worktree
		// Examples: "M " (modified in index), " M" (modified in worktree), "MM" (both), "??" (untracked)
		// There is always a space between the status and the filename
		if len(line) < 4 {
			continue
		}

		indexStatus := rune(line[0])
		workTreeStatus := rune(line[1])
		path := line[3:] // Path starts at position 3 (after "XY ")

		fileStatus := domain.FileStatus{
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

		// Also check workTreeStatus for unstaged changes
		switch workTreeStatus {
		case 'M':
			fileStatus.Staged = false
		case 'D':
			fileStatus.IsDeleted = true
			fileStatus.Staged = false
		case 'A':
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
	out, err := a.runGit("push")
	if err != nil {
		errStr := err.Error()
		// Check if rejected (remote has commits we don't have)
		if strings.Contains(errStr, "push rejected") ||
			strings.Contains(errStr, "Updates were rejected") ||
			strings.Contains(errStr, "non-fast-forward") {

			// Fetch to get remote state
			a.runGit("fetch", "origin")

			// Pull with rebase to integrate remote changes
			pullOut, pullErr := a.runGit("pull", "--rebase")
			if pullErr != nil {
				// If rebase fails, try regular pull
				pullOut, pullErr = a.runGit("pull")
				if pullErr != nil {
					return "", fmt.Errorf("PUSH_REJECTED: pull failed: %s\n%s", pullErr, pullOut)
				}
			}

			// Retry push
			out, err = a.runGit("push")
			if err != nil {
				return "", fmt.Errorf("PUSH_REJECTED: after pull, push still failed: %s", err)
			}
			return pullOut + "\n" + out, nil
		}
	}
	return out, err
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

// Checkout switches to a branch (implements ports.Git)
func (a *ExecAdapter) Checkout(name string) (string, error) {
	return a.runGit("checkout", name)
}

// Switch switches to a branch (same as checkout)
func (a *ExecAdapter) Switch(name string) error {
	_, err := a.runGit("switch", name)
	return err
}

// Remove removes files from the repository
func (a *ExecAdapter) Remove(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"rm"}, paths...)
	_, err := a.runGit(args...)
	return err
}

// CreateBranch creates a new branch
func (a *ExecAdapter) CreateBranch(name string) (string, error) {
	return a.runGit("checkout", "-b", name)
}

// DeleteBranch deletes a branch
func (a *ExecAdapter) DeleteBranch(name string) (string, error) {
	return a.runGit("branch", "-d", name)
}

// RenameBranch renames a branch
func (a *ExecAdapter) RenameBranch(oldName, newName string) (string, error) {
	return a.runGit("branch", "-m", oldName, newName)
}

// DeleteTag deletes a tag
func (a *ExecAdapter) DeleteTag(name string) (string, error) {
	return a.runGit("tag", "-d", name)
}

// RebaseContinue continues a rebase
func (a *ExecAdapter) RebaseContinue() (string, error) {
	return a.runGit("rebase", "--continue")
}

// RebaseAbort aborts a rebase
func (a *ExecAdapter) RebaseAbort() (string, error) {
	return a.runGit("rebase", "--abort")
}

// AddRemote adds a remote
func (a *ExecAdapter) AddRemote(name, url string) (string, error) {
	return a.runGit("remote", "add", name, url)
}

// RemoveRemote removes a remote
func (a *ExecAdapter) RemoveRemote(name string) (string, error) {
	return a.runGit("remote", "remove", name)
}

// Init initializes a git repository
func (a *ExecAdapter) Init() (string, error) {
	return a.runGit("init")
}

// Clone clones a repository
func (a *ExecAdapter) Clone(url string) (string, error) {
	return a.runGit("clone", url)
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

// ResetSoft undoes the last n commits, keeping changes staged
func (a *ExecAdapter) ResetSoft(commits int) error {
	if commits <= 0 {
		return fmt.Errorf("number of commits must be positive")
	}
	_, err := a.runGit("reset", "--soft", fmt.Sprintf("HEAD~%d", commits))
	return err
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

// CherryPick cherry-picks a commit
func (a *ExecAdapter) CherryPick(commit string) (string, error) {
	return a.runGit("cherry-pick", commit)
}

// Tag creates a tag
func (a *ExecAdapter) Tag(name string) (string, error) {
	return a.runGit("tag", name)
}

// Reflog returns the reflog
func (a *ExecAdapter) Reflog(limit int) (string, error) {
	return a.runGit("reflog", fmt.Sprintf("-%d", limit))
}

// PushWithUpstream pushes a branch and sets its upstream
func (a *ExecAdapter) PushWithUpstream(branch string) (string, error) {
	// First try to set upstream
	_, err := a.runGit("branch", "--set-upstream-to=origin/"+branch, branch)
	if err != nil {
		// If that fails, try push with -u
		return a.runGit("push", "-u", "origin", branch)
	}
	// Then push
	return a.runGit("push", "origin", branch)
}

// IsRepo checks if directory is a git repository
func (a *ExecAdapter) IsRepo() bool {
	_, err := os.Stat(fmt.Sprintf("%s/.git", a.workDir))
	return err == nil
}

// Ensure ExecAdapter implements ports.Git
var _ ports.Git = (*ExecAdapter)(nil)
