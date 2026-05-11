package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleGitTag(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{
		git: mockGit,
	}

	tests := []struct {
		name       string
		command    string
		args       map[string]any
		setup      func()
		wantInJSON string
		wantErr    bool
		errContain string
	}{
		{
			name:    "CREATE with tag_name",
			command: "CREATE",
			args:    map[string]any{"tag_name": "v1.0.0"},
			setup: func() {
				mockGit.On("CreateBackup", "CREATE", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("Tag", "v1.0.0", "").Return("created tag", nil)
			},
			wantInJSON: "tag_created",
		},
		{
			name:    "DELETE with tag_name",
			command: "DELETE",
			args:    map[string]any{"tag_name": "v1.0.0"},
			setup: func() {
				mockGit.On("CreateBackup", "DELETE", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("DeleteTag", "v1.0.0").Return("deleted", nil)
			},
			wantInJSON: "tag_deleted",
		},
		{
			name:    "PUSH with tag_name",
			command: "PUSH",
			args:    map[string]any{"tag_name": "v1.0.0"},
			setup: func() {
				mockGit.On("CreateBackup", "PUSH", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("PushTag", "v1.0.0").Return("pushed", nil)
			},
			wantInJSON: "tag_pushed",
		},
		{
			name:    "DELETE_REMOTE with tag_name",
			command: "DELETE_REMOTE",
			args:    map[string]any{"tag_name": "v1.0.0"},
			setup: func() {
				mockGit.On("CreateBackup", "DELETE_REMOTE", domain.StashNone).Return(domain.Backup{}, nil)
				mockGit.On("DeleteTagRemote", "v1.0.0").Return("deleted remote", nil)
			},
			wantInJSON: "tag_deleted from remote",
		},
		{
			name:       "CREATE missing tag_name",
			command:    "CREATE",
			args:       map[string]any{},
			setup:      func() {},
			wantErr:    true,
			errContain: "tag_name is required for CREATE",
		},
		{
			name:       "DELETE missing tag_name",
			command:    "DELETE",
			args:       map[string]any{},
			setup:      func() {},
			wantErr:    true,
			errContain: "tag_name is required for DELETE",
		},
		{
			name:       "PUSH missing tag_name",
			command:    "PUSH",
			args:       map[string]any{},
			setup:      func() {},
			wantErr:    true,
			errContain: "tag_name is required for PUSH",
		},
		{
			name:       "DELETE_REMOTE missing tag_name",
			command:    "DELETE_REMOTE",
			args:       map[string]any{},
			setup:      func() {},
			wantErr:    true,
			errContain: "tag_name is required for DELETE_REMOTE",
		},
		{
			name:       "Unknown command returns error with suggestion",
			command:    "CREAT",
			args:       map[string]any{"tag_name": "v1.0.0"},
			setup:      func() {},
			wantErr:    true,
			errContain: "unknown command",
		},
		{
			name:       "Unknown param returns error",
			command:    "CREATE",
			args:       map[string]any{"tag_name": "v1.0.0", "arg": "v1.0.0"},
			setup:      func() {},
			wantErr:    true,
			errContain: "unknown parameter: arg",
		},
		{
			name:       "Empty command returns error",
			command:    "",
			args:       map[string]any{},
			setup:      func() {},
			wantErr:    true,
			errContain: "command is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			args := map[string]any{}
			if tt.command != "" {
				args["command"] = tt.command
			}
			for k, v := range tt.args {
				args[k] = v
			}
			req := mcpgo.CallToolRequest{
				Params: mcpgo.CallToolParams{
					Name:      "git_tag",
					Arguments: args,
				},
			}

			res, err := srv.handleGitTag(context.Background(), req)

			if tt.wantErr {
				assert.NoError(t, err, "jsonErrorResult should not return a Go error")
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

func TestHandleGitTag_ValidJSON(t *testing.T) {
	mockGit := new(MockGit)
	srv := &Server{
		git: mockGit,
	}

	t.Run("CREATE produces valid JSON", func(t *testing.T) {
		mockGit.On("CreateBackup", "CREATE", domain.StashNone).Return(domain.Backup{}, nil)
		mockGit.On("Tag", "v2.0.0", "").Return("created", nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "git_tag",
				Arguments: map[string]any{"command": "CREATE", "tag_name": "v2.0.0"},
			},
		}

		res, err := srv.handleGitTag(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		var parsed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(text), &parsed), "result should be valid JSON")
		mockGit.AssertExpectations(t)
	})

	t.Run("DELETE_REMOTE uses DeleteTagRemote (unified)", func(t *testing.T) {
		mockGit.On("CreateBackup", "DELETE_REMOTE", domain.StashNone).Return(domain.Backup{}, nil)
		mockGit.On("DeleteTagRemote", "v1.0.0").Return("deleted remote tag", nil)

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "git_tag",
				Arguments: map[string]any{"command": "DELETE_REMOTE", "tag_name": "v1.0.0"},
			},
		}

		res, err := srv.handleGitTag(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		assert.Contains(t, text, "v1.0.0")
		mockGit.AssertExpectations(t)
	})

	t.Run("unknown command error produces valid JSON", func(t *testing.T) {
		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "git_tag",
				Arguments: map[string]any{"command": "BOGUS"},
			},
		}

		res, err := srv.handleGitTag(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		var parsed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(text), &parsed), "error result should be valid JSON")
		assert.Contains(t, parsed["error"], "unknown command")
	})
}
