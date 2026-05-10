package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleGitStatus(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(getStringParam(params, "command", "READ_STATUS"))

	limit, offset := parsePagination(params)
	filter := getStringParam(params, "filter", "")
	arg := getStringParam(params, "arg", "")

	if limit <= 0 {
		limit = 100
	}

	var result string
	var err error

	switch command {
	case "READ_STATUS":
		status, err := s.git.Status()
		if err != nil {
			return jsonErrorResult(command, err)
		}
		result = formatStatusJSON(status, limit, offset, filter)

	case "CURRENT_BRANCH":
		branch, err := s.git.CurrentBranch()
		if err != nil {
			return jsonErrorResult(command, err)
		}
		result = fmt.Sprintf(`{"current_branch":%q}`, branch)

	case "IS_REPO":
		isRepo := s.git.IsRepo()
		result = fmt.Sprintf(`{"is_repo":%v}`, isRepo)

	case "REMOTE_INFO":
		info, err := s.git.RemoteInfo()
		if err != nil {
			return jsonErrorResult(command, err)
		}
		result = fmt.Sprintf(`{"remote_info":%q}`, info)

	case "WHAT_CHANGED":
		filterMode := WhatChangedFilter(getStringParam(params, "filter", "all"))
		useLLM := true
		if v, ok := params["llm"].(bool); ok {
			useLLM = v
		}
		result, err = s.handleWhatChangedCommand(arg, filterMode, useLLM)

	default:
		// Fallback to READ_STATUS if no explicit command or unknown
		status, err := s.git.Status()
		if err != nil {
			return jsonErrorResult("READ_STATUS", err)
		}
		result = formatStatusJSON(status, limit, offset, filter)
	}

	if err != nil {
		s.sendErrorNotification("git_status", command+" failed", map[string]any{"error": err.Error()})
		return jsonErrorResult(command, err)
	}

	s.sendSuccessNotification("git_status", command+" completed", nil)
	return mcpgo.NewToolResultText(result), nil
}
