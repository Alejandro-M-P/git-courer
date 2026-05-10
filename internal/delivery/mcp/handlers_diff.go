package mcp

import (
	"context"
	"strings"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleGitDiff(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(getStringParam(params, "command", "READ_DIFF"))

	// Specific parameters override generic 'arg'
	arg := getStringParam(params, "arg", "")
	if p := getStringParam(params, "path", ""); p != "" {
		arg = p
	} else if h := getStringParam(params, "hash", ""); h != "" {
		arg = h
	}

	limit, offset := parsePagination(params)
	filter := getStringParam(params, "filter", "")
	compact := false
	if v, ok := params["compact"].(bool); ok {
		compact = v
	}

	if limit <= 0 {
		limit = 200
	}

	var result string
	var err error

	switch command {
	case "READ_DIFF":
		result, err = s.handleDiffCommand(arg, limit, offset, "", filter, compact)

	case "READ_DIFF_STATS", "READ_DIFF_STAT":
		result, err = s.handleDiffStatCommand(arg)

	case "READ_DIFF_STAGED":
		result, err = s.handleDiffCommand(arg, limit, offset, "--cached", filter, compact)

	case "READ_DIFF_ALL":
		result, err = s.handleDiffAllCommand(arg, limit, offset, filter, compact)

	case "STASH_DIFF":
		raw, err := s.git.StashDiff(arg)
		if err != nil {
			return jsonErrorResult(command, err)
		}
		res := SanitizeDiff(raw, offset, limit)
		result = diffResultJSON(res)

	default:
		// Fallback to READ_DIFF
		result, err = s.handleDiffCommand(arg, limit, offset, "", filter, compact)
	}

	if err != nil {
		s.sendErrorNotification("git_diff", command+" failed", map[string]any{"error": err.Error()})
		return jsonErrorResult(command, err)
	}

	s.sendSuccessNotification("git_diff", command+" completed", nil)
	return mcpgo.NewToolResultText(result), nil
}
