package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/branching"
	"github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/shared"
	"github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/stage"
	mcpsync "github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/sync"
	"github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/utility"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
)

// --- Task 3.1: Table-driven tests for shared.CheckSafetyGate ---

func TestCheckSafetyGate(t *testing.T) {
	tests := []struct {
		name        string
		cmd         string
		dryRun      bool
		confirmed   bool
		wantBlocked bool
		wantAllow   bool
	}{
		{
			name:      "dry_run=true allows preview for destructive cmd",
			cmd:       "push",
			dryRun:    true,
			confirmed: false,
			wantAllow: true,
		},
		{
			name:        "dry_run=false + confirmed=false + destructive cmd → blocked",
			cmd:         "push",
			dryRun:      false,
			confirmed:   false,
			wantBlocked: true,
		},
		{
			name:      "dry_run=false + confirmed=true + destructive cmd → allowed",
			cmd:       "push",
			dryRun:    false,
			confirmed: true,
			wantAllow: true,
		},
		{
			name:      "non-destructive cmd always allowed",
			cmd:       "fetch",
			dryRun:    false,
			confirmed: false,
			wantAllow: true,
		},
		{
			name:        "clean without confirmed is blocked",
			cmd:         "clean",
			dryRun:      false,
			confirmed:   false,
			wantBlocked: true,
		},
		{
			name:      "clean with confirmed is allowed",
			cmd:       "clean",
			dryRun:    false,
			confirmed: true,
			wantAllow: true,
		},
		{
			name:        "reset_hard without confirmed is blocked",
			cmd:         "reset_hard",
			dryRun:      false,
			confirmed:   false,
			wantBlocked: true,
		},
		{
			name:        "branch_delete without confirmed is blocked",
			cmd:         "branch_delete",
			dryRun:      false,
			confirmed:   false,
			wantBlocked: true,
		},
		{
			name:        "remote_delete without confirmed is blocked",
			cmd:         "remote_delete",
			dryRun:      false,
			confirmed:   false,
			wantBlocked: true,
		},
		{
			name:        "amend without confirmed is blocked",
			cmd:         "amend",
			dryRun:      false,
			confirmed:   false,
			wantBlocked: true,
		},
		{
			name:      "amend with confirmed is allowed",
			cmd:       "amend",
			dryRun:    false,
			confirmed: true,
			wantAllow: true,
		},
		{
			name:      "amend with dry_run is allowed",
			cmd:       "amend",
			dryRun:    true,
			confirmed: false,
			wantAllow: true,
		},
		{
			name:        "revert without confirmed is blocked",
			cmd:         "revert",
			dryRun:      false,
			confirmed:   false,
			wantBlocked: true,
		},
		{
			name:      "revert with confirmed is allowed",
			cmd:       "revert",
			dryRun:    false,
			confirmed: true,
			wantAllow: true,
		},
		{
			name:      "revert with dry_run is allowed",
			cmd:       "revert",
			dryRun:    true,
			confirmed: false,
			wantAllow: true,
		},
		{
			name:        "delete_remote without confirmed is blocked",
			cmd:         "delete_remote",
			dryRun:      false,
			confirmed:   false,
			wantBlocked: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := shared.CheckSafetyGate(tt.cmd, tt.dryRun, tt.confirmed)
			assert.NoError(t, err, "shared.CheckSafetyGate should not return a Go error")

			if tt.wantBlocked {
				assert.NotNil(t, result, "result should not be nil when execution is blocked")
				if result != nil && len(result.Content) > 0 {
					text := result.Content[0].(mcpgo.TextContent).Text
					assert.Contains(t, text, "confirmed=true", "blocked result should instruct to set confirmed=true")
				}
			}
			if tt.wantAllow {
				assert.Nil(t, result, "result should be nil when execution is allowed")
			}
		})
	}
}

func TestCheckSafetyGate_BlockedResultStructure(t *testing.T) {
	result, err := shared.CheckSafetyGate("push", false, false)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	text := result.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &parsed), "blocked result should be valid JSON")
	assert.Equal(t, "blocked", parsed["status"])
	assert.Contains(t, parsed["reason"], "confirmed=true")
	assert.Equal(t, "push", parsed["operation"])
	assert.NotNil(t, parsed["undoable"], "blocked result should include undoable field")
}

// --- Task 3.5: Assert dry_run and confirmed exist in tool schemas ---

