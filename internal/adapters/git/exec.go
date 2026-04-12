// Package git provides the git adapter — executes real git commands via os/exec.
package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// ExecAdapter implements ports.Git using os/exec.
type ExecAdapter struct {
	workDir string
}

// New creates a new git ExecAdapter.
func New(workDir string) *ExecAdapter {
	if workDir == "" {
		workDir = "."
	}
	return &ExecAdapter{workDir: workDir}
}

// runGit executes a git command and returns stdout.
func (a *ExecAdapter) runGit(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = a.workDir
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("git command timed out after 30s")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if args[0] == "push" && (strings.Contains(stderr, "push rejected") ||
				strings.Contains(stderr, "Updates were rejected") ||
				strings.Contains(stderr, "non-fast-forward")) {
				return "", fmt.Errorf("PUSH_REJECTED: remote has commits not in local. Pull first. Details: %s", stderr)
			}
			if args[0] == "pull" && strings.Contains(stderr, "merge conflict") {
				return "", fmt.Errorf("MERGE_CONFLICT: resolve conflicts manually. Details: %s", stderr)
			}
			if stderr == "" {
				return "", fmt.Errorf("git error (empty stderr). Command: git %v. Stdout: %s", args, string(out))
			}
			return "", fmt.Errorf("git error: %s", stderr)
		}
		return "", fmt.Errorf("git error: %w", err)
	}
	return string(out), nil
}

// runGH executes a gh command and returns stdout.
func (a *ExecAdapter) runGH(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = a.workDir
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("gh command timed out after 30s")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if stderr == "" {
				return "", fmt.Errorf("gh error (empty stderr). Command: gh %v. Stdout: %s", args, string(out))
			}
			return "", fmt.Errorf("gh error: %s", stderr)
		}
		return "", fmt.Errorf("gh error: %w", err)
	}
	return string(out), nil
}

// --- Read ---

func (a *ExecAdapter) Status() (domain.Status, error) {
	out, err := a.runGit("status", "--porcelain")
	if err != nil {
		return domain.Status{}, err
	}

	status := domain.Status{Files: []domain.FileStatus{}}
	for _, line := range strings.Split(out, "\n") {
		if len(line) < 4 {
			continue
		}
		indexStatus := rune(line[0])
		workTreeStatus := rune(line[1])
		path := line[3:]

		fs := domain.FileStatus{
			Path:   path,
			Status: string(indexStatus) + string(workTreeStatus),
		}
		switch indexStatus {
		case 'A':
			fs.Staged, fs.IsNew = true, true
		case 'M':
			fs.Staged = true
		case 'D':
			fs.Staged, fs.IsDeleted = true, true
		case 'R':
			fs.Staged, fs.IsRenamed = true, true
		case '?':
			fs.IsNew = true
		}
		switch workTreeStatus {
		case 'M':
			fs.Staged = false
		case 'D':
			fs.IsDeleted, fs.Staged = true, false
		case 'A':
			fs.IsNew = true
		}
		status.Files = append(status.Files, fs)
		if fs.Staged {
			status.Staged++
		}
		if workTreeStatus == 'M' {
			status.Modified++
		}
		if indexStatus == '?' {
			status.Untracked++
		}
	}
	status.Branch, _ = a.CurrentBranch()
	status.IsClean = len(status.Files) == 0
	return status, nil
}

func (a *ExecAdapter) Diff() (string, error)       { return a.runGit("diff") }
func (a *ExecAdapter) DiffStaged() (string, error) { return a.runGit("diff", "--cached") }
func (a *ExecAdapter) ListUntracked() ([]string, error) {
	out, err := a.runGit("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []string{}, nil
	}
	return strings.Split(strings.TrimSpace(out), "\n"), nil
}
func (a *ExecAdapter) Log(limit int) (string, error) {
	return a.runGit("log", fmt.Sprintf("-%d", limit), "--oneline")
}
func (a *ExecAdapter) Show(commit string) (string, error) { return a.runGit("show", commit) }
func (a *ExecAdapter) Blame(file string) (string, error)  { return a.runGit("blame", file) }
func (a *ExecAdapter) Reflog(limit int) (string, error) {
	return a.runGit("reflog", fmt.Sprintf("-%d", limit))
}
func (a *ExecAdapter) CurrentBranch() (string, error) {
	out, err := a.runGit("branch", "--show-current")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
func (a *ExecAdapter) ListBranches() (string, error) { return a.runGit("branch", "-a") }
func (a *ExecAdapter) ListTags() ([]string, error) {
	out, err := a.runGit("tag", "-l")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []string{}, nil
	}
	return strings.Split(strings.TrimSpace(out), "\n"), nil
}
func (a *ExecAdapter) IsRepo() bool {
	_, err := os.Stat(fmt.Sprintf("%s/.git", a.workDir))
	return err == nil
}

// --- Write · Direct ---

func (a *ExecAdapter) Add(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	_, err := a.runGit(append([]string{"add"}, paths...)...)
	return err
}

func (a *ExecAdapter) Remove(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	_, err := a.runGit(append([]string{"rm"}, paths...)...)
	return err
}

func (a *ExecAdapter) Checkout(name string) (string, error) { return a.runGit("checkout", name) }

func (a *ExecAdapter) Switch(name string) error {
	_, err := a.runGit("switch", name)
	return err
}

