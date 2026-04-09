// Package prompts provides template management for LLM prompts.
// It loads prompt templates from embedded files and substitutes placeholders.
package prompts

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed *.txt
var templatesFS embed.FS

// Template names — commit
const (
	RegenerateMessage = "regenerate_message.txt"
	BranchName        = "branch_name.txt"
	GenerateMessage   = "generate_message.txt"
	DecideCommit      = "decide_commit.txt"
)

// Template names — git operations (one per operation that needs LLM interpretation)
const (
	GitOpBranchCreate  = "git_op_branch_create.txt"
	GitOpBranchDelete  = "git_op_branch_delete.txt"
	GitOpBranchRename  = "git_op_branch_rename.txt"
	GitOpMerge         = "git_op_merge.txt"
	GitOpRebase        = "git_op_rebase.txt"
	GitOpResetHard     = "git_op_reset_hard.txt"
	GitOpResetSoft     = "git_op_reset_soft.txt"
	GitOpTagCreate     = "git_op_tag_create.txt"
	GitOpTagDelete     = "git_op_tag_delete.txt"
	GitOpCherryPick    = "git_op_cherry_pick.txt"
	GitOpRevert        = "git_op_revert.txt"
	GitOpRemoteAdd     = "git_op_remote_add.txt"
	GitOpRemoteRemove  = "git_op_remote_remove.txt"
	GitOpClone         = "git_op_clone.txt"
)

// GitOpTemplate maps each operation name to its prompt template file.
// Operations not listed here don't require LLM interpretation (no args needed).
var GitOpTemplate = map[string]string{
	"BRANCH_CREATE":  GitOpBranchCreate,
	"BRANCH_DELETE":  GitOpBranchDelete,
	"BRANCH_RENAME":  GitOpBranchRename,
	"MERGE":          GitOpMerge,
	"REBASE":         GitOpRebase,
	"RESET_HARD":     GitOpResetHard,
	"RESET_SOFT":     GitOpResetSoft,
	"TAG_CREATE":     GitOpTagCreate,
	"TAG_DELETE":     GitOpTagDelete,
	"CHERRY_PICK":    GitOpCherryPick,
	"REVERT":         GitOpRevert,
	"REMOTE_ADD":     GitOpRemoteAdd,
	"REMOTE_REMOVE":  GitOpRemoteRemove,
	"CLONE":          GitOpClone,
}

// BuildMessageParams creates the parameter map for the generate_message template.
func BuildMessageParams(files []string, diff string) map[string]string {
	return map[string]string{
		"files": strings.Join(files, ", "),
		"diff":  diff,
	}
}

// BuildMessageParamsWithRetry creates the parameter map with retry context.
func BuildMessageParamsWithRetry(files []string, diff string, previousMessage string) map[string]string {
	params := BuildMessageParams(files, diff)
	if previousMessage != "" {
		params["previous_message"] = previousMessage
	}
	return params
}

// BuildDecideParams creates the parameter map for the decide_commit template.
func BuildDecideParams(instruction, gitStatus, untracked, modified, deleted string) map[string]string {
	return map[string]string{
		"instruction": instruction,
		"git_status":  gitStatus,
		"untracked":   untracked,
		"modified":    modified,
		"deleted":     deleted,
	}
}

// Render loads a template by name and substitutes all {{.key}} placeholders
// with values from the params map.
func Render(name string, params map[string]string) (string, error) {
	data, err := templatesFS.ReadFile(name)
	if err != nil {
		return "", fmt.Errorf("prompt template %q not found: %w", name, err)
	}

	result := string(data)
	for key, value := range params {
		placeholder := "{{." + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result, nil
}
