package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleGitSync(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{
		git: mockGit,
	}

	tests := []struct {
		name     string
		command  string
		args     map[string]any
		setup    func()
		expected string
	}{
		{
			name:    "FETCH",
			command: "FETCH",
			setup: func() {
				mockGit.On("CreateBackup", "FETCH", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("Fetch").Return("fetched", nil)
			},
			expected: "Fetched from remote",
		},
		{
			name:    "PULL",
			command: "PULL",
			args:    map[string]any{"remote": "origin"},
			setup: func() {
				mockGit.On("CreateBackup", "PULL", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("PullFrom", "origin").Return("pulled", nil)
			},
			expected: "Pulled from origin",
		},
		{
			name:    "PUSH",
			command: "PUSH",
			args:    map[string]any{"remote": "origin"},
			setup: func() {
				mockGit.On("CreateBackup", "PUSH", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("PushTo", "origin").Return("pushed", nil)
			},
			expected: "Pushed to origin",
		},
		{
			name:    "MERGE",
			command: "MERGE",
			args:    map[string]any{"arg": "feature"},
			setup: func() {
				mockGit.On("CreateBackup", "MERGE", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("Merge", "feature").Return("merged", nil)
			},
			expected: "Merged feature",
		},
		{
			name:    "SWITCH",
			command: "SWITCH",
			args:    map[string]any{"arg": "main"},
			setup: func() {
				mockGit.On("CreateBackup", "SWITCH", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("Switch", "main").Return(nil)
			},
			expected: "Switched to main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			args := map[string]any{"command": tt.command}
			for k, v := range tt.args {
				args[k] = v
			}
			req := mcpgo.CallToolRequest{
				Params: mcpgo.CallToolParams{
					Name:      "git_sync",
					Arguments: args,
				},
			}

			res, err := srv.handleGitSync(context.Background(), req)
			assert.NoError(t, err)
			assert.NotNil(t, res)

			var result map[string]any
			err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
			assert.NoError(t, err)
			assert.Contains(t, result["message"], tt.expected)
			mockGit.AssertExpectations(t)
		})
	}
}
