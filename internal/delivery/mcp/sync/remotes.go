package sync

import (
	"context"
	"fmt"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (h *Handler) HandleRemotes(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(shared.GetStringParam(params, "command", ""))

	confirmed := false
	if v, ok := params["confirmed"].(bool); ok {
		confirmed = v
	}

	validCommands := []string{"ADD", "REMOVE"}
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
			return shared.JSONErrorResult("remotes", fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return shared.JSONErrorResult("remotes", fmt.Errorf("unknown command: %s", command))
	}

	remote := shared.GetStringParam(params, "remote_name", "")
	if remote == "" {
		return shared.JSONErrorResult(command, fmt.Errorf("remote_name is required"))
	}

	if command == "REMOVE" {
		if result, err := shared.CheckSafetyGate("remove_remote", false, confirmed); result != nil || err != nil {
			return result, err
		}
	}

	var err error

	_, _ = h.git.CreateBackup(command, domain.StashNone)

	switch command {
	case "ADD":
		url := shared.GetStringParam(params, "url", "")
		if url == "" {
			return shared.JSONErrorResult(command, fmt.Errorf("url is required for ADD"))
		}
		_, err = h.git.RemoteAdd(remote, url)
		if err == nil {
			return mcpgo.NewToolResultText(shared.WriteResultJSON("ADD_REMOTE", true, fmt.Sprintf("Added remote %s: %s", remote, url))), nil
		}
	case "REMOVE":
		_, err = h.git.RemoteRemove(remote)
		if err == nil {
			return mcpgo.NewToolResultText(shared.WriteResultJSON("REMOVE_REMOTE", true, fmt.Sprintf("Removed remote %s", remote))), nil
		}
	}

	return shared.JSONErrorResult(command, err)
}
