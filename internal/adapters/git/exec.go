package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type ExecAdapter struct {
	workDir string
}

func New(workDir string) *ExecAdapter {
	if workDir == "" {
		workDir = "."
	}
	return &ExecAdapter{workDir: workDir}
}

func (a *ExecAdapter) runGit(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = a.workDir
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
