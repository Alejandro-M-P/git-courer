package branching

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/blak0p/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleBranch(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		args       map[string]any
		setup      func(*MockGit)
		wantInJSON string // substring expected in successful JSON result
		wantErr    bool   // true if expecting an error message in the result
		errContain string // substring expected in error JSON result
	}{
		{
			name:    "CREATE with branch_name",
			command: "CREATE",
			args:    map[string]any{"branch_name": "new-branch"},
			setup: func(m *MockGit) {
				m.On("CreateBackup", "CREATE", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("Branch", "new-branch").Return("created", nil)
			},
			wantInJSON: "Created branch: new-branch",
		},
		{
			name:    "CREATE with switch true clean tree",
			command: "CREATE",
			args:    map[string]any{"branch_name": "feature", "switch": true},
			setup: func(m *MockGit) {
				m.On("CreateBackup", "CREATE", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("Status").Return(domain.Status{Branch: "main", IsClean: true}, nil)
				m.On("Branch", "feature").Return("created", nil)
				m.On("Switch", "feature").Return(nil)
			},
			wantInJSON: "Created and switched to branch: feature",
		},
		{
			name:    "CREATE with switch true dirty tree",
			command: "CREATE",
			args:    map[string]any{"branch_name": "feature", "switch": true},
			setup: func(m *MockGit) {
				m.On("CreateBackup", "CREATE", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("Status").Return(domain.Status{Branch: "main", IsClean: false, Modified: 1}, nil)
				m.On("Stash", []string(nil)).Return("stashed", nil)
				m.On("Branch", "feature").Return("created", nil)
				m.On("Switch", "feature").Return(nil)
				m.On("StashPop").Return("popped", nil)
			},
			wantInJSON: "Created and switched to branch: feature",
		},
		{
			name:    "DELETE with force",
			command: "DELETE",
			args:    map[string]any{"branch_name": "old-branch", "force": true, "confirmed": true},
			setup: func(m *MockGit) {
				m.On("CreateBackup", "DELETE", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("DeleteBranch", "old-branch", true).Return("deleted", nil)
			},
			wantInJSON: "Deleted branch old-branch",
		},
		{
			name:    "DELETE without force defaults to false",
			command: "DELETE",
			args:    map[string]any{"branch_name": "old-branch", "confirmed": true},
			setup: func(m *MockGit) {
				m.On("CreateBackup", "DELETE", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("DeleteBranch", "old-branch", false).Return("deleted", nil)
			},
			wantInJSON: "Deleted branch old-branch",
		},
		{
			name:    "RENAME with branch_name and new_branch_name",
			command: "RENAME",
			args:    map[string]any{"branch_name": "old-name", "new_branch_name": "new-name"},
			setup: func(m *MockGit) {
				m.On("CreateBackup", "RENAME", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("RenameBranch", "old-name", "new-name").Return("renamed", nil)
			},
			wantInJSON: "Renamed branch from old-name to new-name",
		},
		{
			name:    "REMOTE_DELETE with branch_name",
			command: "REMOTE_DELETE",
			args:    map[string]any{"branch_name": "remote-branch", "confirmed": true},
			setup: func(m *MockGit) {
				m.On("CreateBackup", "REMOTE_DELETE", domain.StashNone).Return(domain.Backup{}, nil)
				m.On("DeleteRemoteBranch", "remote-branch").Return(nil)
			},
			wantInJSON: "Deleted remote branch remote-branch",
		},
		{
			name:       "CREATE missing branch_name returns error",
			command:    "CREATE",
			args:       map[string]any{},
			setup:      func(m *MockGit) { m.On("CreateBackup", "CREATE", domain.StashNone).Return(domain.Backup{}, nil) },
			wantErr:    true,
			errContain: "branch_name is required for CREATE",
		},
		{
			name:       "DELETE missing branch_name returns error",
			command:    "DELETE",
			args:       map[string]any{"confirmed": true},
			setup:      func(m *MockGit) { m.On("CreateBackup", "DELETE", domain.StashNone).Return(domain.Backup{}, nil) },
			wantErr:    true,
			errContain: "branch_name is required for DELETE",
		},
		{
			name:       "RENAME missing branch_name returns error",
			command:    "RENAME",
			args:       map[string]any{"new_branch_name": "new-name"},
			setup:      func(m *MockGit) { m.On("CreateBackup", "RENAME", domain.StashNone).Return(domain.Backup{}, nil) },
			wantErr:    true,
			errContain: "branch_name is required for RENAME",
		},
		{
			name:       "RENAME missing new_branch_name returns error",
			command:    "RENAME",
			args:       map[string]any{"branch_name": "old-name"},
			setup:      func(m *MockGit) { m.On("CreateBackup", "RENAME", domain.StashNone).Return(domain.Backup{}, nil) },
			wantErr:    true,
			errContain: "new_branch_name is required for RENAME",
		},
		{
			name:       "REMOTE_DELETE missing branch_name returns error",
			command:    "REMOTE_DELETE",
			args:       map[string]any{"confirmed": true},
			setup:      func(m *MockGit) { m.On("CreateBackup", "REMOTE_DELETE", domain.StashNone).Return(domain.Backup{}, nil) },
			wantErr:    true,
			errContain: "branch_name is required for REMOTE_DELETE",
		},
		{
			name:       "Unknown command returns error with suggestion",
			command:    "CREAT",
			args:       map[string]any{"branch_name": "test"},
			setup:      func(m *MockGit) {},
			wantErr:    true,
			errContain: "unknown command",
		},
		{
			name:       "Unknown param returns error",
			command:    "CREATE",
			args:       map[string]any{"branch_name": "feat", "arg": "feat"},
			setup:      func(m *MockGit) {},
			wantErr:    true,
			errContain: "unknown parameter: arg",
		},
		{
			name:       "Empty command returns error",
			command:    "",
			args:       map[string]any{},
			setup:      func(m *MockGit) {},
			wantErr:    true,
			errContain: "command is required",
		},
		{
			name:       "DELETE without confirmed is blocked",
			command:    "DELETE",
			args:       map[string]any{"branch_name": "old-branch"},
			setup:      func(m *MockGit) {},
			wantErr:    true,
			errContain: "confirmed=true",
		},
		{
			name:       "REMOTE_DELETE without confirmed is blocked",
			command:    "REMOTE_DELETE",
			args:       map[string]any{"branch_name": "remote-branch"},
			setup:      func(m *MockGit) {},
			wantErr:    true,
			errContain: "confirmed=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := new(MockGit)
			branchingHandler := NewHandler(mockGit)
			tt.setup(mockGit)

			args := map[string]any{}
			if tt.command != "" {
				args["command"] = tt.command
			}
			for k, v := range tt.args {
				args[k] = v
			}
			req := mcpgo.CallToolRequest{
				Params: mcpgo.CallToolParams{
					Name:      "branch",
					Arguments: args,
				},
			}

			res, err := branchingHandler.HandleBranch(context.Background(), req)

			if tt.wantErr {
				assert.NoError(t, err, "JSONErrorResult should not return a Go error")
				assert.NotNil(t, res, "error result should not be nil")
				if res != nil && len(res.Content) > 0 {
					text := res.Content[0].(mcpgo.TextContent).Text
					assert.Contains(t, text, tt.errContain)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, res)
				if res != nil && len(res.Content) > 0 {
					text := res.Content[0].(mcpgo.TextContent).Text
					assert.Contains(t, text, tt.wantInJSON)
				}
			}
			mockGit.AssertExpectations(t)
		})
	}
}

// Test that successful branch operations produce valid JSON
func TestHandleBranch_ValidJSON(t *testing.T) {
	t.Run("CREATE produces valid JSON", func(t *testing.T) {
		mockGit := new(MockGit)
		handler := NewHandler(mockGit)

		mockGit.On("CreateBackup", "CREATE", domain.StashNone).Return(domain.Backup{}, nil)
		mockGit.On("Branch", "feature").Return("created", nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "branch",
				Arguments: map[string]any{"command": "CREATE", "branch_name": "feature"},
			},
		}

		res, err := handler.HandleBranch(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		var parsed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(text), &parsed), "result should be valid JSON")
		assert.Equal(t, true, parsed["success"])
		assert.Equal(t, "BRANCH_CREATE", parsed["operation"])
		mockGit.AssertExpectations(t)
	})

	t.Run("unknown command error produces valid JSON", func(t *testing.T) {
		mockGit := new(MockGit)
		handler := NewHandler(mockGit)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "branch",
				Arguments: map[string]any{"command": "BOGUS"},
			},
		}

		res, err := handler.HandleBranch(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		var parsed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(text), &parsed), "error result should be valid JSON")
		assert.Contains(t, parsed["error"], "unknown command")
	})
}

