package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleGitSync(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		args     map[string]any
		setup    func(*MockGit)
		expected string
	}{
		{
			name:    "FETCH",
			command: "FETCH",
			setup: func(m *MockGit) {
				m.On("CreateBackup", "FETCH", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("Fetch").Return("fetched", nil)
			},
			expected: "Fetched from remote",
		},
		{
			name:    "PULL with remote",
			command: "PULL",
			args:    map[string]any{"remote": "origin"},
			setup: func(m *MockGit) {
				m.On("CreateBackup", "PULL", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("PullFrom", "origin").Return("pulled", nil)
			},
			expected: "Pulled from origin",
		},
		{
			name:    "PUSH with remote",
			command: "PUSH",
			args:    map[string]any{"remote": "origin"},
			setup: func(m *MockGit) {
				m.On("CreateBackup", "PUSH", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("PushTo", "origin").Return("pushed", nil)
			},
			expected: "Pushed to origin",
		},
		{
			name:    "MERGE with branch param",
			command: "MERGE",
			args:    map[string]any{"branch": "feature"},
			setup: func(m *MockGit) {
				m.On("CreateBackup", "MERGE", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("Merge", "feature").Return("merged", nil)
			},
			expected: "Merged feature",
		},
		{
			name:    "SWITCH with branch param",
			command: "SWITCH",
			args:    map[string]any{"branch": "main"},
			setup: func(m *MockGit) {
				m.On("CreateBackup", "SWITCH", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("Switch", "main").Return(nil)
			},
			expected: "Switched to main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := new(MockGit)
			srv := &Server{git: mockGit}
			tt.setup(mockGit)

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

func TestHandleGitSync_MissingBranch(t *testing.T) {
	tests := []struct {
		name    string
		command string
	}{
		{name: "MERGE without branch", command: "MERGE"},
		{name: "SWITCH without branch", command: "SWITCH"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := new(MockGit)
			srv := &Server{git: mockGit}

			mockGit.On("CreateBackup", tt.command, domain.StashNone).Return(domain.Backup{}, nil)

			args := map[string]any{"command": tt.command}
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
			assert.Contains(t, result["error"], "branch is required for "+tt.command)
		})
	}
}

func TestHandleGitSync_UnknownCommand(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}

	args := map[string]any{"command": "REBASE"}
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
	assert.Contains(t, result["error"], "unknown command")
}

func TestHandleGitSync_ArgRejected(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}

	// Sending `arg` param should cause validation error (unknown parameter)
	args := map[string]any{"command": "FETCH", "arg": "something"}
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_sync",
			Arguments: args,
		},
	}

	res, err := srv.handleGitSync(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.True(t, strings.Contains(text, "unknown parameter"))
}