package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ExecAdapter struct {
	workDir   string
	workDirFn func() string
}

func New(workDir string) *ExecAdapter {
	if workDir == "" || workDir == "." {
		// Resolve to the actual git repo root to avoid CWD dependency
		if root, err := findGitRoot(workDir); err == nil {
			workDir = root
		} else {
			// Fallback: make CWD absolute so it doesn't drift if process chdir
			if abs, err := filepath.Abs(workDir); err == nil {
				workDir = abs
			}
		}
	} else if !filepath.IsAbs(workDir) {
		if abs, err := filepath.Abs(workDir); err == nil {
			workDir = abs
		}
	}
	return &ExecAdapter{workDir: workDir}
}

// findGitRoot returns the absolute path to the git repository root
// using git rev-parse --show-toplevel. Falls back to error if not in a repo.
func findGitRoot(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repo: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (a *ExecAdapter) workDirResolved() string {
	if a.workDirFn != nil {
		return a.workDirFn()
	}
	return a.workDir
}

// SetWorkDirFn installs a callback used to resolve the working directory for
// git/gh commands at execution time. When set, the callback's return value
// takes precedence over the static workDir. Pass nil to clear.
func (a *ExecAdapter) SetWorkDirFn(fn func() string) {
	a.workDirFn = fn
}

func (a *ExecAdapter) runGitWithStdin(args []string, stdinData string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = a.workDirResolved()
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.Stdin = strings.NewReader(stdinData)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("git command timed out after 300s")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if stderr == "" {
				return "", fmt.Errorf("git error (empty stderr). Command: git %v. Stdout: %s", args, string(out))
			}
			return "", fmt.Errorf("git error: %s", stderr)
		}
		return "", fmt.Errorf("git error: %w", err)
	}
	return string(out), nil
}

func (a *ExecAdapter) runGit(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = a.workDirResolved()
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("git command timed out after 300s")
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
			if args[0] == "pull" && (strings.Contains(stderr, "no upstream") || strings.Contains(stderr, "No remote repository") || strings.Contains(stderr, "There is no tracking information") || strings.Contains(stderr, "No hay información de rastreo")) {
				return "", fmt.Errorf("NO_UPSTREAM: no upstream configured for current branch. Details: %s", stderr)
			}
			if len(args) >= 2 && args[0] == "stash" && args[1] == "pop" && (strings.Contains(stderr, "could not restore untracked files") || strings.Contains(stderr, "no se pudo restaurar archivos no rastreados") || strings.Contains(stderr, "could not restore")) {
				return "", fmt.Errorf("STASH_POP_UNTRACKED: could not restore untracked files from stash. Details: %s", stderr)
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

func (a *ExecAdapter) runGH(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", args...)
	cmd.Dir = a.workDirResolved()
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
