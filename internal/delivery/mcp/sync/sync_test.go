package sync

import (
	"context"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleSync_PullWithBranch(t *testing.T) {
	gitMock := new(MockGit)
	backup := domain.Backup{Ref: "ref", Operation: "PULL"}
	gitMock.On("CreateBackup", "PULL", domain.StashNone).Return(backup, nil)
	gitMock.On("PullFromBranch", "upstream", "develop").Return("pulled develop from upstream", nil)

	h := NewHandler(gitMock)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{
				"command":     "PULL",
				"remote_name": "upstream",
				"branch":      "develop",
			},
		},
	}

	res, err := h.HandleSync(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "PULL", "pull with branch should succeed")
	gitMock.AssertExpectations(t)
}

func TestHandleSync_PullWithoutBranch(t *testing.T) {
	gitMock := new(MockGit)
	backup := domain.Backup{Ref: "ref", Operation: "PULL"}
	gitMock.On("CreateBackup", "PULL", domain.StashNone).Return(backup, nil)
	gitMock.On("PullFrom", "origin").Return("pulled", nil)

	h := NewHandler(gitMock)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{
				"command": "PULL",
			},
		},
	}

	res, err := h.HandleSync(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "PULL", "pull without branch should succeed")
	gitMock.AssertExpectations(t)
}

func TestHandleSync_PushWithBranch(t *testing.T) {
	gitMock := new(MockGit)
	gitMock.On("PushToBranch", "origin", "feature").Return("pushed feature to origin", nil).Once()
	gitMock.On("PushToBranch", "origin", "refs/courer/feature").Return("pushed ref", nil)

	h := NewHandler(gitMock)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{
				"command":   "PUSH",
				"confirmed": true,
				"branch":    "feature",
			},
		},
	}

	res, err := h.HandleSync(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "PUSH", "push with branch should succeed")
	gitMock.AssertExpectations(t)
}

func TestHandleSync_PushWithoutBranch(t *testing.T) {
	gitMock := new(MockGit)
	gitMock.On("PushTo", "origin").Return("pushed", nil)

	h := NewHandler(gitMock)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{
				"command":   "PUSH",
				"confirmed": true,
			},
		},
	}

	res, err := h.HandleSync(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "PUSH", "push without branch should succeed")
	gitMock.AssertExpectations(t)
}
