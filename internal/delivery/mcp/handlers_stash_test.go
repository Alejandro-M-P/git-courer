package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleGitStash(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{
		git: mockGit,
	}

	tests := []struct {
		name       string
		command    string
		args       map[string]any
		setup      func()
		wantInJSON string
		wantErr    bool
		errContain string
	}{
		{
			name:    "SAVE without commit_message",
			command: "SAVE",
			args:    map[string]any{},
			setup: func() {
				mockGit.On("CreateBackup", "SAVE", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("Stash", []string(nil)).Return("stashed", nil)
			},
			wantInJSON: "Changes stashed",
		},
		{
			name:    "SAVE with commit_message",
			command: "SAVE",
			args:    map[string]any{"commit_message": "my stash msg"},
			setup: func() {
				mockGit.On("CreateBackup", "SAVE", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("Stash", []string{"my stash msg"}).Return("stashed", nil)
			},
			wantInJSON: "Changes stashed",
		},
		{
			name:    "POP",
			command: "POP",
			args:    map[string]any{},
			setup: func() {
				mockGit.On("CreateBackup", "POP", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("StashPop").Return("restored", nil)
			},
			wantInJSON: "Stash restored",
		},
		{
			name:    "APPLY with stash_index",
			command: "APPLY",
			args:    map[string]any{"stash_index": "0"},
			setup: func() {
				mockGit.On("CreateBackup", "APPLY", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("StashApply", "0").Return("applied", nil)
			},
			wantInJSON: "Stash applied",
		},
		{
			name:    "APPLY without stash_index",
			command: "APPLY",
			args:    map[string]any{},
			setup: func() {
				mockGit.On("CreateBackup", "APPLY", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("StashApply", "").Return("applied", nil)
			},
			wantInJSON: "Stash applied",
		},
		{
			name:    "DROP with stash_index",
			command: "DROP",
			args:    map[string]any{"stash_index": "2"},
			setup: func() {
				mockGit.On("CreateBackup", "DROP", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("StashDrop", "2").Return("dropped", nil)
			},
			wantInJSON: "Stash entry dropped",
		},
		{
			name:       "DROP missing stash_index",
			command:    "DROP",
			args:       map[string]any{},
			setup:      func() {},
			wantErr:    true,
			errContain: "stash_index is required for DROP",
		},
		{
			name:    "CLEAR",
			command: "CLEAR",
			args:    map[string]any{},
			setup: func() {
				mockGit.On("CreateBackup", "CLEAR", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("StashClear").Return("cleared", nil)
			},
			wantInJSON: "All stash entries cleared",
		},
		{
			name:    "SHOW",
			command: "SHOW",
			args:    map[string]any{},
			setup: func() {
				mockGit.On("StashShow").Return("stash diff output", nil)
			},
			wantInJSON: "stash diff output",
		},
		{
			name:       "Unknown command returns error with suggestion",
			command:    "SAV",
			args:       map[string]any{},
			setup:      func() {},
			wantErr:    true,
			errContain: "unknown command",
		},
		{
			name:       "Unknown param returns error",
			command:    "SAVE",
			args:       map[string]any{"arg": "test"},
			setup:      func() {},
			wantErr:    true,
			errContain: "unknown parameter: arg",
		},
		{
			name:       "Empty command returns error",
			command:    "",
			args:       map[string]any{},
			setup:      func() {},
			wantErr:    true,
			errContain: "command is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			args := map[string]any{}
			if tt.command != "" {
				args["command"] = tt.command
			}
			for k, v := range tt.args {
				args[k] = v
			}
			req := mcpgo.CallToolRequest{
				Params: mcpgo.CallToolParams{
					Name:      "git_stash",
					Arguments: args,
				},
			}

			res, err := srv.handleGitStash(context.Background(), req)

			if tt.wantErr {
				assert.NoError(t, err, "jsonErrorResult should not return a Go error")
				assert.NotNil(t, res)
				if res != nil && len(res.Content) > 0 {
					text := res.Content[0].(mcpgo.TextContent).Text
					assert.Contains(t, text, tt.errContain)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, res)
				if res != nil && len(res.Content) > 0 {
					text := res.Content[0].(mcpgo.TextContent).Text
					assert.Contains(t, text, tt.wantInJSON)
				}
			}
			mockGit.AssertExpectations(t)
		})
	}
}

func TestHandleGitStash_ValidJSON(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{
		git: mockGit,
	}

	t.Run("SAVE produces valid JSON", func(t *testing.T) {
		mockGit.On("CreateBackup", "SAVE", domain.StashNone).Return(domain.Backup{}, nil)
		mockGit.On("Stash", []string{"work in progress"}).Return("saved", nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "git_stash",
				Arguments: map[string]any{"command": "SAVE", "commit_message": "work in progress"},
			},
		}

		res, err := srv.handleGitStash(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		var parsed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(text), &parsed), "result should be valid JSON")
		assert.Equal(t, true, parsed["success"])
		mockGit.AssertExpectations(t)
	})

	t.Run("POP untracked conflict returns error hint", func(t *testing.T) {
		conflictMock := new(MockGit)
		conflictSrv := &Server{git: conflictMock}
		conflictMock.On("CreateBackup", "POP", domain.StashNone).Return(domain.Backup{}, nil)
		conflictMock.On("StashPop").Return("", fmt.Errorf("STASH_POP_UNTRACKED: files conflict"))

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "git_stash",
				Arguments: map[string]any{"command": "POP"},
			},
		}

		res, err := conflictSrv.handleGitStash(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)
		text := res.Content[0].(mcpgo.TextContent).Text
		assert.Contains(t, text, "untracked files conflict")
		conflictMock.AssertExpectations(t)
	})

	t.Run("SHOW does NOT create backup (read-only)", func(t *testing.T) {
		mockGit.On("StashShow").Return("stash contents", nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "git_stash",
				Arguments: map[string]any{"command": "SHOW"},
			},
		}

		res, err := srv.handleGitStash(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		assert.Contains(t, text, "stash contents")
		mockGit.AssertExpectations(t)
	})
}
