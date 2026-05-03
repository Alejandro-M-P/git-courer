package git

func (a *ExecAdapter) Stash(message ...string) (string, error) {
	msg := "git-courer stash"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	return a.runGit("stash", "push", "-m", msg)
}

func (a *ExecAdapter) StashPop() (string, error) { return a.runGit("stash", "pop") }
