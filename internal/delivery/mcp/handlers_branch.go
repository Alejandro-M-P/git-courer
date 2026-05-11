package mcp

import (
	"context"
	"fmt"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleGitBranch(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	// First validate that all params are known for this tool
	if result, err := validateKnownParams(params, []string{"command", "name", "new_name", "force"}); result != nil || err != nil {
		return result, err
	}

	command := getStringParam(params, "command", "")
	if command == "" {
		return jsonErrorResult("git_branch", fmt.Errorf("command is required for git_branch"))
	}

	validBranchCommands := []string{"CREATE", "DELETE", "RENAME", "REMOTE_DELETE"}

	switch command {
	case "CREATE", "DELETE", "RENAME", "REMOTE_DELETE":
		// All branch commands modify state — create backup
		backup, bErr := s.git.CreateBackup(command, domain.StashNone)
		if bErr == nil {
			s.lastBackup = backup
		}
	default:
		// Unknown command — provide helpful suggestion
		hint := suggestCommand(command, validBranchCommands)
		if hint != "" {
			return jsonErrorResult(command, fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return jsonErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}

	switch command {
	case "CREATE":
		return s.handleBranchCreate(params)
	case "DELETE":
		return s.handleBranchDelete(params)
	case "RENAME":
		return s.handleBranchRename(params)
	case "REMOTE_DELETE":
		return s.handleBranchRemoteDelete(params)
	default:
		// Unreachable — already validated above
		return jsonErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}
}

func (s *Server) handleBranchCreate(params map[string]any) (*mcpgo.CallToolResult, error) {
	// Validate required parameter
	if result, err := validateRequiredParam(params, "name", "CREATE"); result != nil || err != nil {
		return result, err
	}

	name := getStringParam(params, "name", "")
	_, err := s.git.Branch(name)
	if err != nil {
		return jsonErrorResult("CREATE", err)
	}

	return mcpgo.NewToolResultText(writeResultJSON("BRANCH_CREATE", true, fmt.Sprintf("Created branch: %s", name))), nil
}

func (s *Server) handleBranchDelete(params map[string]any) (*mcpgo.CallToolResult, error) {
	// Validate required parameter
	if result, err := validateRequiredParam(params, "name", "DELETE"); result != nil || err != nil {
		return result, err
	}

	name := getStringParam(params, "name", "")
	force := false
	if v, ok := params["force"].(bool); ok {
		force = v
	}

	_, err := s.git.DeleteBranch(name, force)
	if err != nil {
		return jsonErrorResult("DELETE", err)
	}

	return mcpgo.NewToolResultText(writeResultJSON("BRANCH_DELETE", true, fmt.Sprintf("Deleted branch %s", name))), nil
}

func (s *Server) handleBranchRename(params map[string]any) (*mcpgo.CallToolResult, error) {
	// Validate required parameters
	if result, err := validateRequiredParam(params, "name", "RENAME"); result != nil || err != nil {
		return result, err
	}
	if result, err := validateRequiredParam(params, "new_name", "RENAME"); result != nil || err != nil {
		return result, err
	}

	name := getStringParam(params, "name", "")
	newName := getStringParam(params, "new_name", "")

	_, err := s.git.RenameBranch(name, newName)
	if err != nil {
		return jsonErrorResult("RENAME", err)
	}

	return mcpgo.NewToolResultText(writeResultJSON("BRANCH_RENAME", true, fmt.Sprintf("Renamed branch from %s to %s", name, newName))), nil
}

func (s *Server) handleBranchRemoteDelete(params map[string]any) (*mcpgo.CallToolResult, error) {
	// Validate required parameter
	if result, err := validateRequiredParam(params, "name", "REMOTE_DELETE"); result != nil || err != nil {
		return result, err
	}

	name := getStringParam(params, "name", "")

	err := s.git.DeleteRemoteBranch(name)
	if err != nil {
		return jsonErrorResult("REMOTE_DELETE", err)
	}

	return mcpgo.NewToolResultText(writeResultJSON("BRANCH_REMOTE_DELETE", true, fmt.Sprintf("Deleted remote branch %s", name))), nil
}