package git

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
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

func (a *ExecAdapter) Diff(paths ...string) (string, error) {
	args := []string{"diff"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	return a.runGit(args...)
}

func (a *ExecAdapter) DiffStaged(paths ...string) (string, error) {
	args := []string{"diff", "--cached"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	return a.runGit(args...)
}

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

func (a *ExecAdapter) Log(limit int, paths ...string) (string, error) {
	args := []string{"log", fmt.Sprintf("-%d", limit), "--oneline"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	return a.runGit(args...)
}

func (a *ExecAdapter) LogFull(limit int) (string, error) {
	return a.runGit("log", fmt.Sprintf("-%d", limit))
}

func (a *ExecAdapter) CurrentBranch() (string, error) {
	out, err := a.runGit("branch", "--show-current")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (a *ExecAdapter) ListBranches(pattern ...string) (string, error) {
	args := []string{"branch", "-a"}
	if len(pattern) > 0 && pattern[0] != "" {
		args = append(args, "--list", pattern[0])
	}
	return a.runGit(args...)
}

func (a *ExecAdapter) ListTags(pattern ...string) ([]string, error) {
	args := []string{"tag", "-l"}
	if len(pattern) > 0 && pattern[0] != "" {
		args = append(args, pattern[0])
	}
	out, err := a.runGit(args...)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return []string{}, nil
	}
	return strings.Split(strings.TrimSpace(out), "\n"), nil
}

func (a *ExecAdapter) IsRepo() bool {
	out, err := a.runGit("rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

func (a *ExecAdapter) Add(paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	log.Printf("[DEBUG] gitAdapter.Add: paths=%v", paths)
	_, err := a.runGit(append([]string{"add"}, paths...)...)
	if err != nil {
		log.Printf("[DEBUG] gitAdapter.Add: error=%v", err)
	}
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

func (a *ExecAdapter) Tag(name, message string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("tag name is required")
	}
	if !domain.IsValidTagName(name) {
		return "", fmt.Errorf("invalid tag name: %s (use semver like v1.0.0 or 1.0.0)", name)
	}
	if message != "" {
		return a.runGit("tag", "-a", name, "-m", message)
	}
	return a.runGit("tag", name)
}

func (a *ExecAdapter) DeleteTag(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("tag name is required")
	}
	exists, err := a.TagExists(name)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("tag %s not found", name)
	}
	return a.runGit("tag", "-d", name)
}

func (a *ExecAdapter) PushTag(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("tag name is required")
	}
	out, err := a.runGit("push", "origin", name)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "already exists") || strings.Contains(errStr, "rejected") {
			return "", fmt.Errorf("tag %s already exists in remote. Use TAG_DELETE_REMOTE first.", name)
		}
		return "", err
	}
	return out, nil
}

func (a *ExecAdapter) DeleteTagRemote(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("tag name is required")
	}
	return a.runGit("push", "origin", ":refs/tags/"+name)
}

func (a *ExecAdapter) LatestTag() (string, error) {
	out, err := a.runGit("describe", "--tags", "--abbrev=0")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

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

func (a *ExecAdapter) CreateBackup(operation string, stashUntracked bool) (domain.Backup, error) {
	timestamp := time.Now().Format("20060102150405")
	ref := fmt.Sprintf("refs/git-courer/backup/%s_%s", timestamp, operation)

	_, err := a.runGit("update-ref", "-m", fmt.Sprintf("git-courer backup: %s", operation), ref, "HEAD")
	if err != nil {
		return domain.Backup{}, fmt.Errorf("failed to create backup ref: %w", err)
	}

	hasStash := false
	unstaged, _ := a.hasUnstaged()
	untracked, _ := a.hasUntracked()

	if unstaged || (stashUntracked && untracked) {
		hasStash = true
		label := fmt.Sprintf("git-courer:backup:%s:%s", operation, timestamp)
		args := []string{"stash", "push", "-m", label}
		if stashUntracked {
			args = append(args, "--include-untracked")
		}
		_, err = a.runGit(args...)
		if err != nil {
			return domain.Backup{}, fmt.Errorf("failed to create stash: %w", err)
		}
	}

	return domain.Backup{
		Ref:       ref,
		HasStash:  hasStash,
		Operation: operation,
		CreatedAt: time.Now(),
	}, nil
}

func (a *ExecAdapter) RestoreBackup(backup domain.Backup) error {
	_, err := a.runGit("update-ref", "HEAD", backup.Ref)
	if err != nil {
		return fmt.Errorf("failed to restore HEAD: %w (ref %s still exists for manual restore)", err, backup.Ref)
	}

	if backup.HasStash {
		stashIdx, sErr := a.findStashByLabel("git-courer:backup:" + backup.Operation)
		if sErr == nil {
			_, err = a.runGit("stash", "pop", fmt.Sprintf("stash@{%d}", stashIdx))
			if err != nil {
				return fmt.Errorf("failed to pop stash: %w", err)
			}
		}
	}
	return nil
}

func (a *ExecAdapter) DeleteBackup(backup domain.Backup) error {
	_, err := a.runGit("update-ref", "-d", backup.Ref)
	if err != nil {
		return fmt.Errorf("failed to delete backup ref: %w", err)
	}

	if backup.HasStash {
		stashIdx, sErr := a.findStashByLabel("git-courer:backup:" + backup.Operation)
		if sErr == nil {
			_, err = a.runGit("stash", "drop", fmt.Sprintf("stash@{%d}", stashIdx))
			if err != nil {
				return fmt.Errorf("failed to drop stash: %w", err)
			}
		}
	}
	return nil
}

func (a *ExecAdapter) hasUnstaged() (bool, error) {
	out, err := a.runGit("status", "--porcelain")
	if err != nil {
		return false, err
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	for _, line := range lines {
		if len(line) < 2 {
			continue
		}
		if line[1] != ' ' && line[1] != '?' {
			return true, nil
		}
		if line[0] != ' ' && line[0] != '?' {
			return true, nil
		}
	}
	return false, nil
}

func (a *ExecAdapter) hasUntracked() (bool, error) {
	out, err := a.runGit("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (a *ExecAdapter) hasUnstagedOrUntracked() (bool, error) {
	out, err := a.runGit("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func (a *ExecAdapter) findStashByLabel(label string) (int, error) {
	out, err := a.runGit("stash", "list")
	if err != nil {
		return -1, err
	}
	for i, line := range strings.Split(out, "\n") {
		if strings.Contains(line, label) {
			return i, nil
		}
	}
	return -1, fmt.Errorf("stash with label %q not found", label)
}
