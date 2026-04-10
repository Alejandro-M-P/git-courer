package workflow

import (
	"context"
	"fmt"
)

// deterministicOps are review ops that don't need LLM interpretation.
// The user either provides args directly or the operation has no args.
var deterministicOps = map[string]bool{
	"rebase_continue": true,
	"rebase_abort":    true,
	"init":            true,
}

// generate calls the LLM to interpret the instruction and returns concrete args + a preview string.
// For deterministic ops, it returns the instruction's explicit args without calling the LLM.
func (w *Workflow) generate(_ context.Context, op, instruction string, prep PrepContext, explicitArgs map[string]string) (map[string]string, string, error) {
	// Deterministic ops skip LLM — args come from the user directly.
	if deterministicOps[op] {
		return explicitArgs, buildPreview(op, explicitArgs), nil
	}

	// If all needed args are already provided, skip LLM.
	if fullyProvided(op, explicitArgs) {
		return explicitArgs, buildPreview(op, explicitArgs), nil
	}

	// Call LLM to interpret the natural language instruction.
	args, err := w.llm.InterpretGitOp(op, instruction, prep.toContextMap())
	if err != nil {
		return nil, "", fmt.Errorf("LLM failed to interpret %q: %w", op, err)
	}

	// Merge explicit args (user-provided take priority over LLM).
	for k, v := range explicitArgs {
		if v != "" {
			args[k] = v
		}
	}

	return args, buildPreview(op, args), nil
}

// fullyProvided returns true if the op has all its required args already.
func fullyProvided(op string, args map[string]string) bool {
	required := requiredArgs(op)
	for _, key := range required {
		if args[key] == "" {
			return false
		}
	}
	return len(required) > 0
}

// requiredArgs returns the arg keys that an op needs to execute.
func requiredArgs(op string) []string {
	switch op {
	case "branch_create", "branch_delete", "merge", "rebase":
		return []string{"branch"}
	case "branch_rename":
		return []string{"old_name", "new_name"}
	case "cherry_pick", "revert":
		return []string{"commit"}
	case "reset_hard":
		return []string{"target"}
	case "tag_create", "tag_delete":
		return []string{"name"}
	case "remote_add":
		return []string{"name", "url"}
	case "remote_remove":
		return []string{"name"}
	case "clone":
		return []string{"url"}
	default:
		return nil
	}
}

// buildPreview returns a human-readable description of what will be executed.
func buildPreview(op string, args map[string]string) string {
	switch op {
	case "branch_create":
		return fmt.Sprintf("Create branch: %s", args["branch"])
	case "branch_delete":
		return fmt.Sprintf("Delete branch: %s", args["branch"])
	case "branch_rename":
		return fmt.Sprintf("Rename branch: %s → %s", args["old_name"], args["new_name"])
	case "merge":
		return fmt.Sprintf("Merge branch: %s", args["branch"])
	case "rebase":
		return fmt.Sprintf("Rebase onto: %s", args["branch"])
	case "rebase_continue":
		return "Continue rebase after resolving conflicts"
	case "rebase_abort":
		return "Abort current rebase"
	case "reset_hard":
		return fmt.Sprintf("Hard reset to: %s", args["target"])
	case "cherry_pick":
		return fmt.Sprintf("Cherry-pick commit: %s", args["commit"])
	case "revert":
		return fmt.Sprintf("Revert commit: %s", args["commit"])
	case "clean":
		return "Remove untracked files"
	case "tag_create":
		return fmt.Sprintf("Create tag: %s", args["name"])
	case "tag_delete":
		return fmt.Sprintf("Delete tag: %s", args["name"])
	case "remote_add":
		return fmt.Sprintf("Add remote: %s → %s", args["name"], args["url"])
	case "remote_remove":
		return fmt.Sprintf("Remove remote: %s", args["name"])
	case "clone":
		return fmt.Sprintf("Clone: %s", args["url"])
	case "init":
		return "Initialize new git repository"
	default:
		return op
	}
}