func TestHandleBranch_ListCommand(t *testing.T) {
	mockGit := new(MockGit)
	handler := NewHandler(mockGit)

	mockOutput := `* main
  feature/login
  remotes/origin/main
  remotes/origin/feature/login
  remotes/origin/bugfix/issue-12`

	t.Run("LIST all branches bypasses backups and confirmation", func(t *testing.T) {
		mockGit.On("ListBranches").Return(mockOutput, nil).Once()

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "branch",
				Arguments: map[string]any{"command": "LIST"},
			},
		}

		res, err := handler.HandleBranch(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		var parsed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(text), &parsed))
		assert.Equal(t, true, parsed["success"])
		assert.Equal(t, "BRANCH_LIST", parsed["operation"])

		msg := parsed["message"].(string)
		branches := strings.Split(msg, "\n")
		assert.Contains(t, branches, "main")
		assert.Contains(t, branches, "feature/login")
		assert.Contains(t, branches, "remotes/origin/main")
		assert.Contains(t, branches, "remotes/origin/feature/login")
		assert.Contains(t, branches, "remotes/origin/bugfix/issue-12")

		mockGit.AssertExpectations(t)
	})

	t.Run("LIST LOCAL filter", func(t *testing.T) {
		mockGit.On("ListBranches").Return(mockOutput, nil).Once()

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "branch",
				Arguments: map[string]any{"command": "LIST", "filter": "LOCAL"},
			},
		}

		res, err := handler.HandleBranch(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		var parsed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(text), &parsed))

		msg := parsed["message"].(string)
		branches := strings.Split(msg, "\n")
		assert.Contains(t, branches, "main")
		assert.Contains(t, branches, "feature/login")
		assert.NotContains(t, branches, "remotes/origin/main")

		mockGit.AssertExpectations(t)
	})

	t.Run("LIST REMOTE filter", func(t *testing.T) {
		mockGit.On("ListBranches").Return(mockOutput, nil).Once()

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "branch",
				Arguments: map[string]any{"command": "LIST", "filter": "REMOTE"},
			},
		}

		res, err := handler.HandleBranch(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		var parsed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(text), &parsed))

		msg := parsed["message"].(string)
		branches := strings.Split(msg, "\n")
		assert.NotContains(t, branches, "main")
		assert.Contains(t, branches, "remotes/origin/main")
		assert.Contains(t, branches, "remotes/origin/feature/login")

		mockGit.AssertExpectations(t)
	})

	t.Run("LIST with branch_name glob pattern filtering", func(t *testing.T) {
		mockGit.On("ListBranches").Return(mockOutput, nil).Once()

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "branch",
				Arguments: map[string]any{"command": "LIST", "branch_name": "*/login"},
			},
		}

		res, err := handler.HandleBranch(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		var parsed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(text), &parsed))

		msg := parsed["message"].(string)
		branches := strings.Split(msg, "\n")
		assert.NotContains(t, branches, "main")
		assert.Contains(t, branches, "feature/login")
		assert.Contains(t, branches, "remotes/origin/feature/login")

		mockGit.AssertExpectations(t)
	})
}
