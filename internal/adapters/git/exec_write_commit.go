package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func (a *ExecAdapter) Commit(message string) (string, error) {
	return a.runGit("commit", "-m", message)
}

func (a *ExecAdapter) Revert(commit string) (string, error) {
	return a.runGit("revert", "--no-edit", commit)
}

func (a *ExecAdapter) Amend(message string, paths []string) (string, error) {
	if len(paths) > 0 {
		a.Add(paths)
	}
	if message != "" {
		return a.runGit("commit", "--amend", "-m", message)
	}
	return a.runGit("commit", "--amend", "--no-edit")
}

func (a *ExecAdapter) ShowCommit(commit string) (string, error) {
	return a.runGit("show", commit)
}

func (a *ExecAdapter) RemoteAdd(name, url string) (string, error) {
	return a.runGit("remote", "add", name, url)
}

func (a *ExecAdapter) RemoteRemove(name string) (string, error) {
	return a.runGit("remote", "remove", name)
}

func (a *ExecAdapter) Rebase(branch string) (string, error) {
	return a.runGit("rebase", branch)
}

func (a *ExecAdapter) RebaseAbort() (string, error) {
	return a.runGit("rebase", "--abort")
}

func (a *ExecAdapter) RebaseContinue() (string, error) {
	return a.runGit("rebase", "--continue")
}

func (a *ExecAdapter) RebaseSkip() (string, error) {
	return a.runGit("rebase", "--skip")
}

func (a *ExecAdapter) RebaseOnto(newBase, upstream, branch string) (string, error) {
	return a.runGit("rebase", "--onto", newBase, upstream, branch)
}

func (a *ExecAdapter) CherryPick(commit string) (string, error) {
	return a.runGit("cherry-pick", commit)
}

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

// emptyTreeHash is the SHA-1 of git's empty tree object (git mktree < /dev/null).
const emptyTreeHash = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

func (a *ExecAdapter) WriteTree() (string, error) {
	out, err := a.runGit("write-tree")
	if err != nil {
		return "", err
	}
	hash := strings.TrimSpace(out)
	if hash == "" || hash == emptyTreeHash {
		return "", fmt.Errorf("nothing to commit, staging area is empty")
	}
	return hash, nil
}

// CommitTree creates a commit object pointing at treeHash. When parentHash
// is empty the resulting commit has no parent (a root commit) — the -p flag
// is omitted entirely. This is required for the first commit on a freshly
// init'd repo (unborn HEAD), where applyPlumbing passes "" as parentHash
// because Head() fails. See the unborn-first-commit spec delta.
func (a *ExecAdapter) CommitTree(treeHash, parentHash, message string) (string, error) {
	args := []string{"commit-tree", treeHash}
	if parentHash != "" {
		args = append(args, "-p", parentHash)
	}
	args = append(args, "-m", message)
	out, err := a.runGit(args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (a *ExecAdapter) UpdateRef(ref, commitHash string) (string, error) {
	if commitHash == "" {
		_, err := a.runGit("update-ref", "-d", ref)
		return "", err
	}
	_, err := a.runGit("update-ref", ref, commitHash)
	return "", err
}

func (a *ExecAdapter) Head() (string, error) {
	out, err := a.runGit("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (a *ExecAdapter) HashObject(data []byte) (string, error) {
	out, err := a.runGitWithStdin([]string{"hash-object", "--stdin", "-w"}, string(data))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
