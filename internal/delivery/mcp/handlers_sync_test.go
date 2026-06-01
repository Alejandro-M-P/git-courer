package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpsync "github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/sync"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

// TestHandleSync_V2 tests the sync domain handler directly.
func TestHandleSync_V2(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		args       map[string]any
		wantInJSON string
		wantErr    bool
		errContain string
	}{
		{
			name:       "PULL with remote_name",
			command:    "PULL",
			args:       map[string]any{"remote_name": "origin"},
			wantInJSON: "Pulled from origin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockGit := new(MockGit)
			handler := mcpsync.NewHandler(mockGit)

			mockGit.On("CreateBackup", tt.command, domain.StashNone).Return(domain.Backup{}, nil)
			if tt.command == "PULL" {
				mockGit.On("PullFrom", "origin").Return("pulled", nil)
			}

			args := map[string]any{"command": tt.command}
			for k, v := range tt.args {
				args[k] = v
			}
			req := mcpgo.CallToolRequest{
				Params: mcpgo.CallToolParams{
					Name:      "sync",
					Arguments: args,
				},
			}

			res, err := handler.HandleSync(context.Background(), req)
			if tt.wantErr {
				assert.NoError(t, err)
				assert.NotNil(t, res)
				if res != nil && len(res.Content) > 0 {
					text := res.Content[0].(mcpgo.TextContent).Text
					assert.Contains(t, text, tt.errContain)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, res)
				var result map[string]any
				err = json.Unmarshal([]byte(res.Content[0].(mcpgo.TextContent).Text), &result)
				assert.NoError(t, err)
				assert.Contains(t, result["message"], tt.wantInJSON)
			}
			mockGit.AssertExpectations(t)
		})
	}
}
