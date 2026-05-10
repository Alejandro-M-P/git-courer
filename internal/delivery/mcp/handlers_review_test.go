package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleGitReview_JobResult(t *testing.T) {
	srv := &Server{}
	jobID := srv.newBgJob("commit")
	srv.finishBgJob(jobID, "commit-result")

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "git_review",
			Arguments: map[string]any{
				"command": "JOB_RESULT",
				"arg":     jobID,
			},
		},
	}

	res, err := srv.handleGitReview(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	var result map[string]any
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
	assert.NoError(t, err)
	assert.Equal(t, "done", result["status"])
	assert.Equal(t, "commit-result", result["result"])
}

func TestHandleGitReview_Summary(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{
		git: mockGit,
	}

	mockGit.On("Status").Return(domain.Status{Branch: "main", IsClean: true}, nil)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Name: "git_review",
			Arguments: map[string]any{
				"command": "SUMMARY",
			},
		},
	}

	res, err := srv.handleGitReview(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Contains(t, res.Content[0].(mcpgo.TextContent).Text, "Branch: main")
}
