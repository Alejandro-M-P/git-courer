package mcp

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
)

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

// TestInstructions_NoStaleGitPrefix ensures no description references git_-prefixed tool names.
func TestInstructions_NoStaleGitPrefix(t *testing.T) {
	allDesc := allToolDescriptions()
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

// TestInstructions_EveryToolHasDescription ensures all registered tools have non-empty descriptions.
func TestInstructions_EveryToolHasDescription(t *testing.T) {
	expectedTools := []string{
		"status", "diff", "commit", "amend", "revert",
		"branch", "merge", "rebase", "tag", "cherry_pick",
		"stage", "reset", "stash",
		"history", "blame",
		"sync", "remotes",
		"config", "backup", "release",
	}

	for _, name := range expectedTools {
		desc := toolDescriptionFromSchema(name)
		if desc == "" {
			t.Errorf("missing description for tool %q", name)
		}
	}
}

// TestInstructions_NoShowSearchTools ensures show and search are not documented.
func TestInstructions_NoShowSearchTools(t *testing.T) {
	if toolDescriptionFromSchema("show") != "" {
		t.Error("show tool should not have a description — it was removed")
	}
	if toolDescriptionFromSchema("search") != "" {
		t.Error("search tool should not have a description — it was removed")
	}
}

// --- Helpers ---

// toolDescriptionFromSchema looks up a tool's description from its registered schema.
func toolDescriptionFromSchema(name string) string {
	mcpSrv := server.NewMCPServer("test", "1.0")
	srv := &Server{}
	registerTools(mcpSrv, srv)

	tool := findTool(mcpSrv, name)
	if tool == nil {
		return ""
	}
	return tool.Description
}

func allToolDescriptions() []string {
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
		if d := toolDescriptionFromSchema(n); d != "" {
			result = append(result, d)
		}
	}
	return result
}