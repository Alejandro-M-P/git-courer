package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
)

func TestHandleGitConfig(t *testing.T) {
	srv := &Server{
		cfg: &config.Config{
			LLM: config.LLMConfig{
				Provider: "ollama",
				Model:    "llama3",
				BaseURL:  "http://localhost:11434/v1",
			},
		},
	}

	tests := []struct {
		name       string
		command    string
		args       map[string]any
		wantInJSON string
		wantErr    bool
		errContain string
	}{
		{
			name:       "READ returns config path and content",
			command:    "READ",
			args:       map[string]any{},
			wantInJSON: "config_path",
		},
		{
			name:       "LIST_MODELS returns provider and models",
			command:    "LIST_MODELS",
			args:       map[string]any{},
			wantInJSON: "provider",
		},
		{
			name:       "UPDATE_CONFIG returns unknown command error",
			command:    "UPDATE_CONFIG",
			args:       map[string]any{},
			wantErr:    true,
			errContain: "unknown command",
		},
		{
			name:       "Unknown command returns error with suggestion",
			command:    "REA",
			args:       map[string]any{},
			wantErr:    true,
			errContain: "unknown command",
		},
		{
			name:       "Unknown param returns error",
			command:    "READ",
			args:       map[string]any{"arg": "test"},
			wantErr:    true,
			errContain: "unknown parameter: arg",
		},
		{
			name:       "Empty command returns error",
			command:    "",
			args:       map[string]any{},
			wantErr:    true,
			errContain: "command is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{}
			if tt.command != "" {
				args["command"] = tt.command
			}
			for k, v := range tt.args {
				args[k] = v
			}
			req := mcpgo.CallToolRequest{
				Params: mcpgo.CallToolParams{
					Name:      "git_config",
					Arguments: args,
				},
			}

			res, err := srv.handleGitConfig(context.Background(), req)

			if tt.wantErr {
				assert.NoError(t, err, "jsonErrorResult should not return a Go error")
				assert.NotNil(t, res)
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
		})
	}
}

func TestHandleGitConfig_ValidJSON(t *testing.T) {
	t.Run("READ produces valid JSON with config_path and content", func(t *testing.T) {
		srv := &Server{
			cfg: &config.Config{
				LLM: config.LLMConfig{
					Provider: "openai",
					Model:    "gpt-4",
					BaseURL:  "https://api.openai.com/v1",
				},
			},
		}

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "git_config",
				Arguments: map[string]any{"command": "READ"},
			},
		}

		res, err := srv.handleGitConfig(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		var parsed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(text), &parsed), "result should be valid JSON")
		assert.Contains(t, parsed, "config_path")
		assert.Contains(t, parsed, "content")

		// Content should contain config data (serialized by json.Marshal)
		// json.Marshal uses exported field names when no json tags exist
		contentStr := fmt.Sprintf("%v", parsed["content"])
		assert.Contains(t, contentStr, "openai")
		assert.Contains(t, contentStr, "gpt-4")
	})

	t.Run("LIST_MODELS produces valid JSON with provider and models", func(t *testing.T) {
		srv := &Server{
			cfg: &config.Config{
				LLM: config.LLMConfig{
					Provider: "ollama",
					Model:    "codellama",
				},
			},
		}

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "git_config",
				Arguments: map[string]any{"command": "LIST_MODELS"},
			},
		}

		res, err := srv.handleGitConfig(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		var parsed map[string]any
		assert.NoError(t, json.Unmarshal([]byte(text), &parsed), "result should be valid JSON")
		assert.Equal(t, "ollama", parsed["provider"])
		assert.Contains(t, parsed, "models")
	})

	t.Run("UPDATE_CONFIG explicitly rejected", func(t *testing.T) {
		srv := &Server{
			cfg: &config.Config{},
		}

		req := mcpgo.CallToolRequest{
			Params: mcpgo.CallToolParams{
				Name:      "git_config",
				Arguments: map[string]any{"command": "UPDATE_CONFIG"},
			},
		}

		res, err := srv.handleGitConfig(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, res)

		text := res.Content[0].(mcpgo.TextContent).Text
		// UPDATE_CONFIG should NOT be a valid command — must return unknown command
		assert.Contains(t, text, "unknown command")
		assert.NotContains(t, text, "llm.provider")
	})
}