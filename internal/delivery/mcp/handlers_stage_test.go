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

func TestHandleGitStage(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		args     map[string]any
		setup    func(*MockGit)
		expected string
	}{
		{
			name:    "ADD with paths",
			command: "ADD",
			args:    map[string]any{"paths": "a.go b.go"},
			setup: func(m *MockGit) {
				m.On("CreateBackup", "ADD", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("Add", []string{"a.go", "b.go"}).Return(nil)
			},
			expected: "2 files staged",
		},
		{
			name:    "RM with paths",
			command: "RM",
			args:    map[string]any{"paths": "old.go"},
			setup: func(m *MockGit) {
				m.On("CreateBackup", "RM", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("Remove", []string{"old.go"}).Return(nil)
			},
			expected: "1 files removed",
		},
		{
			name:    "RESET_SOFT with commit",
			command: "RESET_SOFT",
			args:    map[string]any{"commit": "abc123"},
			setup: func(m *MockGit) {
				m.On("CreateBackup", "RESET_SOFT", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("ResetSoft", "abc123").Return(nil)
			},
			expected: "Soft reset to abc123",
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
					Name:      "git_stage",
					Arguments: args,
				},
			}

			res, err := srv.handleGitStage(context.Background(), req)
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

func TestHandleGitStage_MissingRequiredParams(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    map[string]any
		wantErr string
	}{
		{
			name:    "ADD without paths",
			command: "ADD",
			args:    map[string]any{},
			wantErr: "paths is required for ADD",
		},
		{
			name:    "RM without paths",
			command: "RM",
			args:    map[string]any{},
			wantErr: "paths is required for RM",
		},
		{
			name:    "RESET_SOFT without commit",
			command: "RESET_SOFT",
			args:    map[string]any{},
			wantErr: "commit is required for RESET_SOFT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := new(MockGit)
			srv := &Server{git: mockGit}

			args := map[string]any{"command": tt.command}
			for k, v := range tt.args {
				args[k] = v
			}
			req := mcpgo.CallToolRequest{
				Params: mcpgo.CallToolParams{
					Name:      "git_stage",
					Arguments: args,
				},
			}

			res, err := srv.handleGitStage(context.Background(), req)
			assert.NoError(t, err)
			assert.NotNil(t, res)

			var result map[string]any
			err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
			assert.NoError(t, err)
			assert.Contains(t, result["error"], tt.wantErr)
		})
	}
}

func TestHandleGitStage_ArgRejected(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}

	args := map[string]any{"command": "ADD", "arg": "a.go", "paths": "a.go"}
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_stage",
			Arguments: args,
		},
	}

	res, err := srv.handleGitStage(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.True(t, strings.Contains(text, "unknown parameter"))
}

func TestHandleGitStage_UnknownCommand(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}

	args := map[string]any{"command": "COMMIT"}
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_stage",
			Arguments: args,
		},
	}

	res, err := srv.handleGitStage(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result["error"], "unknown command")
}