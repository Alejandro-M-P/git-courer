package mcp

import (
	"context"
	"fmt"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleGitTag(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	// Validate that all params are known for this tool
	if result, err := validateKnownParams(params, []string{"command", "name"}); result != nil || err != nil {
		return result, err
	}

	command := getStringParam(params, "command", "")
	if command == "" {
		return jsonErrorResult("git_tag", fmt.Errorf("command is required for git_tag"))
	}

	validTagCommands := []string{"CREATE", "DELETE", "PUSH", "DELETE_REMOTE"}

	switch command {
	case "CREATE", "DELETE", "PUSH", "DELETE_REMOTE":
		// All tag commands modify state — create backup
		backup, bErr := s.git.CreateBackup(command, domain.StashNone)
		if bErr == nil {
			s.lastBackup = backup
		}
	default:
		hint := suggestCommand(command, validTagCommands)
		if hint != "" {
			return jsonErrorResult(command, fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return jsonErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}

	switch command {
	case "CREATE":
		return s.handleTagCreate(params)
	case "DELETE":
		return s.handleTagDelete(params)
	case "PUSH":
		return s.handleTagPush(params)
	case "DELETE_REMOTE":
		return s.handleTagDeleteRemote(params)
	default:
		return jsonErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}
}

func (s *Server) handleTagCreate(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := validateRequiredParam(params, "name", "CREATE"); result != nil || err != nil {
		return result, err
	}

	name := getStringParam(params, "name", "")
	_, err := s.git.Tag(name, "")
	if err != nil {
		return jsonErrorResult("TAG_CREATE", err)
	}

	return mcpgo.NewToolResultText(tagResultJSON("created", name)), nil
}

func (s *Server) handleTagDelete(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := validateRequiredParam(params, "name", "DELETE"); result != nil || err != nil {
		return result, err
	}

	name := getStringParam(params, "name", "")
	_, err := s.git.DeleteTag(name)
	if err != nil {
		return jsonErrorResult("TAG_DELETE", err)
	}

	return mcpgo.NewToolResultText(tagResultJSON("deleted", name)), nil
}

func (s *Server) handleTagPush(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := validateRequiredParam(params, "name", "PUSH"); result != nil || err != nil {
		return result, err
	}

	name := getStringParam(params, "name", "")
	_, err := s.git.PushTag(name)
	if err != nil {
		return jsonErrorResult("TAG_PUSH", err)
	}

	return mcpgo.NewToolResultText(tagResultJSON("pushed", name)), nil
}

func (s *Server) handleTagDeleteRemote(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := validateRequiredParam(params, "name", "DELETE_REMOTE"); result != nil || err != nil {
		return result, err
	}

	name := getStringParam(params, "name", "")
	_, err := s.git.DeleteTagRemote(name)
	if err != nil {
		return jsonErrorResult("TAG_DELETE_REMOTE", err)
	}

	return mcpgo.NewToolResultText(tagResultJSON("deleted from remote", name)), nil
}