package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleGitLog_ArgRejected(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}

	args := map[string]any{"command": "READ_LOG", "arg": "main", "revision": "main"}
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_log",
			Arguments: args,
		},
	}

	res, err := srv.handleGitLog(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.True(t, strings.Contains(text, "unknown parameter"), "expected 'unknown parameter' error for 'arg', got: %s", text)
}

func TestHandleGitLog_ValidParamsAccepted(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}
	mockGit.On("Log", 20, "", []string(nil)).Return("log output", nil)

	args := map[string]any{
		"command":   "READ_LOG",
		"revision":  "",
		"path":      "",
		"pattern":   "",
		"filter":    "",
	}
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_log",
			Arguments: args,
		},
	}

	res, err := srv.handleGitLog(context.Background(), req)
	assert.NoError(t, err)
	if res != nil {
		text := res.Content[0].(mcpgo.TextContent).Text
		assert.False(t, strings.Contains(text, "unknown parameter"), "should not reject valid params, got: %s", text)
	}
}

func TestHandleGitLog_BlameWithPath(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}
	mockGit.On("Blame", "main.go").Return([]domain.BlameLine{}, nil)

	args := map[string]any{"command": "BLAME", "path": "main.go"}
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_log",
			Arguments: args,
		},
	}

	res, err := srv.handleGitLog(context.Background(), req)
	assert.NoError(t, err)
	if res != nil {
		text := res.Content[0].(mcpgo.TextContent).Text
		assert.False(t, strings.Contains(text, "unknown parameter"), "should not reject 'path' param for BLAME, got: %s", text)
	}
}

func TestHandleGitLog_BlameWithoutPath(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}

	args := map[string]any{"command": "BLAME"}
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_log",
			Arguments: args,
		},
	}

	res, err := srv.handleGitLog(context.Background(), req)
	assert.NoError(t, err)
	if res != nil {
		text := res.Content[0].(mcpgo.TextContent).Text
		assert.True(t, strings.Contains(text, "path is required for BLAME"), "expected 'path is required' error, got: %s", text)
	}
}