package workflow

import (
	"context"
	"strings"
)

// PrepContext holds the git context gathered before calling the LLM.
// Each operation populates only the fields it needs.
type PrepContext struct {
	CurrentBranch string
	Branches      string
	Tags          string
	Log           string
	UntrackedList string
}

// prepare gathers the git context required for the given operation.
func (w *Workflow) prepare(_ context.Context, op string) (PrepContext, error) {
	var ctx PrepContext
	var err error

	switch op {
	case "branch_create", "branch_rename", "merge":
		ctx.CurrentBranch, _ = w.git.CurrentBranch()
		ctx.Branches, err = w.git.ListBranches()

	case "branch_delete":
		ctx.Branches, err = w.git.ListBranches()

	case "rebase":
		ctx.CurrentBranch, _ = w.git.CurrentBranch()
		ctx.Branches, _ = w.git.ListBranches()
		ctx.Log, err = w.git.Log(10)

	case "reset_hard", "cherry_pick", "revert":
		ctx.Log, err = w.git.Log(20)

	case "tag_create":
		ctx.Log, _ = w.git.Log(10)
		tags, _ := w.git.ListTags()
		ctx.Tags = strings.Join(tags, "\n")

	case "tag_delete":
		tags, _ := w.git.ListTags()
		ctx.Tags = strings.Join(tags, "\n")

	case "clean":
		status, sErr := w.git.Status()
		if sErr != nil {
			return ctx, sErr
		}
		var untracked []string
		for _, f := range status.Files {
			if strings.HasPrefix(f.Status, "??") {
				untracked = append(untracked, f.Path)
			}
		}
		ctx.UntrackedList = strings.Join(untracked, "\n")

	case "remote_add", "remote_remove", "clone", "init",
		"rebase_continue", "rebase_abort":
		// No context needed — either deterministic or user provides all args.
	}

	return ctx, err
}

// toContextMap converts PrepContext to the map format expected by LLM.InterpretGitOp.
func (p PrepContext) toContextMap() map[string]string {
	m := make(map[string]string, 4)
	if p.CurrentBranch != "" {
		m["current_branch"] = p.CurrentBranch
	}
	if p.Branches != "" {
		m["branches"] = p.Branches
	}
	if p.Tags != "" {
		m["tags"] = p.Tags
	}
	if p.Log != "" {
		m["recent_commits"] = p.Log
	}
	if p.UntrackedList != "" {
		m["untracked_files"] = p.UntrackedList
	}
	return m
}
