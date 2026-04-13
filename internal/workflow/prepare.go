package workflow

import (
	"context"
	"strings"
)

// PrepContext holds the git context gathered before calling the LLM.
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
	case "branch_create", "branch_rename":
		ctx.CurrentBranch, _ = w.git.CurrentBranch()
		ctx.Branches, err = w.git.ListBranches()

	case "branch_delete":
		ctx.Branches, err = w.git.ListBranches()

	case "release":
		ctx.CurrentBranch, _ = w.git.CurrentBranch()
		ctx.Branches, _ = w.git.ListBranches()
		if tags, err := w.git.ListTags(); err == nil {
			ctx.Tags = strings.Join(tags, "\n")
		}
		ctx.Log, _ = w.git.Log(50)

	case "push", "pull":
		ctx.CurrentBranch, _ = w.git.CurrentBranch()
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
