package branching

import (
	"context"
	"fmt"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleRebase_SkipCallsRebaseSkip(t *testing.T) {
	gitMock := new(MockGit)
	gitMock.On("RebaseSkip").Return("rebase skip completed", nil)

	h := NewHandler(gitMock)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{"skip": true},
		},
	}

	res, err := h.HandleRebase(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "REBASE_SKIP", "rebase skip should succeed")
	assert.Contains(t, text, "rebase skip completed", "should include output from git")
	gitMock.AssertExpectations(t)
}

func TestHandleRebase_SkipError(t *testing.T) {
	gitMock := new(MockGit)
	gitMock.On("RebaseSkip").Return("", assert.AnError)

	h := NewHandler(gitMock)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{"skip": true},
		},
	}

	res, err := h.HandleRebase(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "error", "rebase skip error should be reported")
	gitMock.AssertExpectations(t)
}

func TestHandleRebase_OntoCallsRebaseOnto(t *testing.T) {
	gitMock := new(MockGit)
	gitMock.On("RebaseOnto", "feature", "main", "").Return("rebased onto feature", nil)

	h := NewHandler(gitMock)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{
				"branch_name": "main",
				"onto":        "feature",
			},
		},
	}

	res, err := h.HandleRebase(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "REBASE_ONTO", "rebase onto should succeed")
	assert.Contains(t, text, "feature", "should mention target branch in output")
	gitMock.AssertExpectations(t)
}

func TestHandleRebase_OntoConflictReturnsConflictData(t *testing.T) {
	gitMock := new(MockGit)
	gitMock.On("RebaseOnto", "feature", "main", "").Return("", fmt.Errorf("CONFLICT: merge conflict in main.go"))
	gitMock.On("Status").Return(domain.Status{
		Branch: "current",
		Files: []domain.FileStatus{
			{Path: "main.go", Status: "UU"},
		},
		Conflicted: 1,
	}, nil)

	h := NewHandler(gitMock)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{
				"branch_name": "main",
				"onto":        "feature",
			},
		},
	}

	res, err := h.HandleRebase(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	// Conflict error from RebaseOnto should be caught and return structured conflict data
	assert.Contains(t, text, "conflict", "rebase onto conflict should return conflict data")
	gitMock.AssertExpectations(t)
}

func TestHandleRebase_OntoError(t *testing.T) {
	gitMock := new(MockGit)
	gitMock.On("RebaseOnto", "feature", "main", "").Return("", fmt.Errorf("fatal: not a git repository"))

	h := NewHandler(gitMock)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{
				"branch_name": "main",
				"onto":        "feature",
			},
		},
	}

	res, err := h.HandleRebase(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "error")
	gitMock.AssertExpectations(t)
}

// Test that onto is not used when empty string — falls through to normal rebase
func TestHandleRebase_OntoEmptyFallsThroughToNormalRebase(t *testing.T) {
	backup := domain.Backup{Ref: "ref", Operation: "REBASE"}
	gitMock := new(MockGit)
	gitMock.On("CreateBackup", "REBASE", domain.StashNone).Return(backup, nil)
	gitMock.On("Rebase", "main").Return("rebased", nil)
	gitMock.On("DeleteBackup", backup).Return(nil)

	h := NewHandler(gitMock)

	req := mcpgo.CallToolRequest{
		Params: mcpgo.CallToolParams{
			Arguments: map[string]any{
				"branch_name": "main",
				"onto":        "",
			},
		},
	}

	res, err := h.HandleRebase(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	text := res.Content[0].(mcpgo.TextContent).Text
	assert.Contains(t, text, "REBASE", "empty onto should do normal rebase")
	gitMock.AssertExpectations(t)
}
