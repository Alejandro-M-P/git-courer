package git

import (
	"fmt"
	"strings"
)

func (a *ExecAdapter) Stash(message ...string) (string, error) {
	msg := "git-courer stash"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	return a.runGit("stash", "push", "-m", msg)
}

func (a *ExecAdapter) StashPop() (string, error) { return a.runGit("stash", "pop") }

func (a *ExecAdapter) StashApply(index string) (string, error) {
	if index == "" {
		return a.runGit("stash", "apply")
	}
	return a.runGit("stash", "apply", normalizeStashRef(index))
}

func (a *ExecAdapter) StashDrop(index string) (string, error) {
	if index == "" {
		return a.runGit("stash", "drop")
	}
	return a.runGit("stash", "drop", normalizeStashRef(index))
}

func (a *ExecAdapter) StashClear() (string, error) { return a.runGit("stash", "clear") }

func (a *ExecAdapter) StashShow() (string, error) {
	return a.runGit("stash", "show")
}

func normalizeStashRef(index string) string {
	if strings.HasPrefix(index, "stash@{") {
		return index
	}
	return fmt.Sprintf("stash@{%s}", index)
}
