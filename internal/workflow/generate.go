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
		return "📦 Preparing an atomic commit. I've analyzed the diff and generated a message based on the actual logic of your changes."
	case "branch_create":
		return fmt.Sprintf("✨ Creating branch %q. I've followed naming conventions to keep your repo organized.", args["branch"])
	case "branch_rename":
		return fmt.Sprintf("📝 Renaming branch from %q to %q. Using clear names makes team collaboration much smoother.", args["name"], args["new_name"])
	case "branch_delete":
		return fmt.Sprintf("🗑️ Deleting branch %q. Cleaning up old branches prevents project entropy.", args["branch"])
	case "release":
		return fmt.Sprintf("🚀 Preparing version %s. Generating a polished changelog so your users understand the added value.", args["version"])
	case "merge":
		return fmt.Sprintf("🔀 Merging %q into %q. Safely integrating workflows.", args["source"], args["target"])
	case "push":
		force := ""
		if args["force"] == "true" {
			force = " (Caution: Force mode enabled!)"
		}
		return fmt.Sprintf("⬆️ Pushing changes to %s/%s%s. Syncing your hard work with the team.", args["remote"], args["branch"], force)
	case "pull":
		return fmt.Sprintf("⬇️ Pulling changes from %s/%s. Staying up-to-date avoids future conflicts.", args["remote"], args["branch"])
	case "tag_create":
		return fmt.Sprintf("Create tag: %s", args["tag"])
	case "tag_delete":
		return fmt.Sprintf("Delete tag: %s", args["tag"])
	case "tag_push":
		return fmt.Sprintf("Push tag: %s", args["tag"])
	case "tag_delete_remote":
		return fmt.Sprintf("Delete remote tag: %s", args["tag"])
	default:
		return "🔨 Preparing operation: " + op
	}
}
