package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleGitBackup(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	// Validate that all params are known for this tool
	if result, err := validateKnownParams(params, []string{"command", "days"}); result != nil || err != nil {
		return result, err
	}

	command := getStringParam(params, "command", "")
	if command == "" {
		return jsonErrorResult("git_backup", fmt.Errorf("command is required for git_backup"))
	}

	validBackupCommands := []string{"RESTORE", "UNDO", "LIST", "PRUNE"}

	switch command {
	case "UNDO", "RESTORE":
		return s.handleBackupUndo()
	case "LIST":
		return s.handleBackupList()
	case "PRUNE":
		return s.handleBackupPrune(params)
	default:
		hint := suggestCommand(command, validBackupCommands)
		if hint != "" {
			return jsonErrorResult(command, fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return jsonErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}
}

func (s *Server) handleBackupUndo() (*mcpgo.CallToolResult, error) {
	if s.lastBackup.Ref == "" {
		return jsonErrorResult("UNDO", fmt.Errorf("no operation to undo"))
	}
	err := s.git.RestoreBackup(s.lastBackup)
	if err != nil {
		return jsonErrorResult("UNDO", err)
	}
	msg := fmt.Sprintf("Successfully reverted last operation (%s)", s.lastBackup.Operation)
	s.lastBackup = domain.Backup{} // Clear after undo
	return mcpgo.NewToolResultText(writeResultJSON("UNDO", true, msg)), nil
}

func (s *Server) handleBackupList() (*mcpgo.CallToolResult, error) {
	backups, err := s.git.ListBackups()
	if err != nil {
		return jsonErrorResult("LIST", err)
	}
	// Set Undoable on each backup based on operationUndoability map
	for i := range backups {
		if undoable, ok := operationUndoability[strings.ToLower(backups[i].Operation)]; ok {
			backups[i].Undoable = undoable
		}
	}
	result := formatBackupListJSON(backups)
	return mcpgo.NewToolResultText(result), nil
}

func (s *Server) handleBackupPrune(params map[string]any) (*mcpgo.CallToolResult, error) {
	days := 7
	if v, ok := params["days"].(float64); ok {
		days = int(v)
	}
	err := s.git.PruneBackups(time.Duration(days) * 24 * time.Hour)
	if err != nil {
		return jsonErrorResult("PRUNE", err)
	}
	return mcpgo.NewToolResultText(writeResultJSON("PRUNE", true, fmt.Sprintf("Backups older than %d days deleted", days))), nil
}