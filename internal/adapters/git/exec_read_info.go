package git

import (
	"strconv"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
)

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
			fs.Staged = true
			fs.IsNew = true
		case 'M', 'R', 'C':
			fs.Staged = true
		case 'D':
			fs.Staged = true
			fs.IsDeleted = true
		case 'U':
			status.Conflicted++
		case '?':
			fs.IsNew = true
			status.Untracked++
		}

		switch workTreeStatus {
		case 'M':
			status.Modified++
		case 'D':
			fs.IsDeleted = true
		case 'A':
			fs.IsNew = true
		case 'U':
			status.Conflicted++
		}

		if indexStatus == 'R' {
			fs.IsRenamed = true
		}

		status.Files = append(status.Files, fs)
		if fs.Staged {
			status.Staged++
		}
	}
	status.Branch, _ = a.CurrentBranch()
	status.IsClean = len(status.Files) == 0

	// Ahead/behind only make sense when HEAD resolves to a commit. On an
	// unborn repo (zero commits) rev-list would crash; detect unborn via
	// rev-parse --verify HEAD. When HEAD is unborn OR no upstream is
	// configured, Ahead/Behind stay nil — the JSON null signal. See the
	// spec delta "Status ahead/behind on unborn or upstream-less repos".
	if _, verifyErr := a.runGit("rev-parse", "--verify", "HEAD"); verifyErr == nil {
		abOut, err := a.runGit("rev-list", "--left-right", "--count", "HEAD...@{upstream}")
		if err == nil {
			status.HasUpstream = true
			parts := strings.Fields(strings.TrimSpace(abOut))
			if len(parts) == 2 {
				if ahead, err := strconv.Atoi(parts[0]); err == nil {
					status.Ahead = &ahead
				}
				if behind, err := strconv.Atoi(parts[1]); err == nil {
					status.Behind = &behind
				}
			}
		}
		// err != nil here means no upstream configured — leave Ahead/Behind nil.
	}
	// verifyErr != nil means unborn HEAD — leave Ahead/Behind nil.

	return status, nil
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

func (a *ExecAdapter) RemoteURL() (string, error) {
	return a.runGit("remote", "get-url", "origin")
}

func (a *ExecAdapter) RemoteInfo() (string, error) {
	return a.runGit("remote", "-v")
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

func (a *ExecAdapter) ConfigGet(key string) (string, error) {
	out, err := a.runGit("config", "--get", key)
	if err != nil {
		// git config --get returns exit code 1 if key is not found
		return "", nil
	}
	return strings.TrimSpace(out), nil
}

func (a *ExecAdapter) SymbolicRef(ref string) (string, error) {
	out, err := a.runGit("symbolic-ref", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (a *ExecAdapter) ShowRef(pattern string) (string, error) {
	args := []string{"show-ref"}
	if pattern != "" {
		args = append(args, pattern)
	}
	out, err := a.runGit(args...)
	if err != nil {
		return "", nil
	}
	return strings.TrimSpace(out), nil
}

// GitCommonDir returns the path to the git common directory
// (git rev-parse --git-common-dir). In a linked worktree, this resolves to
// the main repo's .git directory; in the main repo it returns the local .git
// path. The path may be relative to the adapter's workDir.
func (a *ExecAdapter) GitCommonDir() (string, error) {
	out, err := a.runGit("rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
