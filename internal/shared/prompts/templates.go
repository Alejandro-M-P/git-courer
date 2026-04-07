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
	CommitMessageShort  = "commit_message_short.txt"
	CommitMessageMedium = "commit_message_medium.txt"
	CommitMessageLarge  = "commit_message_large.txt"
	RegenerateMessage   = "regenerate_message.txt"
	AnalyzeAndPlan      = "analyze_and_plan.txt"
	AnalyzeAndPlanDeep  = "analyze_and_plan_deep.txt"
	BranchName          = "branch_name.txt"
	GroupFiles          = "group_files.txt"
	GenerateMessage     = "generate_message.txt"
	DecideCommit        = "decide_commit.txt"
)

// ComplexityThreshold defines when to switch from single-call to 2-phase mode.
// 2-phase mode: Phase 1 groups files, Phase 2 generates messages per group.
const (
	DiffThresholdSingleCall = 4000 // chars — below this, single call is fine
	MaxFilesSingleCall      = 10   // files — below this, single call is fine
)

// ShouldUseTwoPhaseMode returns true when changes are too large for a single call.
// In 2-phase mode: first group files, then generate messages per group.
func ShouldUseTwoPhaseMode(numFiles int, diffSize int) bool {
	return diffSize > DiffThresholdSingleCall || numFiles > MaxFilesSingleCall
}

// ShouldUseReasoningMode returns true when the changes are complex enough
// to warrant enabling the model's reasoning/thinking mode.
func ShouldUseReasoningMode(numFiles int, diffSize int) bool {
	return diffSize > DiffThresholdSingleCall || numFiles > MaxFilesSingleCall
}

// BuildGroupParams creates the parameter map for the group_files template.
func BuildGroupParams(files []string, diffStat string) map[string]string {
	return map[string]string{
		"files":      strings.Join(files, ", "),
		"diff_stat":  diffStat,
		"file_count": fmt.Sprintf("%d", len(files)),
		"diff_size":  fmt.Sprintf("%d", len(diffStat)),
	}
}

// BuildMessageParams creates the parameter map for the generate_message template.
func BuildMessageParams(files []string, diff string) map[string]string {
	return map[string]string{
		"files": strings.Join(files, ", "),
		"diff":  diff,
	}
}

// BuildAnalyzeParams creates the parameter map for the analyze templates.
func BuildAnalyzeParams(files []string, diff string) map[string]string {
	return map[string]string{
		"files":      strings.Join(files, ", "),
		"diff":       diff,
		"file_count": fmt.Sprintf("%d", len(files)),
		"diff_size":  fmt.Sprintf("%d", len(diff)),
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

// SelectCommitMessageTemplate returns the appropriate commit message template
// based on the number of files being committed.
func SelectCommitMessageTemplate(numFiles int) string {
	switch {
	case numFiles <= 2:
		return CommitMessageShort
	case numFiles <= 5:
		return CommitMessageMedium
	default:
		return CommitMessageLarge
	}
}
