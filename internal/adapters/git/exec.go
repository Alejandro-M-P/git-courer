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

func (a *ExecAdapter) RemoteURL() (string, error) {
	return a.runGit("remote", "get-url", "origin")
}
