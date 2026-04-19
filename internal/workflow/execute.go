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

	case "release":
		return "", fmt.Errorf("release debe ejecutarse через ReleaseService")

	case "merge":
		return w.git.Merge(args["branch"])

	case "tag_create":
		return w.git.Tag(args["tag"])

	case "tag_delete":
		return w.git.DeleteTag(args["tag"])

	default:
		return "", fmt.Errorf("unknown workflow operation: %q", op)
	}
}
