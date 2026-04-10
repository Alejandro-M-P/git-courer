package workflow

import (
	"context"
	"fmt"
	"strconv"
)

// execute dispatches the git command for the given operation with the resolved args.
func (w *Workflow) execute(_ context.Context, op string, args map[string]string) (string, error) {
	switch op {
	case "branch_create":
		return w.git.Branch(args["branch"])

	case "branch_delete":
		return w.git.DeleteBranch(args["branch"])

	case "branch_rename":
		return w.git.RenameBranch(args["old_name"], args["new_name"])

	case "merge":
		return w.git.Merge(args["branch"])

	case "rebase":
		return w.git.Rebase(args["branch"])

	case "rebase_continue":
		return w.git.RebaseContinue()

	case "rebase_abort":
		return w.git.RebaseAbort()

	case "reset_hard":
		target := args["target"]
		if target == "" {
			target = "HEAD~1"
		}
		return w.git.Reset("--hard", target)

	case "reset_soft":
		n := 1
		if v := args["commits"]; v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
				n = parsed
			}
		}
		err := w.git.ResetSoft(n)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Soft reset of %d commit(s) done", n), nil

	case "cherry_pick":
		return w.git.CherryPick(args["commit"])

	case "revert":
		return w.git.Revert(args["commit"])

	case "clean":
		dirs := args["dirs"] == "true"
		return w.git.Clean(dirs)

	case "tag_create":
		return w.git.Tag(args["name"])

	case "tag_delete":
		return w.git.DeleteTag(args["name"])

	case "remote_add":
		return w.git.AddRemote(args["name"], args["url"])

	case "remote_remove":
		return w.git.RemoveRemote(args["name"])

	case "clone":
		return w.git.Clone(args["url"])

	case "init":
		return w.git.Init()

	default:
		return "", fmt.Errorf("unknown workflow operation: %q", op)
	}
}
