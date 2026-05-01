package git

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

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

func (a *ExecAdapter) Pull() (string, error) { return a.runGit("pull") }

func (a *ExecAdapter) PushTags() (string, error) { return a.runGit("push", "--tags") }

func (a *ExecAdapter) Fetch() (string, error) { return a.runGit("fetch", "--all") }

func (a *ExecAdapter) Stash() (string, error) {
	return a.runGit("stash", "push", "-m", "git-courer stash")
}

func (a *ExecAdapter) StashPop() (string, error) { return a.runGit("stash", "pop") }

func (a *ExecAdapter) Commit(message string) (string, error) {
	log.Printf("[DEBUG] gitAdapter.Commit: message=%q", message)
	out, err := a.runGit("commit", "-m", message)
	if err != nil {
		log.Printf("[DEBUG] gitAdapter.Commit: error=%v, out=%q", err, out)
	}
	return out, err
}

func (a *ExecAdapter) Branch(name string) (string, error) {
	return a.runGit("checkout", "-b", name)
}

func (a *ExecAdapter) RenameBranch(oldName, newName string) (string, error) {
	return a.runGit("branch", "-m", oldName, newName)
}

func (a *ExecAdapter) DeleteBranch(name string) (string, error) {
	return a.runGit("branch", "-d", name)
}

func (a *ExecAdapter) Reset(mode string, commit string) (string, error) {
	return a.runGit("reset", mode, commit)
}

func (a *ExecAdapter) Merge(branch string) (string, error) { return a.runGit("merge", branch) }

func (a *ExecAdapter) IsGHAuthenticated() (bool, error) {
	_, err := a.runGH("auth", "status")
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (a *ExecAdapter) CreateRelease(tagName, changelog string) (string, error) {
	if tagName == "" {
		return "", fmt.Errorf("release tag name is required")
	}
	if changelog == "" {
		changelog = "No changelog provided"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "gh", "release", "create", tagName, "-F", "-")
	cmd.Dir = a.workDir
	cmd.Stdin = strings.NewReader(changelog)
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("gh release timed out after 60s")
		}
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if stderr == "" {
				return "", fmt.Errorf("gh release error (empty stderr). Command: gh release create %s", tagName)
			}
			return "", fmt.Errorf("gh release error: %s", stderr)
		}
		return "", fmt.Errorf("gh release error: %w", err)
	}
	return string(out), nil
}
