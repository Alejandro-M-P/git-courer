package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleGitStatus_ArgRejected(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}
	mockGit.On("Status").Return(domain.Status{}, nil)

	args := map[string]any{"command": "READ_STATUS", "arg": "something"}
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_status",
			Arguments: args,
		},
	}

	res, err := srv.handleGitStatus(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.True(t, strings.Contains(text, "unknown parameter"), "expected 'unknown parameter' error for 'arg', got: %s", text)
}

func TestHandleGitStatus_ValidParamsAccepted(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}
	mockGit.On("Status").Return(domain.Status{}, nil)

	args := map[string]any{
		"command": "READ_STATUS",
		"filter":  "",
	}
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_status",
			Arguments: args,
		},
	}

	res, err := srv.handleGitStatus(context.Background(), req)
	assert.NoError(t, err)
	if res != nil {
		text := res.Content[0].(mcpgo.TextContent).Text
		assert.False(t, strings.Contains(text, "unknown parameter"), "should not reject valid params, got: %s", text)
	}
}