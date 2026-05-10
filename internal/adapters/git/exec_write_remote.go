package git

func (a *ExecAdapter) Push() (string, error) {
	branch, err := a.CurrentBranch()
	if err != nil {
		return "", err
	}
	return a.runGit("push", "-u", "origin", branch)
}

func (a *ExecAdapter) PushTo(remote string) (string, error) {
	if remote == "" {
		remote = "origin"
	}
	branch, err := a.CurrentBranch()
	if err != nil {
		return "", err
	}
	return a.runGit("push", "-u", remote, branch)
}

func (a *ExecAdapter) Pull() (string, error) { return a.runGit("pull") }

func (a *ExecAdapter) PullFrom(remote string) (string, error) {
	if remote == "" {
		return a.runGit("pull")
	}
	return a.runGit("pull", remote)
}

func (a *ExecAdapter) PushTags() (string, error) { return a.runGit("push", "--tags") }

func (a *ExecAdapter) Fetch() (string, error) { return a.runGit("fetch", "--all") }

func (a *ExecAdapter) DeleteRemoteBranch(name string) error {
	_, err := a.runGit("push", "origin", "--delete", name)
	return err
}
