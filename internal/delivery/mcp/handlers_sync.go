package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleGitSync(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(getStringParam(params, "command", ""))

	arg := getStringParam(params, "arg", "")
	remote := getStringParam(params, "remote", "origin")
	branch := getStringParam(params, "branch", "")

	var err error
	var result string

	// FETCH, PULL, PUSH, MERGE, SWITCH don't stash by default in this implementation's logic
	backup, bErr := s.git.CreateBackup(command, domain.StashNone)
	if bErr == nil {
		s.lastBackup = backup
	}

	switch command {
	case "FETCH":
		_, err = s.git.Fetch()
		result = writeResultJSON("FETCH", err == nil, "Fetched from remote")
	case "PULL":
		if arg != "" {
			remote = arg
		}
		_, err = s.git.PullFrom(remote)
		if err != nil && strings.Contains(err.Error(), "NO_UPSTREAM") {
			result = `{"error":"No upstream configured","hint":"Push first or specify remote/branch"}`
			err = nil
			break
		}
		result = writeResultJSON("PULL", err == nil, "Pulled from "+remote)
	case "PUSH":
		if branch == "" {
			_, err = s.git.PushTo(remote)
		} else {
			_, err = s.git.PushTo(remote) // Current implementation of PushTo handles remote
		}
		result = writeResultJSON("PUSH", err == nil, "Pushed to "+remote)
	case "MERGE":
		_, err = s.git.Merge(arg)
		result = writeResultJSON("MERGE", err == nil, fmt.Sprintf("Merged %s", arg))
	case "SWITCH":
		err = s.git.Switch(arg)
		result = writeResultJSON("SWITCH", err == nil, fmt.Sprintf("Switched to %s", arg))
	default:
		return jsonErrorResult("git_sync", fmt.Errorf("unknown command: %s", command))
	}

	if err != nil {
		s.sendErrorNotification("git_sync", command+" failed", map[string]any{"error": err.Error()})
		return jsonErrorResult(command, err)
	}
	s.sendSuccessNotification("git_sync", command+" completed", nil)
	return mcpgo.NewToolResultText(result), nil
}
