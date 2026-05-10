package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleGitStatus_ReadStatus(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{
		git: mockGit,
	}

	mockStatus := domain.Status{
		Branch:  "main",
		IsClean: true,
	}
	mockGit.On("Status").Return(mockStatus, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "git_status",
			Arguments: map[string]any{
				"command": "READ_STATUS",
			},
		},
	}

	res, err := srv.handleGitStatus(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Equal(t, "main", result["branch"])
	assert.Equal(t, true, result["clean"])
}
