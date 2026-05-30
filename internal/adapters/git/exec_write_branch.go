package git

func (a *ExecAdapter) Branch(name string) (string, error) {
	return a.runGit("branch", name)
}

func (a *ExecAdapter) RenameBranch(oldName, newName string) (string, error) {
	return a.runGit("branch", "-m", oldName, newName)
}

func (a *ExecAdapter) DeleteBranch(name string, force bool) (string, error) {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return a.runGit("branch", flag, name)
}

func (a *ExecAdapter) SetUpstream(branch, remote string) (string, error) {
	return a.runGit("branch", "-u", remote+"/"+branch, branch)
}

func (a *ExecAdapter) UnsetUpstream(branch string) (string, error) {
	return a.runGit("branch", "--unset-upstream", branch)
}

func (a *ExecAdapter) Switch(name string) error {
	_, err := a.runGit("switch", name)
	return err
}

func (a *ExecAdapter) Checkout(name string) (string, error) { return a.runGit("checkout", name) }
