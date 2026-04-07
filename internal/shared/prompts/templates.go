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

// Template names
const (
	RegenerateMessage = "regenerate_message.txt"
	BranchName        = "branch_name.txt"
	GenerateMessage   = "generate_message.txt"
	DecideCommit      = "decide_commit.txt"
)

// BuildMessageParams creates the parameter map for the generate_message template.
func BuildMessageParams(files []string, diff string) map[string]string {
	return map[string]string{
		"files": strings.Join(files, ", "),
		"diff":  diff,
	}
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
