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
	if result, err := validateKnownParams(params, []string{"command", "branch_name", "new_branch_name", "remote_name", "force", "confirmed"}); result != nil || err != nil {
		return result, err
	}

	command := getStringParam(params, "command", "")
	if command == "" {
		return jsonErrorResult("git_branch", fmt.Errorf("command is required for git_branch"))
	}

	// Extract safety params early
	confirmed := false
	if v, ok := params["confirmed"].(bool); ok {
		confirmed = v
	}

	validBranchCommands := []string{"CREATE", "DELETE", "RENAME", "REMOTE_DELETE", "SET_UPSTREAM", "UNSET_UPSTREAM"}

	switch command {
	case "CREATE", "DELETE", "RENAME", "REMOTE_DELETE", "SET_UPSTREAM", "UNSET_UPSTREAM":
		// Safety gate for destructive branch commands
		switch command {
		case "DELETE":
			if result, err := checkSafetyGate("branch_delete", false, confirmed); result != nil || err != nil {
				return result, err
			}
		case "REMOTE_DELETE":
			if result, err := checkSafetyGate("remote_delete", false, confirmed); result != nil || err != nil {
				return result, err
			}
		}
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
	case "SET_UPSTREAM":
		return s.handleBranchSetUpstream(params)
	case "UNSET_UPSTREAM":
		return s.handleBranchUnsetUpstream(params)
	default:
		// Unreachable — already validated above
		return jsonErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}
}

func (s *Server) handleBranchCreate(params map[string]any) (*mcpgo.CallToolResult, error) {
	// Validate required parameter
	if result, err := validateRequiredParam(params, "branch_name", "CREATE"); result != nil || err != nil {
		return result, err
	}

	name := getStringParam(params, "branch_name", "")
	_, err := s.git.Branch(name)
	if err != nil {
		return jsonErrorResult("CREATE", err)
	}

	return mcpgo.NewToolResultText(writeResultJSON("BRANCH_CREATE", true, fmt.Sprintf("Created branch: %s", name))), nil
}

func (s *Server) handleBranchDelete(params map[string]any) (*mcpgo.CallToolResult, error) {
	// Validate required parameter
	if result, err := validateRequiredParam(params, "branch_name", "DELETE"); result != nil || err != nil {
		return result, err
	}

	name := getStringParam(params, "branch_name", "")
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
	if result, err := validateRequiredParam(params, "branch_name", "RENAME"); result != nil || err != nil {
		return result, err
	}
	if result, err := validateRequiredParam(params, "new_branch_name", "RENAME"); result != nil || err != nil {
		return result, err
	}

	name := getStringParam(params, "branch_name", "")
	newName := getStringParam(params, "new_branch_name", "")

	_, err := s.git.RenameBranch(name, newName)
	if err != nil {
		return jsonErrorResult("RENAME", err)
	}

	return mcpgo.NewToolResultText(writeResultJSON("BRANCH_RENAME", true, fmt.Sprintf("Renamed branch from %s to %s", name, newName))), nil
}

func (s *Server) handleBranchRemoteDelete(params map[string]any) (*mcpgo.CallToolResult, error) {
	// Validate required parameter
	if result, err := validateRequiredParam(params, "branch_name", "REMOTE_DELETE"); result != nil || err != nil {
		return result, err
	}

	name := getStringParam(params, "branch_name", "")

	err := s.git.DeleteRemoteBranch(name)
	if err != nil {
		return jsonErrorResult("REMOTE_DELETE", err)
	}

	return mcpgo.NewToolResultText(writeResultJSON("BRANCH_REMOTE_DELETE", true, fmt.Sprintf("Deleted remote branch %s", name))), nil
}

func (s *Server) handleBranchSetUpstream(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := validateRequiredParam(params, "branch_name", "SET_UPSTREAM"); result != nil || err != nil {
		return result, err
	}
	if result, err := validateRequiredParam(params, "remote_name", "SET_UPSTREAM"); result != nil || err != nil {
		return result, err
	}

	name := getStringParam(params, "branch_name", "")
	remote := getStringParam(params, "remote_name", "")

	_, err := s.git.SetUpstream(name, remote)
	if err != nil {
		return jsonErrorResult("SET_UPSTREAM", err)
	}

	return mcpgo.NewToolResultText(writeResultJSON("BRANCH_SET_UPSTREAM", true, fmt.Sprintf("Upstream set for branch %s to %s", name, remote))), nil
}

func (s *Server) handleBranchUnsetUpstream(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := validateRequiredParam(params, "branch_name", "UNSET_UPSTREAM"); result != nil || err != nil {
		return result, err
	}

	name := getStringParam(params, "branch_name", "")

	_, err := s.git.UnsetUpstream(name)
	if err != nil {
		return jsonErrorResult("UNSET_UPSTREAM", err)
	}

	return mcpgo.NewToolResultText(writeResultJSON("BRANCH_UNSET_UPSTREAM", true, fmt.Sprintf("Upstream unset for branch %s", name))), nil
}
