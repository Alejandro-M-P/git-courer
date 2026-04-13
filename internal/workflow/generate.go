package workflow

import (
	"context"
	"fmt"
)

// generate calls the LLM to interpret the instruction and returns concrete args + a preview string.
func (w *Workflow) generate(_ context.Context, op, instruction string, prep PrepContext, explicitArgs map[string]string) (map[string]string, string, error) {
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
	case "branch_create", "branch_delete":
		return []string{"branch"}
	case "branch_rename":
		return []string{"old_name", "new_name"}
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
	case "release":
		return fmt.Sprintf("Create release: %s", args["version"])
	case "push":
		return "Push to remote"
	case "pull":
		return "Pull from remote"
	default:
		return op
	}
}
