package mcp

import (
	"context"
	"fmt"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleGitConfig(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	// Validate that all params are known for this tool
	if result, err := validateKnownParams(params, []string{"command"}); result != nil || err != nil {
		return result, err
	}

	command := getStringParam(params, "command", "")
	if command == "" {
		return jsonErrorResult("git_config", fmt.Errorf("command is required for git_config"))
	}

	// Valid commands for git_config — READ and LIST_MODELS only
	// UPDATE_CONFIG is intentionally NOT included (security risk: AI modifying LLM endpoints)
	validConfigCommands := []string{"READ", "LIST_MODELS"}

	switch command {
	case "READ":
		return s.handleConfigRead()
	case "LIST_MODELS":
		return s.handleConfigListModels()
	default:
		hint := suggestCommand(command, validConfigCommands)
		if hint != "" {
			return jsonErrorResult(command, fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return jsonErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}
}

func (s *Server) handleConfigRead() (*mcpgo.CallToolResult, error) {
	result := mustJSON(map[string]interface{}{
		"config_path": config.GlobalConfigPath(),
		"content":     s.cfg,
	})
	return mcpgo.NewToolResultText(result), nil
}

func (s *Server) handleConfigListModels() (*mcpgo.CallToolResult, error) {
	result := mustJSON(map[string]interface{}{
		"provider": s.cfg.LLM.Provider,
		"models":   []string{s.cfg.LLM.Model},
		"message":  "Models are configured statically via config file. Showing current configured model.",
	})
	return mcpgo.NewToolResultText(result), nil
}