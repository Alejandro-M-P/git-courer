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
	skip := false
	if v, ok := params["skip"].(bool); ok {
		skip = v
	}

	if abort {
		_, err := h.git.MergeAbort()
		return mcpgo.NewToolResultText(shared.WriteResultJSON("MERGE_ABORT", err == nil, "Merge aborted")), nil
	}

	if continueMerge {
		_, err := h.git.MergeContinue()
		if err != nil {
			return shared.JSONErrorResult("MERGE_CONTINUE", err)
		}
		return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("MERGE_CONTINUE", true, "merge conflict resolved and committed", "")), nil
	}

	if skip {
		_, err := h.git.MergeSkip()
		if err != nil {
			return shared.JSONErrorResult("MERGE_SKIP", err)
		}
		return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("MERGE_SKIP", true, "merge skip completed", "")), nil
	}

	if result, err := shared.ValidateRequiredParam(params, "branch_name", "MERGE"); result != nil || err != nil {
		return result, err
	}
	branch := shared.GetStringParam(params, "branch_name", "")

	// Composition flags
	deleteSource := false
	if v, ok := params["delete_source"].(bool); ok {
		deleteSource = v
	}
	pushAfter := false
	if v, ok := params["push_after"].(bool); ok {
		pushAfter = v
	}
	newBranch := shared.GetStringParam(params, "new_branch", "")

	backup, bErr := h.git.CreateBackup("MERGE", domain.StashNone)

	_, err := h.git.Merge(branch)
	if err != nil {
		if strings.Contains(err.Error(), "conflict") || strings.Contains(strings.ToLower(err.Error()), "merge conflict") {
			conflictFiles := h.getConflictedFiles()
			if bErr == nil {
				h.git.DeleteBackup(backup) // We don't restore automatically on conflict, user must resolve or abort
			}
			return mcpgo.NewToolResultText(shared.ConflictResultJSON(conflictFiles, "Resolve conflicts, then stage files and call merge continue=true")), nil
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

	// Post-merge composition logic
	msg := fmt.Sprintf("Merged %s", branch)

	if deleteSource {
		if _, dErr := h.git.DeleteBranch(branch, true); dErr != nil {
			msg = fmt.Sprintf("%s\n\n[WARNING] Failed to delete source branch %q: %v", msg, branch, dErr)
		} else {
			msg = fmt.Sprintf("%s\n\n[SUCCESS] Source branch %q deleted", msg, branch)
		}
	}

	if pushAfter {
		if pushOut, pErr := h.git.Push(); pErr != nil {
			msg = fmt.Sprintf("%s\n\n[WARNING] Push failed: %v", msg, pErr)
		} else {
			msg = fmt.Sprintf("%s\n\n[SUCCESS] Changes pushed to remote:\n%s", msg, pushOut)
		}
	}

	if newBranch != "" {
		if _, bErr := h.git.Branch(newBranch); bErr != nil {
			msg = fmt.Sprintf("%s\n\n[WARNING] Failed to create new branch %q: %v", msg, newBranch, bErr)
		} else {
			if sErr := h.git.Switch(newBranch); sErr != nil {
				msg = fmt.Sprintf("%s\n\n[WARNING] Failed to switch to new branch %q: %v", msg, newBranch, sErr)
			} else {
				msg = fmt.Sprintf("%s\n\n[SUCCESS] Created and switched to new branch %q", msg, newBranch)
			}
		}
	}

	return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("MERGE", true, msg, "consider calling diff to verify the result")), nil
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
