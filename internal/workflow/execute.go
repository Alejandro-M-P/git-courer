package workflow

import (
	"context"
	"fmt"
)

// execute dispatches the git command for the given operation with the resolved args.
func (w *Workflow) execute(_ context.Context, op string, args map[string]string) (string, error) {
	if !w.cfg.Commands.IsEnabled(op) {
		return "", fmt.Errorf("operación '%s' deshabilitada por configuración", op)
	}

	switch op {
	case "commit":
		// The CommitService handles the complex logic of chunking, messaging and executing.
		// Since we already did the preview/confirm phase in Run(), here we execute.
		// But wait: CommitService expects a direct call. Let's make it idiomatic.
		return w.commitSvc.Execute(args["instruction"], false)

	case "branch_create":
		return w.git.Branch(args["branch"])

	case "branch_delete":
		return w.git.DeleteBranch(args["branch"])

	case "branch_rename":
		return w.git.RenameBranch(args["name"], args["new_name"])

	case "release":
		return "", fmt.Errorf("release debe ejecutarse mediante ReleaseService")

	case "merge":
		return w.git.Merge(args["branch"])

	case "push":
		return w.git.Push()

	case "pull":
		return w.git.Pull()

	case "tag_create":
		return w.git.Tag(args["tag"], "")
	case "tag_delete":
		return w.git.DeleteTag(args["tag"])

	case "tag_push":
		return w.git.PushTag(args["tag"])

	case "tag_delete_remote":
		return w.git.DeleteTagRemote(args["tag"])

	default:
		return "", fmt.Errorf("unknown workflow operation: %q", op)
	}
}
