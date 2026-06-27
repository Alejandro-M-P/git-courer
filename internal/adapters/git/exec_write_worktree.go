package git

// AddWorktree creates a linked git worktree at worktreePath bound to branch.
// It runs `git worktree add <worktreePath> <branch>`. The caller is responsible
// for ensuring the branch ref already exists (e.g. via CreateRef) before calling
// this method, matching the worktree add semantics where the branch must exist.
// Returns the worktreePath on success.
func (a *ExecAdapter) AddWorktree(worktreePath, branch string) (string, error) {
	_, err := a.runGit("worktree", "add", worktreePath, branch)
	if err != nil {
		return "", err
	}
	return worktreePath, nil
}

// RemoveWorktree removes a linked git worktree from disk. It runs
// `git worktree remove --force <worktreePath>` so it removes the worktree even
// when there are untracked files or modifications (used for session discard and
// rollback paths).
func (a *ExecAdapter) RemoveWorktree(worktreePath string) error {
	_, err := a.runGit("worktree", "remove", "--force", worktreePath)
	return err
}

// CreateRef atomically creates a git ref pointing at commitHash. It runs
// `git update-ref <ref> <commitHash> ""` — the empty old-oid argument makes git
// reject the update when the ref already exists, so collision detection is
// delegated to git itself (no TOCTOU window). Returns an error if the ref already
// existed or the commit hash is invalid.
func (a *ExecAdapter) CreateRef(ref, commitHash string) error {
	_, err := a.runGit("update-ref", ref, commitHash, "")
	return err
}