package branching

import (
	"context"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleMerge_ContinueCallsMergeContinue(t *testing.T) {
	gitMock := new(MockGit)
	gitMock.On("MergeContinue").Return("merge completed", nil)

	h := NewHandler(gitMock)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{"continue": true},
		},
	}

	res, err := h.HandleMerge(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "MERGE_CONTINUE", "merge continue should succeed")
	assert.Contains(t, text, "merge conflict resolved and committed", "should include success message")
	gitMock.AssertExpectations(t)
}

func TestHandleMerge_ContinueError(t *testing.T) {
	gitMock := new(MockGit)
	gitMock.On("MergeContinue").Return("", assert.AnError)

	h := NewHandler(gitMock)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{"continue": true},
		},
	}

	res, err := h.HandleMerge(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "error", "merge continue error should be reported")
	gitMock.AssertExpectations(t)
}

func TestHandleMerge_AbortStillWorks(t *testing.T) {
	gitMock := new(MockGit)
	gitMock.On("MergeAbort").Return("aborted", nil)

	h := NewHandler(gitMock)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{"abort": true},
		},
	}

	res, err := h.HandleMerge(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "MERGE_ABORT")
	gitMock.AssertExpectations(t)
}

func TestHandleMerge_SkipCallsMergeSkip(t *testing.T) {
	gitMock := new(MockGit)
	gitMock.On("MergeSkip").Return("merge skip completed", nil)

	h := NewHandler(gitMock)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{"skip": true},
		},
	}

	res, err := h.HandleMerge(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "MERGE_SKIP", "merge skip should succeed")
	assert.Contains(t, text, "merge skip completed", "should include success message")
	gitMock.AssertExpectations(t)
}

func TestHandleMerge_SkipError(t *testing.T) {
	gitMock := new(MockGit)
	gitMock.On("MergeSkip").Return("", assert.AnError)

	h := NewHandler(gitMock)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{"skip": true},
		},
	}

	res, err := h.HandleMerge(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "error", "merge skip error should be reported")
	gitMock.AssertExpectations(t)
}
