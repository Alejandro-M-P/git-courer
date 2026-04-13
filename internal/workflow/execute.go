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
	case "branch_create":
		return w.git.Branch(args["branch"])

	case "branch_delete":
		return w.git.DeleteBranch(args["branch"])

	case "branch_rename":
		return w.git.RenameBranch(args["old_name"], args["new_name"])

	case "release":
		// Release uses ReleaseService, not the generic workflow execute
		return "", fmt.Errorf("release debe ejecutarse через ReleaseService")

	case "push":
		return w.git.Push()

	case "pull":
		return w.git.Pull()

	default:
		return "", fmt.Errorf("unknown workflow operation: %q", op)
	}
}
