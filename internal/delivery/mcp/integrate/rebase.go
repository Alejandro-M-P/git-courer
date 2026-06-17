package integrate

import (
	"fmt"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (h *Handler) handleRebase(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := shared.ValidateRequiredParam(params, "branch_name", "UPDATE"); result != nil || err != nil {
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
				return mcpgo.NewToolResultText(shared.ConflictResultJSON(conflictFiles, "Resolve conflicts, then stage files and call integrate CONTINUE")), nil
			}
			return shared.JSONErrorResult("UPDATE", err)
		}
		return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("REBASE_ONTO", true, fmt.Sprintf("Rebased %s onto %s", branch, onto), "consider calling diff to verify the rebase result, then pr-review before pushing")), nil
	}

	backup, bErr := h.git.CreateBackup("REBASE", domain.StashNone)

	_, err := h.git.Rebase(branch)
	if err != nil {
		if strings.Contains(err.Error(), "conflict") || strings.Contains(strings.ToLower(err.Error()), "merge conflict") {
			conflictFiles := h.getConflictedFiles()
			if bErr == nil {
				h.git.DeleteBackup(backup)
			}
			return mcpgo.NewToolResultText(shared.ConflictResultJSON(conflictFiles, "Resolve conflicts, then stage files and call integrate CONTINUE")), nil
		}

		if bErr == nil {
			_ = h.git.RestoreBackup(backup)
			h.git.DeleteBackup(backup)
		}
		return shared.JSONErrorResult("UPDATE", err)
	}

	if bErr == nil {
		h.git.DeleteBackup(backup)
	}

	return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("REBASE", true, fmt.Sprintf("Rebased onto %s", branch), "consider calling diff to verify the rebase result, then pr-review before pushing")), nil
}
