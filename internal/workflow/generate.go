package workflow

import (
	"context"
	"fmt"
)

// generate calls the LLM to interpret the user instruction and generate a plan.
func (w *Workflow) generate(_ context.Context, op, instruction string, prep PrepContext, explicitArgs map[string]string) (map[string]string, string, error) {
	// If args are provided explicitly, skip LLM
	if len(explicitArgs) > 0 && fullyProvided(op, explicitArgs) {
		return explicitArgs, buildPreview(op, explicitArgs), nil
	}

	args, err := w.llm.InterpretGitOp(op, instruction, prep.toContextMap())
	if err != nil {
		return nil, "", fmt.Errorf("failed to get LLM decision: %w", err)
	}

	// Merge explicit args on top of LLM-generated ones
	for k, v := range explicitArgs {
		if v != "" {
			args[k] = v
		}
	}

	// Post-process args for stability
	if op == "branch_rename" {
		if args["name"] == "" || args["name"] == args["new_name"] {
			args["name"] = prep.CurrentBranch
		}
	}
	if op == "merge" {
		if args["target"] == "" {
			args["target"] = prep.CurrentBranch
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

func requiredArgs(op string) []string {
	switch op {
	case "branch_create":
		return []string{"branch"}
	case "branch_delete":
		return []string{"branch"}
	case "branch_rename":
		return []string{"name", "new_name"}
	case "merge":
		return []string{"source"}
	case "tag_create":
		return []string{"tag"}
	default:
		return nil
	}
}

// buildPreview returns a human-readable description of what will be executed.
func buildPreview(op string, args map[string]string) string {
	switch op {
	case "commit":
		return "Operation: commit"
	case "branch_create":
		return fmt.Sprintf("Operation: branch_create (branch: %s)", args["branch"])
	case "branch_rename":
		return fmt.Sprintf("Operation: branch_rename (from: %s, to: %s)", args["name"], args["new_name"])
	case "branch_delete":
		return fmt.Sprintf("Operation: branch_delete (branch: %s)", args["branch"])
	case "release":
		return fmt.Sprintf("Operation: release (version: %s)", args["version"])
	case "merge":
		return fmt.Sprintf("Operation: merge (source: %s, target: %s)", args["source"], args["target"])
	case "push":
		return fmt.Sprintf("Operation: push (remote: %s, branch: %s)", args["remote"], args["branch"])
	case "pull":
		return fmt.Sprintf("Operation: pull (remote: %s, branch: %s)", args["remote"], args["branch"])
	default:
		if val, ok := args["tag"]; ok {
			return fmt.Sprintf("Operation: %s (tag: %s)", op, val)
		}
		return fmt.Sprintf("Operation: %s", op)
	}
}
