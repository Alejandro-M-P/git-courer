package branching

import (
	"context"
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (h *Handler) HandleMerge(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	abort := false
	if v, ok := params["abort"].(bool); ok {
		abort = v
	}
	continueMerge := false
	if v, ok := params["continue"].(bool); ok {
		continueMerge = v
	}

	if abort {
		_, err := h.git.MergeAbort()
		return mcpgo.NewToolResultText(shared.WriteResultJSON("MERGE_ABORT", err == nil, "Merge aborted")), nil
	}

	if continueMerge {
		// git merge --continue is handled by committing the resolved files
		return mcpgo.NewToolResultText(shared.WriteResultJSON("MERGE_CONTINUE", false, "Use the commit tool to finish the merge after resolving conflicts.")), nil
	}

	if result, err := shared.ValidateRequiredParam(params, "branch_name", "MERGE"); result != nil || err != nil {
		return result, err
	}
	branch := shared.GetStringParam(params, "branch_name", "")

	backup, bErr := h.git.CreateBackup("MERGE", domain.StashNone)

	_, err := h.git.Merge(branch)
	if err != nil {
		if strings.Contains(err.Error(), "conflict") || strings.Contains(strings.ToLower(err.Error()), "merge conflict") {
			conflictFiles := h.getConflictedFiles()
			if bErr == nil {
				h.git.DeleteBackup(backup) // We don't restore automatically on conflict, user must resolve or abort
			}
			return mcpgo.NewToolResultText(shared.ConflictResultJSON(conflictFiles, "Resolve conflicts then use the stage tool and the commit tool, or call merge with abort=true")), nil
		}

		if bErr == nil {
			_ = h.git.RestoreBackup(backup)
			h.git.DeleteBackup(backup)
		}
		return shared.JSONErrorResult("MERGE", err)
	}

	if bErr == nil {
		h.git.DeleteBackup(backup)
	}

	return mcpgo.NewToolResultText(shared.WriteResultJSON("MERGE", true, fmt.Sprintf("Merged %s", branch))), nil
}

// getConflictedFiles is a helper to get files with conflicts
func (h *Handler) getConflictedFiles() []string {
	status, err := h.git.Status()
	if err != nil {
		return nil
	}
	var files []string
	for _, f := range status.Files {
		if f.Status == "UU" || f.Status == "AA" || f.Status == "DD" || f.Status == "AU" || f.Status == "UA" || f.Status == "DU" || f.Status == "UD" {
			files = append(files, f.Path)
		}
	}
	return files
}
