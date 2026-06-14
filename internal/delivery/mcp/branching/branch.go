package branching

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (h *Handler) HandleBranch(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	// First validate that all params are known for this tool
	if result, err := shared.ValidateKnownParams(params, []string{"command", "branch_name", "new_branch_name", "remote_name", "force", "confirmed", "switch", "filter"}); result != nil || err != nil {
		return result, err
	}

	command := shared.GetStringParam(params, "command", "")
	if command == "" {
		return shared.JSONErrorResult("branch", fmt.Errorf("command is required for branch"))
	}

	// Extract safety params early
	confirmed := false
	if v, ok := params["confirmed"].(bool); ok {
		confirmed = v
	}

	validBranchCommands := []string{"CREATE", "DELETE", "RENAME", "REMOTE_DELETE", "SET_UPSTREAM", "UNSET_UPSTREAM", "SWITCH", "LIST"}

	switch command {
	case "CREATE", "DELETE", "RENAME", "REMOTE_DELETE", "SET_UPSTREAM", "UNSET_UPSTREAM", "SWITCH":
		// Safety gate for destructive branch commands
		switch command {
		case "DELETE":
			if result, err := shared.CheckSafetyGate("branch_delete", false, confirmed); result != nil || err != nil {
				return result, err
			}
		case "REMOTE_DELETE":
			if result, err := shared.CheckSafetyGate("remote_delete", false, confirmed); result != nil || err != nil {
				return result, err
			}
		}
		// All branch commands modify state — create backup
		_, _ = h.git.CreateBackup(command, domain.StashNone)
	case "LIST":
		// read-only bypasses safety checks and backup creation
	default:
		// Unknown command — provide helpful suggestion
		hint := shared.SuggestCommand(command, validBranchCommands)
		if hint != "" {
			return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}

	switch command {
	case "CREATE":
		return h.handleBranchCreate(params)
	case "DELETE":
		return h.handleBranchDelete(params)
	case "RENAME":
		return h.handleBranchRename(params)
	case "REMOTE_DELETE":
		return h.handleBranchRemoteDelete(params)
	case "SET_UPSTREAM":
		return h.handleBranchSetUpstream(params)
	case "UNSET_UPSTREAM":
		return h.handleBranchUnsetUpstream(params)
	case "SWITCH":
		return h.handleBranchSwitch(params)
	case "LIST":
		return h.handleBranchList(params)
	default:
		// Unreachable — already validated above
		return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}
}

func (h *Handler) handleBranchCreate(params map[string]any) (*mcpgo.CallToolResult, error) {
	// Validate required parameter
	if result, err := shared.ValidateRequiredParam(params, "branch_name", "CREATE"); result != nil || err != nil {
		return result, err
	}

	name := shared.GetStringParam(params, "branch_name", "")

	switchOn := false
	if v, ok := params["switch"].(bool); ok {
		switchOn = v
	}

	var stashed bool
	if switchOn {
		status, err := h.git.Status()
		if err != nil {
			return shared.JSONErrorResult("CREATE", err)
		}
		if !status.IsClean {
			if _, err := h.git.Stash(); err != nil {
				return shared.JSONErrorResult("CREATE", err)
			}
			stashed = true
		}
	}

	if _, err := h.git.Branch(name); err != nil {
		return shared.JSONErrorResult("CREATE", err)
	}

	if switchOn {
		if err := h.git.Switch(name); err != nil {
			return shared.JSONErrorResult("CREATE", err)
		}
		if stashed {
			if _, err := h.git.StashPop(); err != nil {
				return shared.JSONErrorResult("CREATE", err)
			}
		}
		return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("BRANCH_CREATE", true, fmt.Sprintf("Created and switched to branch: %s", name), "consider calling status to check the new branch state")), nil
	}

	return mcpgo.NewToolResultText(shared.WriteResultJSON("BRANCH_CREATE", true, fmt.Sprintf("Created branch: %s", name))), nil
}

func (h *Handler) handleBranchDelete(params map[string]any) (*mcpgo.CallToolResult, error) {
	// Validate required parameter
	if result, err := shared.ValidateRequiredParam(params, "branch_name", "DELETE"); result != nil || err != nil {
		return result, err
	}

	name := shared.GetStringParam(params, "branch_name", "")
	force := false
	if v, ok := params["force"].(bool); ok {
		force = v
	}

	_, err := h.git.DeleteBranch(name, force)
	if err != nil {
		return shared.JSONErrorResult("DELETE", err)
	}

	return mcpgo.NewToolResultText(shared.WriteResultJSON("BRANCH_DELETE", true, fmt.Sprintf("Deleted branch %s", name))), nil
}

func (h *Handler) handleBranchRename(params map[string]any) (*mcpgo.CallToolResult, error) {
	// Validate required parameters
	if result, err := shared.ValidateRequiredParam(params, "branch_name", "RENAME"); result != nil || err != nil {
		return result, err
	}
	if result, err := shared.ValidateRequiredParam(params, "new_branch_name", "RENAME"); result != nil || err != nil {
		return result, err
	}

	name := shared.GetStringParam(params, "branch_name", "")
	newName := shared.GetStringParam(params, "new_branch_name", "")

	_, err := h.git.RenameBranch(name, newName)
	if err != nil {
		return shared.JSONErrorResult("RENAME", err)
	}

	return mcpgo.NewToolResultText(shared.WriteResultJSON("BRANCH_RENAME", true, fmt.Sprintf("Renamed branch from %s to %s", name, newName))), nil
}

