package branching

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleMerge_StructuredConflictJSON(t *testing.T) {
	tests := []struct {
		name          string
		branch        string
		conflictFiles []string
		wantStatus    string
	}{
		{
			name:          "merge conflict with files",
			branch:        "feature",
			conflictFiles: []string{"main.go", "README.md"},
			wantStatus:    "conflict",
		},
		{
			name:          "merge conflict with single file",
			branch:        "bugfix",
			conflictFiles: []string{"pkg/handler.go"},
			wantStatus:    "conflict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := new(MockGit)
			handler := NewHandler(mockGit)

			backup := domain.Backup{Ref: "ref", Operation: "MERGE"}
			mockGit.On("CreateBackup", "MERGE", domain.StashNone).Return(backup, nil)
			mockGit.On("Merge", tt.branch).Return("", fmt.Errorf("CONFLICT: merge conflict"))
			mockGit.On("DeleteBackup", backup).Return(nil)
			// Setup status to return conflicted files
			fileStatuses := make([]domain.FileStatus, len(tt.conflictFiles))
			for i, f := range tt.conflictFiles {
				fileStatuses[i] = domain.FileStatus{Path: f, Status: "UU"}
			}
			mockGit.On("Status").Return(domain.Status{
				Branch:     "main",
				Files:      fileStatuses,
				Conflicted: len(tt.conflictFiles),
			}, nil)

			req := mcpgo.CallToolRequest{
				Params: mcpgo.CallToolParams{
					Name:      "merge",
					Arguments: map[string]any{"merge_branch_name": tt.branch},
				},
			}

			res, err := handler.HandleMerge(context.Background(), req)
			assert.NoError(t, err)
			assert.NotNil(t, res)

			text := res.Content[0].(mcpgo.TextContent).Text
			var parsed map[string]any
			assert.NoError(t, json.Unmarshal([]byte(text), &parsed))

			// Verify structured format
			assert.Equal(t, tt.wantStatus, parsed["status"])
			assert.NotNil(t, parsed["message"], "should have message field")
			assert.NotNil(t, parsed["conflicted_files"], "should have conflicted_files field")

			// Verify files match
			filesRaw, _ := parsed["conflicted_files"].([]any)
			assert.Equal(t, len(tt.conflictFiles), len(filesRaw))

			mockGit.AssertExpectations(t)
		})
	}
}

func TestHandleRebase_StructuredConflictJSON(t *testing.T) {
	t.Run("rebase conflict returns structured JSON", func(t *testing.T) {
		mockGit := new(MockGit)
		handler := NewHandler(mockGit)

		backup := domain.Backup{Ref: "ref", Operation: "REBASE"}
		mockGit.On("CreateBackup", "REBASE", domain.StashNone).Return(backup, nil)
		mockGit.On("Rebase", "main").Return("", fmt.Errorf("CONFLICT (content): Merge conflict"))
		mockGit.On("DeleteBackup", backup).Return(nil)
		mockGit.On("Status").Return(domain.Status{
			Branch: "feature",
			Files: []domain.FileStatus{
				{Path: "src/app.go", Status: "UU"},
			},
			Conflicted: 1,
		}, nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "rebase",
				Arguments: map[string]any{"branch_name": "main"},
			},
		}

		res, err := handler.HandleRebase(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		var parsed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(text), &parsed))

		assert.Equal(t, "conflict", parsed["status"])
		assert.Contains(t, parsed["message"], "Resolve conflicts")
		assert.NotNil(t, parsed["conflicted_files"])

		mockGit.AssertExpectations(t)
	})
}
