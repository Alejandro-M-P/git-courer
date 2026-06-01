package branching

import (
	"context"
	"fmt"
	"strings"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (h *Handler) HandleCherryPick(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	if result, err := shared.ValidateRequiredParam(params, "target_commit", "CHERRY_PICK"); result != nil || err != nil {
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
			return mcpgo.NewToolResultText(shared.ConflictResultJSON(conflictFiles, "Resolve conflicts then use the stage tool and the commit tool, or use git cherry-pick --abort")), nil
		}

		if bErr == nil {
			_ = h.git.RestoreBackup(backup)
			h.git.DeleteBackup(backup)
		}
		return shared.JSONErrorResult("CHERRY_PICK", err)
	}

	if bErr == nil {
		h.git.DeleteBackup(backup)
	}

	return mcpgo.NewToolResultText(shared.WriteResultJSON("CHERRY_PICK", true, fmt.Sprintf("Cherry-picked %s", commit))), nil
}
