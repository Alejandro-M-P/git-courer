package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleGitBranch(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{
		git: mockGit,
	}

	tests := []struct {
		name       string
		command    string
		args       map[string]any
		setup      func()
		wantInJSON string // substring expected in successful JSON result
		wantErr    bool   // true if expecting an error message in the result
		errContain string  // substring expected in error JSON result
	}{
		{
			name:       "CREATE with branch_name",
			command:    "CREATE",
			args:       map[string]any{"branch_name": "new-branch"},
			setup: func() {
				mockGit.On("CreateBackup", "CREATE", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("Branch", "new-branch").Return("created", nil)
			},
			wantInJSON: "Created branch: new-branch",
		},
		{
			name:       "DELETE with force",
			command:    "DELETE",
			args:       map[string]any{"branch_name": "old-branch", "force": true},
			setup: func() {
				mockGit.On("CreateBackup", "DELETE", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("DeleteBranch", "old-branch", true).Return("deleted", nil)
			},
			wantInJSON: "Deleted branch old-branch",
		},
		{
			name:       "DELETE without force defaults to false",
			command:    "DELETE",
			args:       map[string]any{"branch_name": "old-branch"},
			setup: func() {
				mockGit.On("CreateBackup", "DELETE", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("DeleteBranch", "old-branch", false).Return("deleted", nil)
			},
			wantInJSON: "Deleted branch old-branch",
		},
		{
			name:       "RENAME with branch_name and new_branch_name",
			command:    "RENAME",
			args:       map[string]any{"branch_name": "old-name", "new_branch_name": "new-name"},
			setup: func() {
				mockGit.On("CreateBackup", "RENAME", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("RenameBranch", "old-name", "new-name").Return("renamed", nil)
			},
			wantInJSON: "Renamed branch from old-name to new-name",
		},
		{
			name:       "REMOTE_DELETE with branch_name",
			command:    "REMOTE_DELETE",
			args:       map[string]any{"branch_name": "remote-branch"},
			setup: func() {
				mockGit.On("CreateBackup", "REMOTE_DELETE", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("DeleteRemoteBranch", "remote-branch").Return(nil)
			},
			wantInJSON: "Deleted remote branch remote-branch",
		},
		{
			name:       "CREATE missing branch_name returns error",
			command:    "CREATE",
			args:       map[string]any{},
			setup:      func() {},
			wantErr:    true,
			errContain: "branch_name is required for CREATE",
		},
		{
			name:       "DELETE missing branch_name returns error",
			command:    "DELETE",
			args:       map[string]any{},
			setup:      func() {},
			wantErr:    true,
			errContain: "branch_name is required for DELETE",
		},
		{
			name:       "RENAME missing branch_name returns error",
			command:    "RENAME",
			args:       map[string]any{"new_branch_name": "new-name"},
			setup:      func() {},
			wantErr:    true,
			errContain: "branch_name is required for RENAME",
		},
		{
			name:       "RENAME missing new_branch_name returns error",
			command:    "RENAME",
			args:       map[string]any{"branch_name": "old-name"},
			setup:      func() {},
			wantErr:    true,
			errContain: "new_branch_name is required for RENAME",
		},
		{
			name:       "REMOTE_DELETE missing branch_name returns error",
			command:    "REMOTE_DELETE",
			args:       map[string]any{},
			setup:      func() {},
			wantErr:    true,
			errContain: "branch_name is required for REMOTE_DELETE",
		},
		{
			name:       "Unknown command returns error with suggestion",
			command:    "CREAT",
			args:       map[string]any{"branch_name": "test"},
			setup:      func() {},
			wantErr:    true,
			errContain: "unknown command",
		},
		{
			name:       "Unknown param returns error",
			command:    "CREATE",
			args:       map[string]any{"branch_name": "feat", "arg": "feat"},
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
					Name:      "git_branch",
					Arguments: args,
				},
			}

			res, err := srv.handleGitBranch(context.Background(), req)

			if tt.wantErr {
				assert.NoError(t, err, "jsonErrorResult should not return a Go error")
				assert.NotNil(t, res, "error result should not be nil")
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

// Test that successful branch operations produce valid JSON
func TestHandleGitBranch_ValidJSON(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{
		git: mockGit,
	}

	t.Run("CREATE produces valid JSON", func(t *testing.T) {
		mockGit.On("CreateBackup", "CREATE", domain.StashNone).Return(domain.Backup{}, nil)
		mockGit.On("Branch", "feature").Return("created", nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "git_branch",
				Arguments: map[string]any{"command": "CREATE", "branch_name": "feature"},
			},
		}

		res, err := srv.handleGitBranch(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		var parsed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(text), &parsed), "result should be valid JSON")
		assert.Equal(t, true, parsed["success"])
		assert.Equal(t, "BRANCH_CREATE", parsed["operation"])
		mockGit.AssertExpectations(t)
	})

	t.Run("unknown command error produces valid JSON", func(t *testing.T) {
		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "git_branch",
				Arguments: map[string]any{"command": "BOGUS"},
			},
		}

		res, err := srv.handleGitBranch(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		var parsed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(text), &parsed), "error result should be valid JSON")
		assert.Contains(t, parsed["error"], "unknown command")
	})
}
