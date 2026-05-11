package mcp

import (
	"context"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleGitDiff_ArgRejected(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}

	args := map[string]any{"command": "READ_DIFF", "arg": "file.go", "path": "file.go"}
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_diff",
			Arguments: args,
		},
	}

	res, err := srv.handleGitDiff(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.True(t, strings.Contains(text, "unknown parameter"), "expected 'unknown parameter' error for 'arg', got: %s", text)
}

func TestHandleGitDiff_ReadDiffWithPath(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}
	mockGit.On("Diff", []string{"main.go"}).Return("diff output", nil)

	args := map[string]any{"command": "READ_DIFF", "path": "main.go"}
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_diff",
			Arguments: args,
		},
	}

	res, err := srv.handleGitDiff(context.Background(), req)
	assert.NoError(t, err)
	if res != nil {
		text := res.Content[0].(mcpgo.TextContent).Text
		assert.False(t, strings.Contains(text, "unknown parameter"), "should not reject 'path' param, got: %s", text)
	}
}

func TestHandleGitDiff_ValidParamsAccepted(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{git: mockGit}
	mockGit.On("Diff", []string{"file.go"}).Return("diff output", nil)

	args := map[string]any{
		"command": "READ_DIFF",
		"path":   "file.go",
		"filter": "",
		"limit":  float64(10),
		"offset": float64(0),
		"compact": false,
	}
	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name:      "git_diff",
			Arguments: args,
		},
	}

	res, err := srv.handleGitDiff(context.Background(), req)
	assert.NoError(t, err)
	if res != nil {
		text := res.Content[0].(mcpgo.TextContent).Text
		assert.False(t, strings.Contains(text, "unknown parameter"), "should not reject valid params, got: %s", text)
	}
}