func (a *ExecAdapter) Push() (string, error) {
	out, err := a.runGit("push")
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "PUSH_REJECTED") ||
			strings.Contains(errStr, "Updates were rejected") ||
			strings.Contains(errStr, "non-fast-forward") {
			a.runGit("fetch", "origin")
			pullOut, pullErr := a.runGit("pull", "--rebase")
			if pullErr != nil {
				pullOut, pullErr = a.runGit("pull")
				if pullErr != nil {
					return "", fmt.Errorf("PUSH_REJECTED: pull failed: %s\n%s", pullErr, pullOut)
				}
			}
			out, err = a.runGit("push")
			if err != nil {
				return "", fmt.Errorf("PUSH_REJECTED: after pull, push still failed: %s", err)
			}
			return pullOut + "\n" + out, nil
		}
	}
	return out, err
}

func (a *ExecAdapter) PushWithUpstream(branch string) (string, error) {
	_, err := a.runGit("branch", "--set-upstream-to=origin/"+branch, branch)
	if err != nil {
		return a.runGit("push", "-u", "origin", branch)
	}
	return a.runGit("push", "origin", branch)
}

func (a *ExecAdapter) Pull() (string, error)       { return a.runGit("pull") }
func (a *ExecAdapter) PullRebase() (string, error) { return a.runGit("pull", "--rebase") }
func (a *ExecAdapter) Fetch() (string, error)      { return a.runGit("fetch", "--all") }
func (a *ExecAdapter) Stash() (string, error) {
	return a.runGit("stash", "push", "-m", "git-courer stash")
}
func (a *ExecAdapter) StashPop() (string, error) { return a.runGit("stash", "pop") }

func (a *ExecAdapter) ResetSoft(commits int) error {
	if commits <= 0 {
		return fmt.Errorf("number of commits must be positive")
	}
	_, err := a.runGit("reset", "--soft", fmt.Sprintf("HEAD~%d", commits))
	return err
}

// --- Write · Workflow ---

func (a *ExecAdapter) Commit(message string) (string, error) {
	return a.runGit("commit", "-m", message)
}

func (a *ExecAdapter) Branch(name string) (string, error) {
	return a.runGit("checkout", "-b", name)
}

func (a *ExecAdapter) DeleteBranch(name string) (string, error) {
	return a.runGit("branch", "-d", name)
}

func (a *ExecAdapter) RenameBranch(oldName, newName string) (string, error) {
	return a.runGit("branch", "-m", oldName, newName)
}

func (a *ExecAdapter) Merge(branch string) (string, error)  { return a.runGit("merge", branch) }
func (a *ExecAdapter) Rebase(branch string) (string, error) { return a.runGit("rebase", branch) }
func (a *ExecAdapter) RebaseContinue() (string, error)      { return a.runGit("rebase", "--continue") }
func (a *ExecAdapter) RebaseAbort() (string, error)         { return a.runGit("rebase", "--abort") }

func (a *ExecAdapter) Reset(mode string, commit string) (string, error) {
	return a.runGit("reset", mode, commit)
}

func (a *ExecAdapter) CherryPick(commit string) (string, error) {
	return a.runGit("cherry-pick", commit)
}

func (a *ExecAdapter) Revert(commit string) (string, error) {
	return a.runGit("revert", "--no-edit", commit)
}

func (a *ExecAdapter) Clean(directories bool) (string, error) {
	flags := "-f"
	if directories {
		flags = "-fd"
	}
	return a.runGit("clean", flags)
}

func (a *ExecAdapter) Tag(name string) (string, error)       { return a.runGit("tag", name) }
func (a *ExecAdapter) DeleteTag(name string) (string, error) { return a.runGit("tag", "-d", name) }

func (a *ExecAdapter) AddRemote(name, url string) (string, error) {
	return a.runGit("remote", "add", name, url)
}

func (a *ExecAdapter) RemoveRemote(name string) (string, error) {
	return a.runGit("remote", "remove", name)
}

func (a *ExecAdapter) Init() (string, error)            { return a.runGit("init") }
func (a *ExecAdapter) Clone(url string) (string, error) { return a.runGit("clone", url) }

// --- Tags & Releases ---

// LatestTag returns the most recent tag.
func (a *ExecAdapter) LatestTag() (string, error) {
	out, err := a.runGit("describe", "--tags", "--abbrev=0")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// CommitsFromTag returns commits since the given tag.
func (a *ExecAdapter) CommitsFromTag(tag string) (string, error) {
	if tag == "" {
		return "", fmt.Errorf("tag name is required")
	}
	out, err := a.runGit("log", tag+"..HEAD", "--oneline")
	if err != nil {
		return "", err
	}
	return out, nil
}

// TagExists checks if a tag exists.
func (a *ExecAdapter) TagExists(name string) (bool, error) {
	if name == "" {
		return false, fmt.Errorf("tag name is required")
	}
	out, err := a.runGit("tag", "-l", name)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == name, nil
}

// IsGHAuthenticated checks if gh is authenticated.
func (a *ExecAdapter) IsGHAuthenticated() (bool, error) {
	_, err := a.runGH("auth", "status")
	if err != nil {
		return false, nil
	}
	// If command succeeds without error, user is authenticated
	return true, nil
}

// CreateRelease creates a GitHub release.
func (a *ExecAdapter) CreateRelease(name, changelog string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("release name is required")
	}
	if changelog == "" {
		changelog = "No changelog provided"
	}

	// Create temp file for release notes
	f, err := os.CreateTemp("", "release-notes-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := f.Name()
	defer os.Remove(tempPath)

	// Write changelog to temp file
	if _, err := f.WriteString(changelog); err != nil {
		os.Remove(tempPath)
		return "", fmt.Errorf("failed to write changelog: %w", err)
	}
	f.Close()

	// Create release with temp file
	out, err := a.runGH("release", "create", name, "--notes-file", tempPath)
	if err != nil {
		return "", err
	}

	return out, nil
}

// Compile-time interface check.
var _ ports.Git = (*ExecAdapter)(nil)
