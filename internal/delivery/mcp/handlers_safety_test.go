package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
)

// --- Task 3.1: Table-driven tests for checkSafetyGate ---

func TestCheckSafetyGate(t *testing.T) {
	tests := []struct {
		name        string
		cmd         string
		dryRun      bool
		confirmed   bool
		wantBlocked bool // true = result is not nil (execution blocked)
		wantAllow   bool // true = result is nil (execution allowed)
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
			name:      "clean without confirmed is blocked",
			cmd:       "clean",
			dryRun:    false,
			confirmed: false,
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
			name:      "reset_hard without confirmed is blocked",
			cmd:       "reset_hard",
			dryRun:    false,
			confirmed: false,
			wantBlocked: true,
		},
		{
			name:      "branch_delete without confirmed is blocked",
			cmd:       "branch_delete",
			dryRun:    false,
			confirmed: false,
			wantBlocked: true,
		},
		{
			name:      "remote_delete without confirmed is blocked",
			cmd:       "remote_delete",
			dryRun:    false,
			confirmed: false,
			wantBlocked: true,
		},
		{
			name:      "delete_remote without confirmed is blocked",
			cmd:       "delete_remote",
			dryRun:    false,
			confirmed: false,
			wantBlocked: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := checkSafetyGate(tt.cmd, tt.dryRun, tt.confirmed)
			assert.NoError(t, err, "checkSafetyGate should not return a Go error")

			if tt.wantBlocked {
				assert.NotNil(t, result, "result should not be nil when execution is blocked")
				// Verify the blocked result contains "confirmed=true" instruction
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

// Triangulation: verify that the blocked result JSON has the expected structure
func TestCheckSafetyGate_BlockedResultStructure(t *testing.T) {
	result, err := checkSafetyGate("push", false, false)
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
		name       string
		toolName   string
		wantDryRun bool
		wantConfirmed bool
	}{
		// Tools that need BOTH dry_run AND confirmed
		{name: "git_sync has dry_run+confirmed", toolName: "git_sync", wantDryRun: true, wantConfirmed: true},
		{name: "git_stage has dry_run+confirmed", toolName: "git_stage", wantDryRun: true, wantConfirmed: true},

		// Tools that need dry_run only (preview, no confirmation gate)
		{name: "git_review has dry_run", toolName: "git_review", wantDryRun: true, wantConfirmed: false},

		// Tools that need confirmed only
		{name: "git_branch has confirmed", toolName: "git_branch", wantDryRun: false, wantConfirmed: true},
		{name: "git_tag has confirmed", toolName: "git_tag", wantDryRun: false, wantConfirmed: true},

		// Standalone tools
		{name: "git_revert has dry_run+confirmed", toolName: "git_revert", wantDryRun: true, wantConfirmed: true},
		{name: "git_amend has dry_run+confirmed", toolName: "git_amend", wantDryRun: true, wantConfirmed: true},
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

func TestHandleGitBackup_RestoreAlias(t *testing.T) {
	t.Run("RESTORE dispatches same logic as UNDO", func(t *testing.T) {
		mockGit := new(MockGit)
		srv := &Server{
			git:        mockGit,
			lastBackup: domain.Backup{Ref: "backup-ref", Operation: "commit"},
		}
		mockGit.On("RestoreBackup", srv.lastBackup).Return(nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "git_backup",
				Arguments: map[string]any{"command": "RESTORE"},
			},
		}

		res, err := srv.handleGitBackup(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		assert.Contains(t, text, "Successfully reverted", "RESTORE should produce same result as UNDO")
		mockGit.AssertExpectations(t)
	})
}

func TestHandleGitBackup_ListUndoable(t *testing.T) {
	t.Run("LIST shows undoable field per backup", func(t *testing.T) {
		mockGit := new(MockGit)
		srv := &Server{
			git: mockGit,
		}

		// Create backups: commit is undoable, push is not
		mockGit.On("ListBackups").Return([]domain.Backup{
			{Ref: "ref1", Operation: "commit", CreatedAt: testingTime(), Undoable: true},
			{Ref: "ref2", Operation: "push", CreatedAt: testingTime(), Undoable: false},
		}, nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "git_backup",
				Arguments: map[string]any{"command": "LIST"},
			},
		}

		res, err := srv.handleGitBackup(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		var parsed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(text), &parsed))

		backups, ok := parsed["backups"].([]any)
		assert.True(t, ok, "result should have 'backups' array")
		assert.Len(t, backups, 2, "should return 2 backups")

		// First backup: commit → undoable: true
		b1, _ := backups[0].(map[string]any)
		assert.Equal(t, true, b1["undoable"], "commit backup should be undoable")

		// Second backup: push → undoable: false
		b2, _ := backups[1].(map[string]any)
		assert.Equal(t, false, b2["undoable"], "push backup should not be undoable")

		mockGit.AssertExpectations(t)
	})
}

// --- Task 3.2: PUSH dry-run and MERGE conflict scenarios ---

func TestHandleGitSync_PushDryRun(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}

	// dry_run=true → should NOT call PushTo, should return preview
	// No backup expected for dry_run
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_sync",
			Arguments: map[string]any{"command": "PUSH", "remote_name": "origin", "dry_run": true},
		},
	}

	res, err := srv.handleGitSync(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	// Should be a preview result, not an error
	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Equal(t, "push", parsed["operation"])
	assert.NotNil(t, parsed["undoable"], "dry_run preview should include undoable field")

	// Mock should NOT have been called for PushTo
	mockGit.AssertNotCalled(t, "PushTo")
}

