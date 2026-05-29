package git

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

func (a *ExecAdapter) DiffRange(base, target, mode string, paths ...string) (string, error) {
	ref := base + mode + target
	args := []string{"diff", "--no-ext-diff", ref}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	return a.runGit(args...)
}
