package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleGitBackup(t *testing.T) {
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
			name:    "UNDO with last backup",
			command: "UNDO",
			args:    map[string]any{},
			setup: func() {
				srv.lastBackup = domain.Backup{Ref: "backup-ref", Operation: "previous-op"}
				mockGit.On("RestoreBackup", srv.lastBackup).Return(nil)
			},
			wantInJSON: "Successfully reverted",
		},
		{
			name:       "UNDO without last backup returns error",
			command:    "UNDO",
			args:       map[string]any{},
			setup:      func() { srv.lastBackup = domain.Backup{} },
			wantErr:    true,
			errContain: "no operation to undo",
		},
		{
			name:    "LIST with backups",
			command: "LIST",
			args:    map[string]any{},
			setup: func() {
				mockGit.On("ListBackups").Return([]domain.Backup{
					{Ref: "ref1", Operation: "commit", CreatedAt: time.Now()},
				}, nil)
			},
			wantInJSON: "backups",
		},
		{
			name:    "PRUNE with default days",
			command: "PRUNE",
			args:    map[string]any{},
			setup: func() {
				mockGit.On("PruneBackups", 7*24*time.Hour).Return(nil)
			},
			wantInJSON: "7",
		},
		{
			name:    "PRUNE with custom days",
			command: "PRUNE",
			args:    map[string]any{"days": float64(14)},
			setup: func() {
				mockGit.On("PruneBackups", 14*24*time.Hour).Return(nil)
			},
			wantInJSON: "14",
		},
		{
			name:       "Unknown command returns error with suggestion",
			command:    "UND",
			args:       map[string]any{},
			setup:      func() {},
			wantErr:    true,
			errContain: "unknown command",
		},
		{
			name:       "Unknown param returns error",
			command:    "UNDO",
			args:       map[string]any{"arg": "test"},
			setup:      func() { srv.lastBackup = domain.Backup{Ref: "ref1", Operation: "op"} },
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
					Name:      "git_backup",
					Arguments: args,
				},
			}

			res, err := srv.handleGitBackup(context.Background(), req)

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

func TestHandleGitBackup_ValidJSON(t *testing.T) {
	t.Run("UNDO produces valid JSON", func(t *testing.T) {
		mockGit := new(MockGit)
		srv := &Server{
			git:        mockGit,
			lastBackup: domain.Backup{Ref: "backup-ref", Operation: "commit"},
		}
		mockGit.On("RestoreBackup", srv.lastBackup).Return(nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "git_backup",
				Arguments: map[string]any{"command": "UNDO"},
			},
		}

		res, err := srv.handleGitBackup(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		var parsed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(text), &parsed))
		assert.Equal(t, true, parsed["success"])
		assert.Equal(t, "UNDO", parsed["operation"])
		mockGit.AssertExpectations(t)
	})

	t.Run("UNDO clears lastBackup after successful restore", func(t *testing.T) {
		mockGit := new(MockGit)
		srv := &Server{
			git:        mockGit,
			lastBackup: domain.Backup{Ref: "backup-ref", Operation: "commit"},
		}
		mockGit.On("RestoreBackup", srv.lastBackup).Return(nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "git_backup",
				Arguments: map[string]any{"command": "UNDO"},
			},
		}

		_, err := srv.handleGitBackup(context.Background(), req)
		assert.NoError(t, err)
		assert.Equal(t, domain.Backup{}, srv.lastBackup, "lastBackup should be cleared after UNDO")
		mockGit.AssertExpectations(t)
	})

	t.Run("UPDATE_CONFIG command returns unknown command error", func(t *testing.T) {
		mockGit := new(MockGit)
		srv := &Server{
			git: mockGit,
		}

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "git_backup",
				Arguments: map[string]any{"command": "UPDATE_CONFIG"},
			},
		}

		res, err := srv.handleGitBackup(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		assert.Contains(t, text, "unknown command")
	})
}