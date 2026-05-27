package branching

import (
	"context"
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (h *Handler) HandleRebase(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	abort := false
	if v, ok := params["abort"].(bool); ok {
		abort = v
	}
	continueRebase := false
	if v, ok := params["continue"].(bool); ok {
		continueRebase = v
	}
	skip := false
	if v, ok := params["skip"].(bool); ok {
		skip = v
	}

	if abort {
		_, err := h.git.RebaseAbort()
		return mcpgo.NewToolResultText(shared.WriteResultJSON("REBASE_ABORT", err == nil, "Rebase aborted")), nil
	}

	if continueRebase {
		_, err := h.git.RebaseContinue()
		if err != nil {
			if strings.Contains(err.Error(), "conflict") || strings.Contains(strings.ToLower(err.Error()), "merge conflict") {
				conflictFiles := h.getConflictedFiles()
				return mcpgo.NewToolResultText(shared.ConflictResultJSON(conflictFiles, "Conflicts still remain. Resolve, stage, and continue again.")), nil
			}
			return shared.JSONErrorResult("REBASE_CONTINUE", err)
		}
		return mcpgo.NewToolResultText(shared.WriteResultJSON("REBASE_CONTINUE", true, "Rebase continued successfully")), nil
	}

	if skip {
		out, err := h.git.RebaseSkip()
		if err != nil {
			return shared.JSONErrorResult("REBASE_SKIP", err)
		}
		return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("REBASE_SKIP", true, out, "rebase skip completed")), nil
	}

	if result, err := shared.ValidateRequiredParam(params, "branch_name", "REBASE"); result != nil || err != nil {
		return result, err
	}
	branch := shared.GetStringParam(params, "branch_name", "")

	// Check for --onto parameter
	onto := shared.GetStringParam(params, "onto", "")
	if onto != "" {
		_, err := h.git.RebaseOnto(onto, branch, "")
		if err != nil {
			if strings.Contains(err.Error(), "conflict") || strings.Contains(strings.ToLower(err.Error()), "merge conflict") {
				conflictFiles := h.getConflictedFiles()
				return mcpgo.NewToolResultText(shared.ConflictResultJSON(conflictFiles, "Resolve conflicts, then stage files and call rebase continue=true")), nil
			}
			return shared.JSONErrorResult("REBASE_ONTO", err)
		}
		return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("REBASE_ONTO", true, fmt.Sprintf("Rebased %s onto %s", branch, onto), "consider calling diff to verify the rebase result, then pr-review before pushing")), nil
	}

	backup, bErr := h.git.CreateBackup("REBASE", domain.StashNone)

	_, err := h.git.Rebase(branch)
	if err != nil {
		if strings.Contains(err.Error(), "conflict") || strings.Contains(strings.ToLower(err.Error()), "merge conflict") {
			conflictFiles := h.getConflictedFiles()
			if bErr == nil {
				h.git.DeleteBackup(backup) // Don't restore on conflict
			}
			return mcpgo.NewToolResultText(shared.ConflictResultJSON(conflictFiles, "Resolve conflicts, then stage files and call rebase continue=true")), nil
		}

		if bErr == nil {
			_ = h.git.RestoreBackup(backup)
			h.git.DeleteBackup(backup)
		}
		return shared.JSONErrorResult("REBASE", err)
	}

	if bErr == nil {
		h.git.DeleteBackup(backup)
	}

	return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("REBASE", true, fmt.Sprintf("Rebased onto %s", branch), "consider calling diff to verify the rebase result, then pr-review before pushing")), nil
}
