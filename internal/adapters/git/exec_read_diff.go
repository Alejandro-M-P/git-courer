package git

import "strings"

func (a *ExecAdapter) Diff(paths ...string) (string, error) {
	args := []string{"diff", "--no-ext-diff"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	return a.runGit(args...)
}

func (a *ExecAdapter) DiffStat(paths ...string) (string, error) {
	args := []string{"diff", "--no-ext-diff", "--stat"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	return a.runGit(args...)
}

func (a *ExecAdapter) DiffStatStaged(paths ...string) (string, error) {
	args := []string{"diff", "--no-ext-diff", "--cached", "--stat"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	return a.runGit(args...)
}

func (a *ExecAdapter) DiffStaged(paths ...string) (string, error) {
	args := []string{"diff", "--no-ext-diff", "--cached"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	return a.runGit(args...)
}

func (a *ExecAdapter) DiffAll(paths ...string) (string, error) {
	args := []string{"diff", "--no-ext-diff", "HEAD"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	return a.runGit(args...)
}

func (a *ExecAdapter) DiffUntracked() (string, error) {
	files, err := a.ListUntracked()
	if err != nil || len(files) == 0 {
		return "", err
	}
	var sb strings.Builder
	for _, f := range files {
		out, err := a.runGit("diff", "--no-index", "/dev/null", f)
		if err != nil {
			continue
		}
		sb.WriteString(out)
		sb.WriteString("\n")
	}
	return sb.String(), nil
}

func (a *ExecAdapter) DiffRange(base, target, mode string, paths ...string) (string, error) {
	ref := base + mode + target
	args := []string{"diff", "--no-ext-diff", ref}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	return a.runGit(args...)
}