func TestToolRegistration_SafetyParams(t *testing.T) {
	tests := []struct {
		name          string
		toolName      string
		wantDryRun    bool
		wantConfirmed bool
	}{
		{name: "branch has confirmed", toolName: "branch", wantDryRun: false, wantConfirmed: true},
		{name: "tag has confirmed", toolName: "tag", wantDryRun: false, wantConfirmed: true},
		{name: "revert has dry_run and confirmed", toolName: "revert", wantDryRun: true, wantConfirmed: true},
		{name: "amend has dry_run and confirmed", toolName: "amend", wantDryRun: true, wantConfirmed: true},
		{name: "reset has confirmed", toolName: "reset", wantDryRun: false, wantConfirmed: true},
		{name: "sync has confirmed", toolName: "sync", wantDryRun: false, wantConfirmed: true},

		{name: "commit no safety params", toolName: "commit", wantDryRun: false, wantConfirmed: false},
		{name: "stage has dry_run and confirmed", toolName: "stage", wantDryRun: true, wantConfirmed: true},
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

			props := found.InputSchema.Properties
			if props == nil {
				t.Fatalf("tool %q has no input schema properties", tt.toolName)
			}

			if tt.wantDryRun {
				prop, ok := props["dry_run"]
				if !ok {
					t.Errorf("tool %q missing 'dry_run' property in schema", tt.toolName)
				} else {
					propMap, ok := prop.(map[string]any)
					if !ok {
						t.Errorf("tool %q 'dry_run' property schema is not a map", tt.toolName)
					} else {
						propType, _ := propMap["type"].(string)
						assert.Equal(t, "boolean", propType, "dry_run should be type boolean")
					}
				}
			}

			if tt.wantConfirmed {
				prop, ok := props["confirmed"]
				if !ok {
					t.Errorf("tool %q missing 'confirmed' property in schema", tt.toolName)
				} else {
					propMap, ok := prop.(map[string]any)
					if !ok {
						t.Errorf("tool %q 'confirmed' property schema is not a map", tt.toolName)
					} else {
						propType, _ := propMap["type"].(string)
						assert.Equal(t, "boolean", propType, "confirmed should be type boolean")
					}
				}
			}
		})
	}
}

// --- Task 3.4: RESTORE alias and LIST undoable ---

// --- Task 1.3: Hidden params for stage, reset, stash tools ---

func TestToolRegistration_HiddenParams_StageResetStash(t *testing.T) {
	tests := []struct {
		name      string
		toolName  string
		param     string
		paramType string // "boolean" or "string"
	}{
		{name: "reset has dry_run", toolName: "reset", param: "dry_run", paramType: "boolean"},
		{name: "stash has commit_message", toolName: "stash", param: "commit_message", paramType: "string"},
		{name: "stash has stash_index", toolName: "stash", param: "stash_index", paramType: "string"},
		{name: "stash has include_untracked", toolName: "stash", param: "include_untracked", paramType: "boolean"},
	}

	mcpSrv := server.NewMCPServer("test", "1.0")
	srv := &Server{}
	registerTools(mcpSrv, srv)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := findTool(mcpSrv, tt.toolName)
			if found == nil {
				t.Fatalf("tool %q not found after registration", tt.toolName)
			}

			props := found.InputSchema.Properties
			if props == nil {
				t.Fatalf("tool %q has no input schema properties", tt.toolName)
			}

			prop, ok := props[tt.param]
			if !ok {
				t.Fatalf("tool %q missing '%s' property in schema", tt.toolName, tt.param)
			}
			propMap, ok := prop.(map[string]any)
			if !ok {
				t.Fatalf("tool %q '%s' property schema is not a map", tt.toolName, tt.param)
			}
			propType, _ := propMap["type"].(string)
			assert.Equal(t, tt.paramType, propType, "%s should be type %s", tt.param, tt.paramType)
		})
	}
}

func TestHandleBackup_RestoreAlias(t *testing.T) {
	t.Run("RESTORE dispatches same logic as undo", func(t *testing.T) {
		mockGit := new(MockGit)
		backup := domain.Backup{Ref: "refs/git-courer/backup/20260517123000_commit", Operation: "commit"}
		mockGit.On("ListBackups").Return([]domain.Backup{backup}, nil)
		mockGit.On("RestoreBackup", backup).Return(nil)

		h := utility.NewHandler(mockGit, nil, "", nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "backup",
				Arguments: map[string]any{"command": "RESTORE"},
			},
		}

		res, err := h.HandleBackup(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		assert.Contains(t, text, "Successfully restored", "RESTORE should produce restore result")
		mockGit.AssertExpectations(t)
	})
}

func TestHandleBackup_ListUndoable(t *testing.T) {
	t.Run("LIST shows undoable field per backup", func(t *testing.T) {
		mockGit := new(MockGit)

		mockGit.On("ListBackups").Return([]domain.Backup{
			{Ref: "ref1", Operation: "commit", CreatedAt: testingTime(), Undoable: true},
			{Ref: "ref2", Operation: "push", CreatedAt: testingTime(), Undoable: false},
		}, nil)

		h := utility.NewHandler(mockGit, nil, "", nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "backup",
				Arguments: map[string]any{"command": "LIST"},
			},
		}

		res, err := h.HandleBackup(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		var parsed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(text), &parsed))

		backups, ok := parsed["backups"].([]any)
		assert.True(t, ok, "result should have 'backups' array")
		assert.Len(t, backups, 2, "should return 2 backups")

		b1, _ := backups[0].(map[string]any)
		assert.Equal(t, true, b1["undoable"], "commit backup should be undoable")

		b2, _ := backups[1].(map[string]any)
		assert.Equal(t, false, b2["undoable"], "push backup should not be undoable")

		mockGit.AssertExpectations(t)
	})
}

