package mcp

import (
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// registerAllToolsForTest creates an MCP server with all tools registered
// and returns it for schema inspection.
func registerAllToolsForTest() *server.MCPServer {
	s := server.NewMCPServer("test", "1.0")
	srv := &Server{}
	registerTools(s, srv)
	return s
}

// toolDescriptionFromSchema looks up a tool's description from its registered schema.
func toolDescriptionFromSchema(t *testing.T, mcpSrv *server.MCPServer, name string) string {
	t.Helper()
	tools := mcpSrv.ListTools()
	st, ok := tools[name]
	require.True(t, ok, "tool %q should be registered", name)
	return st.Tool.Description
}

// allSchemaDescriptions returns all registered tool descriptions.
func allSchemaDescriptions(t *testing.T, mcpSrv *server.MCPServer) []string {
	t.Helper()
	tools := mcpSrv.ListTools()
	var result []string
	for _, st := range tools {
		if st.Tool.Description != "" {
			result = append(result, st.Tool.Description)
		}
	}
	return result
}

// TestToolDescriptions_EveryToolHasDescription verifies every registered tool
// has a non-empty description in its MCP schema.
func TestToolDescriptions_EveryToolHasDescription(t *testing.T) {
	mcpSrv := registerAllToolsForTest()
	expectedTools := []string{
		"status", "diff", "commit", "amend", "revert",
		"branch", "merge", "rebase", "tag", "cherry_pick",
		"stage", "reset", "stash",
		"history", "blame",
		"sync", "remotes", "pr-review",
		"config", "backup", "undo", "commit-jobs",
	}

	for _, name := range expectedTools {
		desc := toolDescriptionFromSchema(t, mcpSrv, name)
		assert.NotEmpty(t, desc, "tool %q should have a description in its MCP schema", name)
	}
}

// TestToolDescriptions_DescribesSafetyBehavior ensures destructive tools mention
// confirmation requirements or safety in their descriptions.
func TestToolDescriptions_DescribesSafetyBehavior(t *testing.T) {
	mcpSrv := registerAllToolsForTest()
	destructiveTools := []string{"amend", "revert", "reset", "branch", "sync", "tag"}

	for _, name := range destructiveTools {
		desc := toolDescriptionFromSchema(t, mcpSrv, name)
		lower := strings.ToLower(desc)
		assert.True(t,
			strings.Contains(lower, "confirmed") || strings.Contains(lower, "blocked") || strings.Contains(lower, "require"),
			"%q description should mention confirmation/safety requirement", name,
		)
	}
}

// TestToolDescriptions_ExplainsWhenToUse ensures tools with common alternatives
// explain when to prefer this tool vs the alternative.
func TestToolDescriptions_ExplainsWhenToUse(t *testing.T) {
	mcpSrv := registerAllToolsForTest()
	// amend should mention commit as alternative
	amend := toolDescriptionFromSchema(t, mcpSrv, "amend")
	assert.True(t, strings.Contains(strings.ToLower(amend), "commit") || strings.Contains(strings.ToLower(amend), "do not"),
		"amend description should mention when NOT to use amend")

	// diff should mention when to use it (before push, before PR)
	diff := toolDescriptionFromSchema(t, mcpSrv, "diff")
	assert.True(t, strings.Contains(strings.ToLower(diff), "before") || strings.Contains(strings.ToLower(diff), "review"),
		"diff description should mention when to use it")

	// reset should mention amend as alternative
	reset := toolDescriptionFromSchema(t, mcpSrv, "reset")
	assert.True(t, strings.Contains(strings.ToLower(reset), "amend") || strings.Contains(strings.ToLower(reset), "safest"),
		"reset description should explain safety levels or alternatives")
}

// TestToolDescriptions_NoStaleGitPrefix ensures no description references git_-prefixed tool names.
func TestToolDescriptions_NoStaleGitPrefix(t *testing.T) {
	mcpSrv := registerAllToolsForTest()
	allDesc := allSchemaDescriptions(t, mcpSrv)
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
			assert.NotContains(t, desc, prefix, "description should not contain stale git_ prefix")
		}
	}
}

