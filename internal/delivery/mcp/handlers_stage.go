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

	// Validate known params — no 'arg'
	if result, err := validateKnownParams(params, []string{"command", "paths", "commit"}); result != nil || err != nil {
		return result, err
	}

	// Validate command before any side effects
	validCommands := []string{"ADD", "RM", "RESET_SOFT"}
	valid := false
	for _, c := range validCommands {
		if command == c {
			valid = true
			break
		}
	}
	if !valid {
		hint := suggestCommand(command, validCommands)
		if hint != "" {
			return jsonErrorResult("git_stage", fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return jsonErrorResult("git_stage", fmt.Errorf("unknown command: %s", command))
	}

	// Validate required params per command before any side effects
	switch command {
	case "ADD", "RM":
		if result, err := validateRequiredParam(params, "paths", command); result != nil || err != nil {
			return result, err
		}
	case "RESET_SOFT":
		if result, err := validateRequiredParam(params, "commit", command); result != nil || err != nil {
			return result, err
		}
	}

	paths := getStringParam(params, "paths", "")
	commit := getStringParam(params, "commit", "")

	backup, bErr := s.git.CreateBackup(command, domain.StashNone)
	if bErr == nil {
		s.lastBackup = backup
	}

	var err error
	var result string

	switch command {
	case "ADD":
		pathList := git.SplitPaths(paths)
		err = s.git.Add(pathList)
		result = writeResultJSON("ADD", err == nil, fmt.Sprintf("%d files staged", len(pathList)))
	case "RM":
		pathList := git.SplitPaths(paths)
		err = s.git.Remove(pathList)
		result = writeResultJSON("RM", err == nil, fmt.Sprintf("%d files removed", len(pathList)))
	case "RESET_SOFT":
		err = s.git.ResetSoft(commit)
		result = writeResultJSON("RESET_SOFT", err == nil, fmt.Sprintf("Soft reset to %s", commit))
	}

	if err != nil {
		s.sendErrorNotification("git_stage", command+" failed", map[string]any{"error": err.Error()})
		return jsonErrorResult(command, err)
	}

	s.sendSuccessNotification("git_stage", command+" completed", nil)
	return mcpgo.NewToolResultText(result), nil
}
