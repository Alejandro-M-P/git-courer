package integrate

import (
	"context"
	"fmt"
	"strings"

	"github.com/blak0p/git-courer/internal/core/ports"
	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

type Handler struct {
	git ports.Git
}

func NewHandler(git ports.Git) *Handler {
	return &Handler{git: git}
}

func (h *Handler) HandleIntegrate(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(shared.GetStringParam(params, "command", ""))

	validCommands := []string{"MERGE", "UPDATE", "PICK", "CONTINUE", "ABORT"}
	valid := false
	for _, c := range validCommands {
		if command == c {
			valid = true
			break
		}
	}
	if !valid {
		hint := shared.SuggestCommand(command, validCommands)
		if hint != "" {
			return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}

	switch command {
	case "MERGE":
		return h.handleMerge(params)
	case "UPDATE":
		return h.handleRebase(params)
	case "PICK":
		return h.handleCherryPick(params)
	case "CONTINUE":
		return h.handleContinue(params)
	case "ABORT":
		return h.handleAbort(params)
	default:
		return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}
}
