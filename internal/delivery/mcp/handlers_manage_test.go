package mcp

import (
	"context"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleGitManage(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{
		git: mockGit,
		cfg: &config.Config{},
	}

	tests := []struct {
		name     string
		command  string
		args     map[string]any
		setup    func()
		expected string
	}{
		{
			name:    "BRANCH_CREATE",
			command: "BRANCH_CREATE",
			args:    map[string]any{"arg": "new-branch"},
			setup: func() {
				mockGit.On("CreateBackup", "BRANCH_CREATE", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("Branch", "new-branch").Return("created", nil)
			},
			expected: "Created branches: new-branch",
		},
		{
			name:    "STASH",
			command: "STASH",
			setup: func() {
				mockGit.On("CreateBackup", "STASH", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("Stash", []string(nil)).Return("stashed", nil)
			},
			expected: "Changes stashed",
		},
		{
			name:    "UNDO",
			command: "UNDO",
			setup: func() {
				srv.lastBackup = domain.Backup{Ref: "backup-ref", Operation: "previous-op"}
				mockGit.On("RestoreBackup", srv.lastBackup).Return(nil)
			},
			expected: "Successfully reverted last operation",
		},
		{
			name:    "LIST_BACKUPS",
			command: "LIST_BACKUPS",
			setup: func() {
				mockGit.On("ListBackups").Return([]domain.Backup{}, nil)
			},
			expected: "[]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			args := map[string]any{"command": tt.command}
			for k, v := range tt.args {
				args[k] = v
			}
			req := mcpgo.CallToolRequest{
				Params: mcpgo.CallToolParams{
					Name:      "git_manage",
					Arguments: args,
				},
			}

			res, err := srv.handleGitManage(context.Background(), req)
			assert.NoError(t, err)
			assert.NotNil(t, res)

			var resultStr string
			if len(res.Content) > 0 {
				resultStr = res.Content[0].(mcpgo.TextContent).Text
			}
			assert.Contains(t, resultStr, tt.expected)
			mockGit.AssertExpectations(t)
		})
	}
}
