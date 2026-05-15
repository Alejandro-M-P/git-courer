package branching

import (
	"context"
	"fmt"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (h *Handler) HandleTag(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	// Validate that all params are known for this tool
	if result, err := shared.ValidateKnownParams(params, []string{"command", "tag_name", "commit_message", "confirmed"}); result != nil || err != nil {
		return result, err
	}

	command := shared.GetStringParam(params, "command", "")
	if command == "" {
		return shared.JSONErrorResult("tag", fmt.Errorf("command is required for tag"))
	}

	// Extract safety params early
	confirmed := false
	if v, ok := params["confirmed"].(bool); ok {
		confirmed = v
	}

	validTagCommands := []string{"CREATE", "DELETE", "PUSH", "DELETE_REMOTE"}

	switch command {
	case "CREATE", "DELETE", "PUSH", "DELETE_REMOTE":
		// Safety gate for destructive tag commands
		if command == "DELETE_REMOTE" {
			if result, err := shared.CheckSafetyGate("delete_remote", false, confirmed); result != nil || err != nil {
				return result, err
			}
		}
		// All tag commands modify state — create backup
		_, _ = h.git.CreateBackup(command, domain.StashNone)
	default:
		hint := shared.SuggestCommand(command, validTagCommands)
		if hint != "" {
			return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}

	switch command {
	case "CREATE":
		return h.handleTagCreate(params)
	case "DELETE":
		return h.handleTagDelete(params)
	case "PUSH":
		return h.handleTagPush(params)
	case "DELETE_REMOTE":
		return h.handleTagDeleteRemote(params)
	default:
		return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}
}

func (h *Handler) handleTagCreate(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := shared.ValidateRequiredParam(params, "tag_name", "CREATE"); result != nil || err != nil {
		return result, err
	}

	name := shared.GetStringParam(params, "tag_name", "")
	message := shared.GetStringParam(params, "commit_message", "")

	_, err := h.git.Tag(name, message)
	if err != nil {
		return shared.JSONErrorResult("TAG_CREATE", err)
	}

	msg := "created"
	if message != "" {
		msg = "created (annotated)"
	}
	return mcpgo.NewToolResultText(shared.TagResultJSON(msg, name)), nil
}

func (h *Handler) handleTagDelete(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := shared.ValidateRequiredParam(params, "tag_name", "DELETE"); result != nil || err != nil {
		return result, err
	}

	name := shared.GetStringParam(params, "tag_name", "")
	_, err := h.git.DeleteTag(name)
	if err != nil {
		return shared.JSONErrorResult("TAG_DELETE", err)
	}

	return mcpgo.NewToolResultText(shared.TagResultJSON("deleted", name)), nil
}

func (h *Handler) handleTagPush(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := shared.ValidateRequiredParam(params, "tag_name", "PUSH"); result != nil || err != nil {
		return result, err
	}

	name := shared.GetStringParam(params, "tag_name", "")
	_, err := h.git.PushTag(name)
	if err != nil {
		return shared.JSONErrorResult("TAG_PUSH", err)
	}

	return mcpgo.NewToolResultText(shared.TagResultJSON("pushed", name)), nil
}

func (h *Handler) handleTagDeleteRemote(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := shared.ValidateRequiredParam(params, "tag_name", "DELETE_REMOTE"); result != nil || err != nil {
		return result, err
	}

	name := shared.GetStringParam(params, "tag_name", "")
	_, err := h.git.DeleteTagRemote(name)
	if err != nil {
		return shared.JSONErrorResult("TAG_DELETE_REMOTE", err)
	}

	return mcpgo.NewToolResultText(shared.TagResultJSON("deleted from remote", name)), nil
}
