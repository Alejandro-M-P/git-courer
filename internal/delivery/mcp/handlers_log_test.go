package mcp

import (
	"context"
	"encoding/json"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleGitLog_ReadLog(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{
		git: mockGit,
	}

	mockLog := "123456|Test|2024-01-01|Initial commit"
	mockGit.On("Log", 20, "", []string(nil)).Return(mockLog, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "git_log",
			Arguments: map[string]any{
				"command": "READ_LOG",
			},
		},
	}

	res, err := srv.handleGitLog(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Contains(t, result, "commits")
	commits := result["commits"].([]any)
	assert.Len(t, commits, 1)
	assert.Equal(t, "123456", commits[0].(map[string]any)["hash"])
}
