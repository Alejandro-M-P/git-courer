package core

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/delivery/mcp/descriptions"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHandleCommit_NoAI_BypassLLM(t *testing.T) {
	mGit := new(mockGit)
	h := NewHandler(mGit, nil, nil, nil, "", nil, nil)
	h.SetLLMEnabled(false)

	// Mock git calls
	mGit.On("Add", []string{"somefile.go"}).Return(nil)
	mGit.On("WriteTree").Return("mocktreehash123", nil).Once()

	// Call PREVIEW with valid message
	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"command":      "PREVIEW",
		"message":      "feat: offline commit message",
		"target_paths": "somefile.go",
	}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	// Unmarshal result
	var resp struct {
		Status  string `json:"status"`
		JobID   string `json:"job_id"`
		Message string `json:"message"`
	}
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
	assert.Contains(t, resp.Message, "feat: offline commit message")
	assert.NotEmpty(t, resp.JobID)

	// Check job registration
	jobVal, found := h.bgJobs.Load(resp.JobID)
	assert.True(t, found)
	bgJob := jobVal.(*BgJob)
	assert.Equal(t, BgDone, bgJob.Status)
	assert.Equal(t, "feat: offline commit message", bgJob.Message)
	assert.Equal(t, "mocktreehash123", bgJob.TreeHash)

	// Now apply plumbing commit
	mGit.On("Head").Return("parenthash456", nil)
	// mock CommitTree(treeHash, parentHash, message)
	mGit.On("CommitTree", "mocktreehash123", "parenthash456", "feat: offline commit message").Return("newcommithash789", nil).Once()
	mGit.On("UpdateRef", "HEAD", "newcommithash789").Return("SUCCESS", nil).Once()
	mGit.On("Add", []string{domain.MetadataDir}).Return(nil)
	mGit.On("WriteTree").Return("treehashwithmetadata", nil).Once()
	mGit.On("CommitTree", "treehashwithmetadata", "parenthash456", "feat: offline commit message").Return("replacementhash", nil).Once()
	mGit.On("UpdateRef", "HEAD", "replacementhash").Return("SUCCESS", nil).Once()
	mGit.On("Reset", "HEAD", ".").Return("SUCCESS", nil)

	reqApply := mcpgo.CallToolRequest{}
	reqApply.Params.Arguments = map[string]any{
		"command": "APPLY",
		"job_id":  resp.JobID,
	}

	resApply, err := h.HandleCommit(context.Background(), reqApply)
	assert.NoError(t, err)
	assert.NotNil(t, resApply)

	mGit.AssertExpectations(t)
}

func TestHandleCommit_NoAI_MissingMessage(t *testing.T) {
	mGit := new(mockGit)
	h := NewHandler(mGit, nil, nil, nil, "", nil, nil)
	h.SetLLMEnabled(false)

	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"command": "PREVIEW",
		// missing message
	}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.True(t, res.IsError)

	var errResp struct {
		Status  string `json:"status"`
		Command string `json:"command"`
		Error   string `json:"error"`
	}
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &errResp)
	assert.NoError(t, err)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.Error, "message parameter is required")
}

func TestRegistration_LLMEnabledVsDisabled(t *testing.T) {
	// Test case 1: LLM Enabled
	{
		s := server.NewMCPServer("test-server", "1.0.0")
		mGit := new(mockGit)
		h := NewHandler(mGit, nil, nil, nil, "", nil, nil)
		h.SetLLMEnabled(true)

		Register(s, h)

		tools := s.ListTools()
		var commitTool *mcpgo.Tool
		for _, tool := range tools {
			if tool.Tool.Name == "commit" {
				t := tool.Tool
				commitTool = &t
				break
			}
		}
		assert.NotNil(t, commitTool)
		assert.Equal(t, descriptions.DescCommit, commitTool.Description)
		properties := commitTool.InputSchema.Properties
		_, hasWhy := properties["why"]
		assert.True(t, hasWhy)
		
		requiredFields := commitTool.InputSchema.Required
		isWhyRequired := false
		for _, f := range requiredFields {
			if f == "why" {
				isWhyRequired = true
			}
		}
		assert.True(t, isWhyRequired)
		
		_, hasMessage := properties["message"]
		assert.False(t, hasMessage)
	}

	// Test case 2: LLM Disabled
	{
		s := server.NewMCPServer("test-server", "1.0.0")
		mGit := new(mockGit)
		h := NewHandler(mGit, nil, nil, nil, "", nil, nil)
		h.SetLLMEnabled(false)

		Register(s, h)

		tools := s.ListTools()
		var commitTool *mcpgo.Tool
		for _, tool := range tools {
			if tool.Tool.Name == "commit" {
				t := tool.Tool
				commitTool = &t
				break
			}
		}
		assert.NotNil(t, commitTool)
		assert.Equal(t, descriptions.DescCommitNoAI, commitTool.Description)
		properties := commitTool.InputSchema.Properties
		_, hasMessage := properties["message"]
		assert.True(t, hasMessage)
		
		_, hasWhy := properties["why"]
		assert.False(t, hasWhy)
		
		requiredFields := commitTool.InputSchema.Required
		isMessageRequired := false
		isWhyRequired := false
		for _, f := range requiredFields {
			if f == "message" {
				isMessageRequired = true
			}
			if f == "why" {
				isWhyRequired = true
			}
		}
		assert.True(t, isMessageRequired)
		assert.False(t, isWhyRequired)
	}
}

func TestHandleCommit_NoAI_InferredPrefix(t *testing.T) {
	mGit := new(mockGit)
	h := NewHandler(mGit, nil, nil, nil, "", nil, nil)
	h.SetLLMEnabled(false)

	// Mock git calls
	mGit.On("Add", []string{"somefile.go"}).Return(nil)
	mGit.On("Status").Return(domain.Status{
		Files: []domain.FileStatus{
			{Path: "somefile.go", Staged: true},
		},
	}, nil)
	mGit.On("DiffStaged", mock.Anything).Return("diff --git a/somefile.go b/somefile.go\nnew file mode 100644\n--- /dev/null\n+++ b/somefile.go", nil)
	mGit.On("WriteTree").Return("mocktreehash123", nil).Once()

	// Call PREVIEW with message without prefix
	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"command":      "PREVIEW",
		"message":      "added a cool new feature",
		"target_paths": "somefile.go",
	}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	// Unmarshal result
	var resp struct {
		Status  string `json:"status"`
		JobID   string `json:"job_id"`
		Message string `json:"message"`
	}
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
	assert.Contains(t, resp.Message, "feat: added a cool new feature")
}

func TestHandleCommit_NoAI_KeepsPrefix(t *testing.T) {
	mGit := new(mockGit)
	h := NewHandler(mGit, nil, nil, nil, "", nil, nil)
	h.SetLLMEnabled(false)

	// Mock git calls
	mGit.On("Add", []string{"somefile.go"}).Return(nil)
	// Status and DiffStaged should NOT be called because prefix is already valid!
	mGit.On("WriteTree").Return("mocktreehash123", nil).Once()

	// Call PREVIEW with message that already starts with "docs:"
	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"command":      "PREVIEW",
		"message":      "docs: update documentation",
		"target_paths": "somefile.go",
	}

	res, err := h.HandleCommit(context.Background(), req)
	assert.NoError(t, err)
	assert.NotNil(t, res)

	// Unmarshal result
	var resp struct {
		Status  string `json:"status"`
		JobID   string `json:"job_id"`
		Message string `json:"message"`
	}
	err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "success", resp.Status)
	assert.Contains(t, resp.Message, "docs: update documentation")
}