// TestToolDescriptions_NoFalsePromises ensures no description claims GitHub API support.
func TestToolDescriptions_NoFalsePromises(t *testing.T) {
	mcpSrv := registerAllToolsForTest()
	allDesc := allSchemaDescriptions(t, mcpSrv)
	for _, desc := range allDesc {
		assert.NotContains(t, desc, "GitHub API", "description should not claim GitHub API support")
		assert.NotContains(t, desc, "git_courer_create_pr", "description should not reference removed tool")
	}
}

// TestToolDescriptions_CommitDescribesPreviewApply ensures commit description
// mentions PREVIEW/APPLY workflow and job_id.
func TestToolDescriptions_CommitDescribesPreviewApply(t *testing.T) {
	mcpSrv := registerAllToolsForTest()
	desc := toolDescriptionFromSchema(t, mcpSrv, "commit")
	assert.Contains(t, desc, "PREVIEW", "commit description should mention PREVIEW workflow")
	assert.Contains(t, desc, "APPLY", "commit description should mention APPLY workflow")
	assert.True(t, strings.Contains(strings.ToLower(desc), "job_id") || strings.Contains(strings.ToLower(desc), "job id"),
		"commit description should mention job_id")
}

// TestToolDescriptions_CommitDescribesWhyAndTwoPaths ensures commit description
// mentions the 'why' parameter and the two execution paths of APPLY (plumbing vs legacy).
func TestToolDescriptions_CommitDescribesWhyAndTwoPaths(t *testing.T) {
	mcpSrv := registerAllToolsForTest()
	desc := toolDescriptionFromSchema(t, mcpSrv, "commit")
	assert.Contains(t, strings.ToLower(desc), "why", "commit description should mention the 'why' parameter")
	assert.Contains(t, strings.ToLower(desc), "plumbing", "commit description should mention the plumbing path")
}

// TestToolDescriptions_ConflictToolsDescribeStructuredError ensures merge/rebase
// document structured conflict output.
func TestToolDescriptions_ConflictToolsDescribeStructuredError(t *testing.T) {
	mcpSrv := registerAllToolsForTest()
	for _, name := range []string{"merge", "rebase"} {
		desc := toolDescriptionFromSchema(t, mcpSrv, name)
		assert.Contains(t, strings.ToLower(desc), "conflict", "%s description should mention conflicts", name)
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
		"sync", "remotes", "pr-review",
		"config", "backup", "undo", "commit-jobs",
	}
	for _, name := range expected {
		assert.True(t, strings.Contains(gitCourerSummary, name),
			"summary should reference tool %q", name)
	}
}

// TestParamDescriptions_Exist ensures key parameters have descriptions.
func TestParamDescriptions_Exist(t *testing.T) {
	mcpSrv := registerAllToolsForTest()
	tools := mcpSrv.ListTools()

	// Check specific parameters that MUST have descriptions per REQ-2
	type paramCheck struct {
		tool string
		param string
	}
	criticalParams := []paramCheck{
		{"amend", "confirmed"},
		{"amend", "dry_run"},
		{"revert", "confirmed"},
		{"revert", "dry_run"},
		{"stage", "confirmed"},
		{"reset", "dry_run"},
		{"sync", "confirmed"},
	}

	for _, check := range criticalParams {
		st, ok := tools[check.tool]
		require.True(t, ok, "tool %q should be registered", check.tool)

		props := st.Tool.InputSchema.Properties
		paramRaw, exists := props[check.param]
		require.True(t, exists, "tool %q should have param %q", check.tool, check.param)
		param, ok := paramRaw.(map[string]any)
		require.True(t, ok, "param %q on tool %q should be a map", check.param, check.tool)
		desc, _ := param["description"].(string)
		assert.NotEmpty(t, desc,
			"param %q on tool %q should have a description (REQ-2)", check.param, check.tool)
	}
}