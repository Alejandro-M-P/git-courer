package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleGitStash(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	// Validate that all params are known for this tool
	if result, err := validateKnownParams(params, []string{"command", "message", "index"}); result != nil || err != nil {
		return result, err
	}

	command := getStringParam(params, "command", "")
	if command == "" {
		return jsonErrorResult("git_stash", fmt.Errorf("command is required for git_stash"))
	}

	validStashCommands := []string{"SAVE", "POP", "APPLY", "DROP", "CLEAR", "SHOW"}

	// Validate command before creating backup
	switch command {
	case "SAVE", "POP", "APPLY", "DROP", "CLEAR":
		// These commands modify state — create backup
		backup, bErr := s.git.CreateBackup(command, domain.StashNone)
		if bErr == nil {
			s.lastBackup = backup
		}
	case "SHOW":
		// SHOW is read-only — no backup needed
	default:
		hint := suggestCommand(command, validStashCommands)
		if hint != "" {
			return jsonErrorResult(command, fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return jsonErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}

	switch command {
	case "SAVE":
		return s.handleStashSave(params)
	case "POP":
		return s.handleStashPop(params)
	case "APPLY":
		return s.handleStashApply(params)
	case "DROP":
		return s.handleStashDrop(params)
	case "CLEAR":
		return s.handleStashClear()
	case "SHOW":
		return s.handleStashShow()
	default:
		return jsonErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}
}

func (s *Server) handleStashSave(params map[string]any) (*mcpgo.CallToolResult, error) {
	message := getStringParam(params, "message", "")
	var err error
	if message != "" {
		_, err = s.git.Stash(message)
	} else {
		_, err = s.git.Stash()
	}
	if err != nil {
		// Stash can fail if there's nothing to stash
		s.sendErrorNotification("git_stash", "SAVE failed", map[string]any{"error": err.Error()})
		return jsonErrorResult("SAVE", err)
	}
	s.sendSuccessNotification("git_stash", "SAVE completed", nil)
	return mcpgo.NewToolResultText(writeResultJSON("STASH_SAVE", true, "Changes stashed")), nil
}

func (s *Server) handleStashPop(params map[string]any) (*mcpgo.CallToolResult, error) {
	_, err := s.git.StashPop()
	if err != nil {
		if strings.Contains(err.Error(), "STASH_POP_UNTRACKED:") {
			return mcpgo.NewToolResultText(`{"error":"Stash pop failed: untracked files conflict","hint":"Use 'SAVE' command with -u flag to include untracked files next time"}`), nil
		}
		s.sendErrorNotification("git_stash", "POP failed", map[string]any{"error": err.Error()})
		return jsonErrorResult("POP", err)
	}
	s.sendSuccessNotification("git_stash", "POP completed", nil)
	return mcpgo.NewToolResultText(writeResultJSON("STASH_POP", true, "Stash restored")), nil
}

func (s *Server) handleStashApply(params map[string]any) (*mcpgo.CallToolResult, error) {
	index := getStringParam(params, "index", "")
	_, err := s.git.StashApply(index)
	if err != nil {
		s.sendErrorNotification("git_stash", "APPLY failed", map[string]any{"error": err.Error()})
		return jsonErrorResult("APPLY", err)
	}
	s.sendSuccessNotification("git_stash", "APPLY completed", nil)
	return mcpgo.NewToolResultText(writeResultJSON("STASH_APPLY", true, "Stash applied (kept in stash list)")), nil
}

func (s *Server) handleStashDrop(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := validateRequiredParam(params, "index", "DROP"); result != nil || err != nil {
		return result, err
	}

	index := getStringParam(params, "index", "")
	_, err := s.git.StashDrop(index)
	if err != nil {
		s.sendErrorNotification("git_stash", "DROP failed", map[string]any{"error": err.Error()})
		return jsonErrorResult("DROP", err)
	}
	s.sendSuccessNotification("git_stash", "DROP completed", nil)
	return mcpgo.NewToolResultText(writeResultJSON("STASH_DROP", true, "Stash entry dropped")), nil
}

func (s *Server) handleStashClear() (*mcpgo.CallToolResult, error) {
	_, err := s.git.StashClear()
	if err != nil {
		s.sendErrorNotification("git_stash", "CLEAR failed", map[string]any{"error": err.Error()})
		return jsonErrorResult("CLEAR", err)
	}
	s.sendSuccessNotification("git_stash", "CLEAR completed", nil)
	return mcpgo.NewToolResultText(writeResultJSON("STASH_CLEAR", true, "All stash entries cleared")), nil
}

func (s *Server) handleStashShow() (*mcpgo.CallToolResult, error) {
	output, err := s.git.StashShow()
	if err != nil {
		return jsonErrorResult("SHOW", err)
	}
	return mcpgo.NewToolResultText(output), nil
}