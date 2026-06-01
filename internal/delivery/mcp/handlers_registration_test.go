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

// findTool looks up a registered tool by name using ListTools.
func findTool(mcpSrv *server.MCPServer, name string) *mcpgo.Tool {
	tools := mcpSrv.ListTools()
	st, ok := tools[name]
	if !ok || st == nil {
		return nil
	}
	return &st.Tool
}

// TestToolRegistration_EnumConstraints verifies tools with command enums have correct values.
func TestToolRegistration_EnumConstraints(t *testing.T) {
	tests := []struct {
		name           string
		toolName       string
		wantEnumValues []string
	}{
		{
			name:           "branch",
			toolName:       "branch",
			wantEnumValues: []string{"CREATE", "DELETE", "RENAME", "REMOTE_DELETE", "SET_UPSTREAM", "UNSET_UPSTREAM", "SWITCH", "LIST"},
		},
		{
			name:           "tag",
			toolName:       "tag",
			wantEnumValues: []string{"CREATE", "DELETE", "PUSH", "DELETE_REMOTE"},
		},
		{
			name:           "stash",
			toolName:       "stash",
			wantEnumValues: []string{"SAVE", "POP", "SHOW"},
		},
		{
			name:           "backup",
			toolName:       "backup",
			wantEnumValues: []string{"CREATE", "DELETE", "RESTORE", "LIST"},
		},
		{
			name:           "sync",
			toolName:       "sync",
			wantEnumValues: []string{"PUSH", "PULL", "FETCH"},
		},
		{
			name:           "stage",
			toolName:       "stage",
			wantEnumValues: []string{"ADD", "RM", "RESTORE", "CLEAN"},
		},
		{
			name:           "history",
			toolName:       "history",
			wantEnumValues: []string{"LOG", "REFLOG"},
		},
		{
			name:           "commit",
			toolName:       "commit",
			wantEnumValues: []string{"PREVIEW", "APPLY", "ABORT", "REGENERATE", "STATUS"},
		},
		{
			name:           "config",
			toolName:       "config",
			wantEnumValues: []string{"GET", "SET_TEST_COMMAND", "SET_USER_NAME", "SET_USER_EMAIL", "SET_SIGNING_KEY"},
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
		})
	}
}

// TestToolRegistration_NoCommandOnSimpleTools verifies tools that don't use command enums.
func TestToolRegistration_NoCommandOnSimpleTools(t *testing.T) {
	// These tools return full objects without command enums
	// Note: config now uses SET_TEST_COMMAND as a write command, but calling config
	// without a command still returns full data (backward compatible)
	toolsWithNoCommand := []string{"status", "diff", "amend", "revert", "blame"}

	for _, toolName := range toolsWithNoCommand {
		t.Run(toolName, func(t *testing.T) {
			mcpSrv := server.NewMCPServer("test", "1.0")
			srv := &Server{}
			registerTools(mcpSrv, srv)

			found := findTool(mcpSrv, toolName)
			if found == nil {
				t.Fatalf("tool %q not found after registration", toolName)
			}

			_, ok := getCommandEnum(found)
			if ok {
				t.Errorf("tool %q should NOT have a 'command' enum (it returns full data without discriminator)", toolName)
			}
		})
	}
}

// TestToolRegistration_Commit_DynamicCommands verifies commit tool enum covers all phases.
func TestToolRegistration_Commit_DynamicCommands(t *testing.T) {
	mcpSrv := server.NewMCPServer("test", "1.0")
	srv := &Server{}
	registerTools(mcpSrv, srv)

	found := findTool(mcpSrv, "commit")
	if found == nil {
		t.Fatal("commit tool not found after registration")
	}

	enumVals, ok := getCommandEnum(found)
	if !ok {
		t.Fatal("commit missing 'command' enum")
	}

	wantPhases := []string{"PREVIEW", "APPLY", "ABORT", "REGENERATE"}
	for _, want := range wantPhases {
		foundPhase := false
		for _, v := range enumVals {
			if v == want {
				foundPhase = true
				break
			}
		}
		if !foundPhase {
			t.Errorf("commit enum missing phase command %q", want)
		}
	}
}

// TestToolRegistration_ShowAndSearchRemoved verifies show and search tools are no longer registered.
func TestToolRegistration_ShowAndSearchRemoved(t *testing.T) {
	mcpSrv := server.NewMCPServer("test", "1.0")
	srv := &Server{}
	registerTools(mcpSrv, srv)

	for _, name := range []string{"show", "search"} {
		found := findTool(mcpSrv, name)
		if found != nil {
			t.Errorf("tool %q should NOT be registered (it was removed)", name)
		}
	}
}

// TestToolRegistration_Release_Removed verifies the release tool is NOT registered
// (removed in Phase 3 — CLI release command is the replacement).
func TestToolRegistration_Release_Removed(t *testing.T) {
	mcpSrv := server.NewMCPServer("test", "1.0")
	srv := &Server{}
	registerTools(mcpSrv, srv)

	found := findTool(mcpSrv, "release")
	if found != nil {
		t.Error("tool \"release\" should NOT be registered (removed in Phase 3; use CLI release command instead)")
	}
}