func TestHandleGitSync_PushBlockedWithoutConfirmed(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}

	// dry_run=false + confirmed=false → should be blocked
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_sync",
			Arguments: map[string]any{"command": "PUSH", "remote_name": "origin"},
		},
	}

	res, err := srv.handleGitSync(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "blocked", "PUSH without confirmed should be blocked")
	assert.Contains(t, text, "confirmed=true", "blocked result should suggest confirmed=true")

	mockGit.AssertNotCalled(t, "PushTo")
}

func TestHandleGitSync_PushWithConfirmed(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}

	mockGit.On("CreateBackup", "PUSH", domain.StashNone).Return(domain.Backup{}, nil)
	mockGit.On("PushTo", "origin").Return("pushed", nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_sync",
			Arguments: map[string]any{"command": "PUSH", "remote_name": "origin", "confirmed": true},
		},
	}

	res, err := srv.handleGitSync(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Contains(t, parsed["message"], "Pushed to origin")
	mockGit.AssertExpectations(t)
}

func TestHandleGitSync_MergeConflict(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}

	// MERGE with confirmed=true, but git returns a conflict error
	mockGit.On("CreateBackup", "MERGE", domain.StashNone).Return(domain.Backup{}, nil)
	mockGit.On("Merge", "feature").Return("", fmt.Errorf("CONFLICT (content): Merge conflict in main.go"))
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
			Name:      "git_sync",
			Arguments: map[string]any{"command": "MERGE", "branch_name": "feature", "confirmed": true},
		},
	}

	res, err := srv.handleGitSync(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Equal(t, true, parsed["conflict"], "result should indicate conflict")
	assert.NotNil(t, parsed["files"], "result should include conflicted files")
	mockGit.AssertExpectations(t)
}

// --- Task 3.3: CLEAN without confirmed (blocked) and with confirmed (allowed) ---

func TestHandleGitStage_CleanBlockedWithoutConfirmed(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_stage",
			Arguments: map[string]any{"command": "CLEAN"},
		},
	}

	res, err := srv.handleGitStage(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "blocked", "CLEAN without confirmed should be blocked")

	mockGit.AssertNotCalled(t, "Clean")
}

func TestHandleGitStage_CleanWithConfirmed(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}

	mockGit.On("CreateBackup", "CLEAN", domain.StashNone).Return(domain.Backup{}, nil)
	mockGit.On("Clean").Return(nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_stage",
			Arguments: map[string]any{"command": "CLEAN", "confirmed": true},
		},
	}

	res, err := srv.handleGitStage(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Contains(t, parsed["message"], "cleaned")
	mockGit.AssertExpectations(t)
}

func TestHandleGitStage_CleanDryRun(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}

	// dry_run should not call Clean and should not create backup
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_stage",
			Arguments: map[string]any{"command": "CLEAN", "dry_run": true},
		},
	}

	res, err := srv.handleGitStage(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	var parsed map[string]any
	assert.NoError(t, json.Unmarshal([]byte(text), &parsed))
	assert.Equal(t, "clean", parsed["operation"])

	mockGit.AssertNotCalled(t, "Clean")
	mockGit.AssertNotCalled(t, "CreateBackup")
}

func TestHandleGitStage_ResetHardBlockedWithoutConfirmed(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_stage",
			Arguments: map[string]any{"command": "RESET_HARD", "target_commit": "abc123"},
		},
	}

	res, err := srv.handleGitStage(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "blocked")

	mockGit.AssertNotCalled(t, "Reset")
}

// helper for consistent timestamps in tests
func testingTime() time.Time {
	t, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
	return t
}