func (h *Handler) handleBranchRemoteDelete(params map[string]any) (*mcpgo.CallToolResult, error) {
	// Validate required parameter
	if result, err := shared.ValidateRequiredParam(params, "branch_name", "REMOTE_DELETE"); result != nil || err != nil {
		return result, err
	}

	name := shared.GetStringParam(params, "branch_name", "")

	err := h.git.DeleteRemoteBranch(name)
	if err != nil {
		return shared.JSONErrorResult("REMOTE_DELETE", err)
	}

	return mcpgo.NewToolResultText(shared.WriteResultJSON("BRANCH_REMOTE_DELETE", true, fmt.Sprintf("Deleted remote branch %s", name))), nil
}

func (h *Handler) handleBranchSetUpstream(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := shared.ValidateRequiredParam(params, "branch_name", "SET_UPSTREAM"); result != nil || err != nil {
		return result, err
	}
	if result, err := shared.ValidateRequiredParam(params, "remote_name", "SET_UPSTREAM"); result != nil || err != nil {
		return result, err
	}

	name := shared.GetStringParam(params, "branch_name", "")
	remote := shared.GetStringParam(params, "remote_name", "")

	_, err := h.git.SetUpstream(name, remote)
	if err != nil {
		return shared.JSONErrorResult("SET_UPSTREAM", err)
	}

	return mcpgo.NewToolResultText(shared.WriteResultJSON("BRANCH_SET_UPSTREAM", true, fmt.Sprintf("Upstream set for branch %s to %s", name, remote))), nil
}

func (h *Handler) handleBranchUnsetUpstream(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := shared.ValidateRequiredParam(params, "branch_name", "UNSET_UPSTREAM"); result != nil || err != nil {
		return result, err
	}

	name := shared.GetStringParam(params, "branch_name", "")

	_, err := h.git.UnsetUpstream(name)
	if err != nil {
		return shared.JSONErrorResult("UNSET_UPSTREAM", err)
	}

	return mcpgo.NewToolResultText(shared.WriteResultJSON("BRANCH_UNSET_UPSTREAM", true, fmt.Sprintf("Upstream unset for branch %s", name))), nil
}

func (h *Handler) handleBranchSwitch(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := shared.ValidateRequiredParam(params, "branch_name", "SWITCH"); result != nil || err != nil {
		return result, err
	}

	name := shared.GetStringParam(params, "branch_name", "")

	err := h.git.Switch(name)
	if err != nil {
		return shared.JSONErrorResult("SWITCH", err)
	}

	return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("SWITCH", true, fmt.Sprintf("Switched to branch %s", name), "consider calling status to check the new branch state")), nil
}

func (h *Handler) handleBranchList(params map[string]any) (*mcpgo.CallToolResult, error) {
	out, err := h.git.ListBranches()
	if err != nil {
		return shared.JSONErrorResult("LIST", err)
	}

	filter := shared.GetStringParam(params, "filter", "ALL")
	branchPattern := shared.GetStringParam(params, "branch_name", "")

	rawLines := strings.Split(out, "\n")
	var filtered []string

	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Clean active branch marker prefix if present (e.g. "* main" -> "main")
		cleaned := strings.TrimPrefix(line, "*")
		cleaned = strings.TrimSpace(cleaned)
		if cleaned == "" {
			continue
		}

		isRemote := strings.HasPrefix(cleaned, "remotes/")

		// Filter by location
		switch filter {
		case "LOCAL":
			if isRemote {
				continue
			}
		case "REMOTE":
			if !isRemote {
				continue
			}
		}

		// Filter by name pattern (if provided)
		if branchPattern != "" {
			if !matchBranchPattern(branchPattern, cleaned) {
				continue
			}
		}

		filtered = append(filtered, cleaned)
	}

	resultMsg := strings.Join(filtered, "\n")
	return mcpgo.NewToolResultText(shared.WriteResultJSON("BRANCH_LIST", true, resultMsg)), nil
}

func matchBranchPattern(pattern, name string) bool {
	// Try standard filepath.Match first in case it works (for simple non-slash globs)
	if matched, err := filepath.Match(pattern, name); err == nil && matched {
		return true
	}

	// Convert glob pattern to a regex to allow wildcards to cross slashes/separators
	var sb strings.Builder
	// If the pattern doesn't start/end with wildcards but contains no wildcards at all,
	// we fall back to strings.Contains.
	// But let's build the full regex:
	sb.WriteString("^")
	hasWildcard := false
	for _, r := range pattern {
		switch r {
		case '*':
			sb.WriteString(".*")
			hasWildcard = true
		case '?':
			sb.WriteString(".")
			hasWildcard = true
		case '\\', '.', '+', '(', ')', '[', ']', '{', '}', '^', '$', '|':
			sb.WriteString("\\")
			sb.WriteRune(r)
		default:
			sb.WriteRune(r)
		}
	}
	sb.WriteString("$")

	if !hasWildcard {
		return strings.Contains(name, pattern)
	}

	reg, err := regexp.Compile(sb.String())
	if err != nil {
		return strings.Contains(name, pattern)
	}
	return reg.MatchString(name)
}
