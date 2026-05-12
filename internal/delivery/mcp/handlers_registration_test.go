package mcp

import (
	"reflect"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// getCommandEnum extracts the enum string slice from a tool's "command" property schema.
func getCommandEnum(t *mcpgo.Tool) ([]string, bool) {
	if t.InputSchema.Properties == nil {
		return nil, false
	}
	raw, ok := t.InputSchema.Properties["command"]
	if !ok {
		return nil, false
	}
	propMap, ok := raw.(map[string]any)
	if !ok {
		return nil, false
	}
	enumRaw, ok := propMap["enum"]
	if !ok {
		return nil, false
	}
	enumSlice, ok := enumRaw.([]string)
	if !ok {
		return nil, false
	}
	return enumSlice, true
}

// getCommandRequired checks whether "command" is in the tool's required list.
func getCommandRequired(t *mcpgo.Tool) bool {
	for _, r := range t.InputSchema.Required {
		if r == "command" {
			return true
		}
	}
	return false
}

// findTool looks up a registered tool by name using ListTools.
func findTool(mcpSrv *server.MCPServer, name string) *mcpgo.Tool {
	tools := mcpSrv.ListTools()
	st, ok := tools[name]
	if !ok || st == nil {
		return nil
	}
	return &st.Tool
}

// TestToolRegistration_EnumConstraints verifies that the 11 multi-command tools
// have the "command" parameter declared as an enum with the expected values.
func TestToolRegistration_EnumConstraints(t *testing.T) {
	tests := []struct {
		name           string
		toolName       string
		wantEnumValues []string
	}{
		{
			name:           "git_branch",
			toolName:       "git_branch",
			wantEnumValues: []string{"CREATE", "DELETE", "RENAME", "REMOTE_DELETE", "SET_UPSTREAM", "UNSET_UPSTREAM"},
		},
		{
			name:           "git_tag",
			toolName:       "git_tag",
			wantEnumValues: []string{"CREATE", "DELETE", "PUSH", "DELETE_REMOTE"},
		},
		{
			name:           "git_stash",
			toolName:       "git_stash",
			wantEnumValues: []string{"SAVE", "POP", "APPLY", "DROP", "CLEAR", "SHOW"},
		},
		{
			name:           "git_backup",
			toolName:       "git_backup",
			wantEnumValues: []string{"RESTORE", "UNDO", "LIST", "PRUNE"},
		},
		{
			name:           "git_config",
			toolName:       "git_config",
			wantEnumValues: []string{"READ", "LIST_MODELS"},
		},
		{
			name:           "git_sync",
			toolName:       "git_sync",
			wantEnumValues: []string{"FETCH", "PULL", "PUSH", "MERGE", "MERGE_ABORT", "SWITCH", "REBASE", "REBASE_ABORT", "REBASE_CONTINUE", "CHERRY_PICK", "ADD_REMOTE", "REMOVE_REMOTE"},
		},
		{
			name:           "git_stage",
			toolName:       "git_stage",
			wantEnumValues: []string{"ADD", "RM", "RESTORE", "RESET_SOFT", "RESET_MIXED", "RESET_HARD", "CLEAN"},
		},
		{
			name:           "git_log",
			toolName:       "git_log",
			wantEnumValues: []string{"READ_LOG", "READ_BRANCHES", "READ_TAGS", "REMOTE_BRANCH_LIST", "REMOTE_TAG_LIST", "BLAME", "SHOW", "REFLOG", "MERGE_BASE", "READ_SEARCH", "CAT_FILE", "LIST_TREE"},
		},
		{
			name:           "git_status",
			toolName:       "git_status",
			wantEnumValues: []string{"READ_STATUS", "CURRENT_BRANCH", "IS_REPO", "REMOTE_INFO", "WHAT_CHANGED"},
		},
		{
			name:           "git_review",
			toolName:       "git_review",
			wantEnumValues: []string{"STATUS", "SUMMARY", "JOB_RESULT", "REVERT", "AMEND", "COMMIT_START", "COMMIT_APPLY", "COMMIT_ABORT", "COMMIT_REGENERATE", "RELEASE_START", "RELEASE_APPLY", "RELEASE_ABORT"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mcpSrv := server.NewMCPServer("test", "1.0")
			srv := &Server{}
			registerTools(mcpSrv, srv)

			found := findTool(mcpSrv, tt.toolName)
			if found == nil {
				t.Fatalf("tool %q not found after registration", tt.toolName)
			}

			enumVals, ok := getCommandEnum(found)
			if !ok {
				t.Fatalf("tool %q missing 'command' enum in schema", tt.toolName)
			}
			if len(enumVals) == 0 {
				t.Fatalf("tool %q 'command' property has no enum constraint", tt.toolName)
			}

			if !reflect.DeepEqual(enumVals, tt.wantEnumValues) {
				t.Errorf("tool %q enum mismatch\ngot:  %v\nwant: %v", tt.toolName, enumVals, tt.wantEnumValues)
			}

			if !getCommandRequired(found) {
				t.Errorf("tool %q 'command' should be in required list", tt.toolName)
			}
		})
	}
}

// TestToolRegistration_GitDiff_AliasIncluded verifies that git_diff enum includes both alias values.
func TestToolRegistration_GitDiff_AliasIncluded(t *testing.T) {
	mcpSrv := server.NewMCPServer("test", "1.0")
	srv := &Server{}
	registerTools(mcpSrv, srv)

	found := findTool(mcpSrv, "git_diff")
	if found == nil {
		t.Fatal("git_diff tool not found after registration")
	}

	propMap, ok := found.InputSchema.Properties["command"].(map[string]any)
	if !ok {
		t.Fatal("git_diff 'command' property schema missing")
	}
	enumRaw, ok := propMap["enum"].([]string)
	if !ok {
		t.Fatal("git_diff 'command' enum not a []string")
	}

	hasStat := false
	hasStats := false
	for _, v := range enumRaw {
		if v == "READ_DIFF_STAT" {
			hasStat = true
		}
		if v == "READ_DIFF_STATS" {
			hasStats = true
		}
	}

	if !hasStat {
		t.Error("git_diff enum missing READ_DIFF_STAT alias")
	}
	if !hasStats {
		t.Error("git_diff enum missing READ_DIFF_STATS")
	}
}

// TestToolRegistration_GitReview_DynamicCommands verifies git_review enum covers all OP_PHASE combos.
func TestToolRegistration_GitReview_DynamicCommands(t *testing.T) {
	mcpSrv := server.NewMCPServer("test", "1.0")
	srv := &Server{}
	registerTools(mcpSrv, srv)

	found := findTool(mcpSrv, "git_review")
	if found == nil {
		t.Fatal("git_review tool not found after registration")
	}

	enumVals, ok := getCommandEnum(found)
	if !ok {
		t.Fatal("git_review missing 'command' enum")
	}

	wantDynamic := []string{
		"COMMIT_START", "COMMIT_APPLY", "COMMIT_ABORT", "COMMIT_REGENERATE",
		"RELEASE_START", "RELEASE_APPLY", "RELEASE_ABORT",
	}
	for _, want := range wantDynamic {
		found := false
		for _, v := range enumVals {
			if v == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("git_review enum missing dynamic command %q", want)
		}
	}

	// Also verify static commands
	wantStatic := []string{"STATUS", "SUMMARY", "JOB_RESULT", "REVERT", "AMEND"}
	for _, want := range wantStatic {
		found := false
		for _, v := range enumVals {
			if v == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("git_review enum missing static command %q", want)
		}
	}
}

// TestToolRegistration_NoEnumOnSinglePurposeTools verifies git_revert and git_amend
// have no command parameter and no enum constraint.
func TestToolRegistration_NoEnumOnSinglePurposeTools(t *testing.T) {
	mcpSrv := server.NewMCPServer("test", "1.0")
	srv := &Server{}
	registerTools(mcpSrv, srv)

	toolsToCheck := []string{"git_revert", "git_amend"}
	for _, toolName := range toolsToCheck {
		t.Run(toolName, func(t *testing.T) {
			found := findTool(mcpSrv, toolName)
			if found == nil {
				t.Fatalf("tool %q not found after registration", toolName)
			}

			_, ok := getCommandEnum(found)
			if ok {
				t.Errorf("tool %q should NOT have a 'command' enum", toolName)
			}
		})
	}
}

// TestToolRegistration_EmptyCommandDefaults verifies that git_diff, git_log, git_stage, git_status
// have enum constraints while still allowing empty/default behavior in the handler.
func TestToolRegistration_EmptyCommandDefaults(t *testing.T) {
	toolsWithDefaults := []string{"git_diff", "git_log", "git_stage", "git_status"}

	for _, toolName := range toolsWithDefaults {
		t.Run(toolName, func(t *testing.T) {
			mcpSrv := server.NewMCPServer("test", "1.0")
			srv := &Server{}
			registerTools(mcpSrv, srv)

			found := findTool(mcpSrv, toolName)
			if found == nil {
				t.Fatalf("tool %q not found", toolName)
			}

			enumVals, ok := getCommandEnum(found)
			if !ok {
				t.Fatalf("tool %q missing 'command' enum", toolName)
			}
			if len(enumVals) == 0 {
				t.Errorf("tool %q should have enum on 'command'", toolName)
			}

			// Verify the tool's default command is represented in the enum.
			var expectedDefault string
			switch toolName {
			case "git_diff":
				expectedDefault = "READ_DIFF"
			case "git_log":
				expectedDefault = "READ_LOG"
			case "git_stage":
				expectedDefault = "ADD"
			case "git_status":
				expectedDefault = "READ_STATUS"
			}

			foundDefault := false
			for _, v := range enumVals {
				if v == expectedDefault {
					foundDefault = true
					break
				}
			}
			if !foundDefault {
				t.Errorf("tool %q enum missing default command %q", toolName, expectedDefault)
			}
		})
	}
}
