package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleGitStage(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(getStringParam(params, "command", "ADD"))

	arg := getStringParam(params, "arg", "")
	if p := getStringParam(params, "paths", ""); p != "" {
		arg = p
	} else if c := getStringParam(params, "commit", ""); c != "" {
		arg = c
	}

	backup, bErr := s.git.CreateBackup(command, domain.StashNone)
	if bErr == nil {
		s.lastBackup = backup
	}

	var err error
	var result string

	switch command {
	case "ADD":
		paths := git.SplitPaths(arg)
		err = s.git.Add(paths)
		result = writeResultJSON("ADD", err == nil, fmt.Sprintf("%d files staged", len(paths)))
	case "RM":
		paths := git.SplitPaths(arg)
		err = s.git.Remove(paths)
		result = writeResultJSON("RM", err == nil, fmt.Sprintf("%d files removed", len(paths)))
	case "RESET_SOFT":
		err = s.git.ResetSoft(arg)
		result = writeResultJSON("RESET_SOFT", err == nil, fmt.Sprintf("Soft reset to %s", arg))
	default:
		return jsonErrorResult("git_stage", fmt.Errorf("unknown command: %s", command))
	}

	if err != nil {
		s.sendErrorNotification("git_stage", command+" failed", map[string]any{"error": err.Error()})
		return jsonErrorResult(command, err)
	}

	s.sendSuccessNotification("git_stage", command+" completed", nil)
	return mcpgo.NewToolResultText(result), nil
}
