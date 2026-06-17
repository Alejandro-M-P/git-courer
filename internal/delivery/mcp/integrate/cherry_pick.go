package integrate

import (
	"fmt"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (h *Handler) handleCherryPick(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := shared.ValidateRequiredParam(params, "target_commit", "PICK"); result != nil || err != nil {
		return result, err
	}
	commit := shared.GetStringParam(params, "target_commit", "")

	backup, bErr := h.git.CreateBackup("CHERRY_PICK", domain.StashNone)

	_, err := h.git.CherryPick(commit)
	if err != nil {
		if strings.Contains(err.Error(), "conflict") || strings.Contains(strings.ToLower(err.Error()), "merge conflict") {
			conflictFiles := h.getConflictedFiles()
			if bErr == nil {
				h.git.DeleteBackup(backup)
			}
			return mcpgo.NewToolResultText(shared.ConflictResultJSON(conflictFiles, "Resolve conflicts then use the stage tool and the commit tool, or call integrate ABORT")), nil
		}

		if bErr == nil {
			_ = h.git.RestoreBackup(backup)
			h.git.DeleteBackup(backup)
		}
		return shared.JSONErrorResult("PICK", err)
	}

	if bErr == nil {
		h.git.DeleteBackup(backup)
	}

	return mcpgo.NewToolResultText(shared.WriteResultJSON("CHERRY_PICK", true, fmt.Sprintf("Cherry-picked %s", commit))), nil
}

func (h *Handler) handleContinue(params map[string]any) (*mcpgo.CallToolResult, error) {
	// Try merge continue first, then rebase continue
	_, err := h.git.MergeContinue()
	if err == nil {
		return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("CONTINUE", true, "merge conflict resolved and committed", "")), nil
	}

	_, err = h.git.RebaseContinue()
	if err != nil {
		if strings.Contains(err.Error(), "conflict") || strings.Contains(strings.ToLower(err.Error()), "merge conflict") {
			conflictFiles := h.getConflictedFiles()
			return mcpgo.NewToolResultText(shared.ConflictResultJSON(conflictFiles, "Conflicts still remain. Resolve, stage, and continue again.")), nil
		}
		return shared.JSONErrorResult("CONTINUE", err)
	}
	return mcpgo.NewToolResultText(shared.WriteResultJSON("CONTINUE", true, "Rebase continued successfully")), nil
}

func (h *Handler) handleAbort(params map[string]any) (*mcpgo.CallToolResult, error) {
	// Try merge abort first, then rebase abort
	_, err := h.git.MergeAbort()
	if err == nil {
		return mcpgo.NewToolResultText(shared.WriteResultJSON("ABORT", true, "Merge aborted")), nil
	}

	_, err = h.git.RebaseAbort()
	if err != nil {
		return shared.JSONErrorResult("ABORT", err)
	}
	return mcpgo.NewToolResultText(shared.WriteResultJSON("ABORT", true, "Rebase aborted")), nil
}
