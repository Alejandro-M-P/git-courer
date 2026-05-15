package mcp

import (
	"strings"
	"testing"
)

// TestInstructions_DocumentsAllTools verifies every registered tool has a description.
func TestInstructions_DocumentsAllTools(t *testing.T) {
	expectedTools := []string{
		"status", "diff", "commit", "amend", "revert",
		"branch", "merge", "rebase", "tag", "cherry_pick",
		"stage", "reset", "stash",
		"history", "blame",
		"sync", "remotes",
		"config", "backup", "release",
	}

	for _, name := range expectedTools {
		desc := toolDescription(name)
		if desc == "" {
			t.Errorf("missing description for tool %q", name)
		}
	}
}

// TestInstructions_NoStaleGitPrefix ensures no description references git_-prefixed tool names.
func TestInstructions_NoStaleGitPrefix(t *testing.T) {
	allDesc := allDescriptions()
	stalePrefixes := []string{
		"git_status", "git_diff", "git_branch", "git_stage",
		"git_stash", "git_sync", "git_tag", "git_backup",
		"git_config", "git_log", "git_review", "git_revert",
		"git_amend", "git_commit", "git_merge", "git_rebase",
		"git_reset", "git_cherry_pick", "git_remotes", "git_release",
		"git_blame", "git_history", "git_pr_review",
	}
	for _, desc := range allDesc {
		for _, prefix := range stalePrefixes {
			if strings.Contains(desc, prefix) {
				t.Errorf("stale git_ prefix %q found in description", prefix)
			}
		}
	}
}

// TestInstructions_SummaryReferencesRealToolNames ensures the summary block
// references each tool by its current name.
func TestInstructions_SummaryReferencesRealToolNames(t *testing.T) {
	expected := []string{
		"status", "diff", "commit", "amend", "revert",
		"branch", "merge", "rebase", "tag", "cherry_pick",
		"stage", "reset", "stash",
		"history", "blame",
		"sync", "remotes",
		"config", "backup", "release",
	}
	for _, name := range expected {
		if !strings.Contains(gitCourerSummary, name) {
			t.Errorf("summary missing reference to tool %q", name)
		}
	}
}

// TestInstructions_CommitDescribesPreviewApply ensures commit description
// mentions PREVIEW/APPLY workflow and job_id.
func TestInstructions_CommitDescribesPreviewApply(t *testing.T) {
	desc := toolDescription("commit")
	if !strings.Contains(desc, "PREVIEW") {
		t.Error("commit description missing PREVIEW workflow")
	}
	if !strings.Contains(desc, "APPLY") {
		t.Error("commit description missing APPLY workflow")
	}
	if !strings.Contains(strings.ToLower(desc), "job_id") {
		t.Error("commit description missing job_id reference")
	}
}

// TestInstructions_ConflictToolsDescribeStructuredError ensures merge/rebase
// document structured conflict JSON.
func TestInstructions_ConflictToolsDescribeStructuredError(t *testing.T) {
	for _, name := range []string{"merge", "rebase"} {
		desc := toolDescription(name)
		if !strings.Contains(strings.ToLower(desc), "conflict") {
			t.Errorf("%s description missing conflict mention", name)
		}
		if !strings.Contains(desc, "conflicted_files") {
			t.Errorf("%s description missing conflicted_files return shape", name)
		}
	}
}

// TestInstructions_NoFalsePromises ensures no description claims GitHub API support.
func TestInstructions_NoFalsePromises(t *testing.T) {
	allDesc := allDescriptions()
	for _, desc := range allDesc {
		if strings.Contains(desc, "GitHub API") || strings.Contains(desc, "git_courer_create_pr") {
			t.Errorf("description falsely claims GitHub API support")
		}
	}
}

// TestInstructions_DescribesReturnShapes ensures core tools mention
// return shapes or JSON.
func TestInstructions_DescribesReturnShapes(t *testing.T) {
	status := toolDescription("status")
	if !strings.Contains(strings.ToLower(status), "returns") && !strings.Contains(strings.ToLower(status), "json") {
		t.Error("status description should mention return shape")
	}

	diff := toolDescription("diff")
	if !strings.Contains(strings.ToLower(diff), "returns") && !strings.Contains(strings.ToLower(diff), "pagination") {
		t.Error("diff description should mention return shape or pagination")
	}
}

// TestInstructions_DoesntClaimBuiltInAST ensures instructions honestly
// disclose the nature of the analysis (not built-in).
func TestInstructions_DoesntClaimBuiltInAST(t *testing.T) {
	allDesc := allDescriptions()
	for _, desc := range allDesc {
		if strings.Contains(desc, "built-in AST") || strings.Contains(desc, "built-in tree-sitter") {
			t.Errorf("description falsely claims built-in AST")
		}
	}
}

// TestInstructions_NoShowSearchTools ensures show and search are not documented.
func TestInstructions_NoShowSearchTools(t *testing.T) {
	if toolDescription("show") != "" {
		t.Error("show tool should not have a description — it was removed")
	}
	if toolDescription("search") != "" {
		t.Error("search tool should not have a description — it was removed")
	}
}

// TestInstructions_LLMToolsExplainPainPoint ensures all LLM-driven tools
// explain what the LLM CANNOT do with raw git or why git-courer is better.
func TestInstructions_LLMToolsExplainPainPoint(t *testing.T) {
	llmTools := []string{
		"status", "diff", "commit", "amend", "revert",
		"branch", "merge", "rebase", "stage", "reset", "stash",
		"history", "sync", "blame",
	}
	for _, name := range llmTools {
		desc := toolDescription(name)
		hasWhy := strings.Contains(desc, "WHY NOT bash") || strings.Contains(desc, "WHY NOT")
		hasCannot := strings.Contains(strings.ToLower(desc), "cannot") || strings.Contains(strings.ToLower(desc), "can't")
		hasReturn := strings.Contains(desc, "Returns")
		if !hasWhy && !hasCannot && !hasReturn {
			t.Errorf("%s description should explain pain point (WHY NOT bash / cannot / Returns)", name)
		}
	}
}

// --- Helpers ---

func toolDescription(name string) string {
	switch name {
	case "status":
		return descStatus
	case "diff":
		return descDiff
	case "commit":
		return descCommit
	case "amend":
		return descAmend
	case "revert":
		return descRevert
	case "branch":
		return descBranch
	case "merge":
		return descMerge
	case "rebase":
		return descRebase
	case "tag":
		return descTag
	case "cherry_pick":
		return descCherryPick
	case "stage":
		return descStage
	case "reset":
		return descReset
	case "stash":
		return descStash
	case "history":
		return descHistory
	case "blame":
		return descBlame
	case "sync":
		return descSync
	case "remotes":
		return descRemotes
	case "config":
		return descConfig
	case "backup":
		return descBackup
	case "release":
		return descRelease
	default:
		return ""
	}
}

func allDescriptions() []string {
	names := []string{
		"status", "diff", "commit", "amend", "revert",
		"branch", "merge", "rebase", "tag", "cherry_pick",
		"stage", "reset", "stash",
		"history", "blame",
		"sync", "remotes",
		"config", "backup", "release",
	}
	var result []string
	for _, n := range names {
		if d := toolDescription(n); d != "" {
			result = append(result, d)
		}
	}
	return result
}