// --- Task 3.2: PUSH dry-run and MERGE conflict scenarios ---

func TestHandleSync_PushDryRun(t *testing.T) {
	mockGit := new(MockGit)
	handler := mcpsync.NewHandler(mockGit)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "sync",
			Arguments: map[string]any{"command": "PUSH", "remote_name": "origin", "dry_run": true},
		},
	}

	res, err := handler.HandleSync(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Equal(t, "push", parsed["operation"])
	assert.NotNil(t, parsed["undoable"], "dry_run preview should include undoable field")

	mockGit.AssertNotCalled(t, "PushTo")
}

func TestHandleSync_PushBlockedWithoutConfirmed(t *testing.T) {
	mockGit := new(MockGit)
	handler := mcpsync.NewHandler(mockGit)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "sync",
			Arguments: map[string]any{"command": "PUSH", "remote_name": "origin"},
		},
	}

	res, err := handler.HandleSync(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "blocked", "PUSH without confirmed should be blocked")
	assert.Contains(t, text, "confirmed=true", "blocked result should suggest confirmed=true")

	mockGit.AssertNotCalled(t, "PushTo")
}

func TestHandleSync_PushWithConfirmed(t *testing.T) {
	mockGit := new(MockGit)
	handler := mcpsync.NewHandler(mockGit)

	mockGit.On("CreateBackup", "PUSH", domain.StashNone).Return(domain.Backup{}, nil)
	mockGit.On("PushTo", "origin").Return("pushed", nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "sync",
			Arguments: map[string]any{"command": "PUSH", "remote_name": "origin", "confirmed": true},
		},
	}

	res, err := handler.HandleSync(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Contains(t, parsed["message"], "Pushed to origin")
	mockGit.AssertExpectations(t)
}

func TestHandleSync_MergeConflict(t *testing.T) {
	mockGit := new(MockGit)
	handler := branching.NewHandler(mockGit)

	backup := domain.Backup{Ref: "backup-ref", Operation: "MERGE"}
	mockGit.On("CreateBackup", "MERGE", domain.StashNone).Return(backup, nil)
	mockGit.On("Merge", "feature").Return("", fmt.Errorf("CONFLICT (content): Merge conflict in main.go"))
	mockGit.On("DeleteBackup", backup).Return(nil)
	mockGit.On("Status").Return(domain.Status{
		Branch: "main",
		Files: []domain.FileStatus{
			{Path: "main.go", Status: "UU"},
			{Path: "README.md", Status: "AA"},
		},
		Conflicted: 2,
	}, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "merge",
			Arguments: map[string]any{"merge_branch_name": "feature"},
		},
	}

	res, err := handler.HandleMerge(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Equal(t, "conflict", parsed["status"], "result status should be 'conflict'")
	assert.NotNil(t, parsed["conflicted_files"], "result should include conflicted_files")
	assert.NotNil(t, parsed["message"], "result should include message")
	mockGit.AssertExpectations(t)
}

// --- Task 3.3: CLEAN without confirmed (blocked) and with confirmed (allowed) ---

func TestHandleStage_CleanBlockedWithoutConfirmed(t *testing.T) {
	mockGit := new(MockGit)

	h := stage.NewHandler(mockGit, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "stage",
			Arguments: map[string]any{"command": "CLEAN"},
		},
	}

	res, err := h.HandleStage(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "blocked", "CLEAN without confirmed should be blocked")

	mockGit.AssertNotCalled(t, "Clean")
}

func TestHandleStage_CleanWithConfirmed(t *testing.T) {
	mockGit := new(MockGit)
	mockGit.On("CreateBackup", "CLEAN", domain.StashNone).Return(domain.Backup{}, nil)
	mockGit.On("Clean").Return(nil)

	h := stage.NewHandler(mockGit, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "stage",
			Arguments: map[string]any{"command": "CLEAN", "confirmed": true},
		},
	}

	res, err := h.HandleStage(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Contains(t, parsed["message"], "cleaned")
	mockGit.AssertExpectations(t)
}

func TestHandleStage_CleanDryRun(t *testing.T) {
	mockGit := new(MockGit)
	h := stage.NewHandler(mockGit, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "stage",
			Arguments: map[string]any{"command": "CLEAN", "dry_run": true},
		},
	}

	res, err := h.HandleStage(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Equal(t, "clean", parsed["operation"])

	mockGit.AssertNotCalled(t, "Clean")
	mockGit.AssertNotCalled(t, "CreateBackup")
}

func TestHandleStage_ResetHardBlockedWithoutConfirmed(t *testing.T) {
	mockGit := new(MockGit)
	h := stage.NewHandler(mockGit, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "reset",
			Arguments: map[string]any{"command": "HARD", "target_commit": "abc123"},
		},
	}

	res, err := h.HandleReset(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "blocked")

	mockGit.AssertNotCalled(t, "Reset")
}

func testingTime() time.Time {
	t, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
	return t
}
