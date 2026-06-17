package rewrite

import (
	"encoding/json"

	"github.com/blak0p/git-courer/internal/adapters/git"
	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// ─── AMEND ────────────────────────────────────────────────────────────

func (h *Handler) handleAmend(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := shared.ValidateKnownParams(params, []string{"command", "commit_message", "target_paths", "confirmed", "dry_run"}); result != nil || err != nil {
		return result, err
	}

	dryRun := false
	if v, ok := params["dry_run"].(bool); ok {
		dryRun = v
	}
	confirmed := false
	if v, ok := params["confirmed"].(bool); ok {
		confirmed = v
	}

	if dryRun {
		impact, _ := shared.ComputeImpact("rewrite_amend", params)
		jsonBytes, _ := json.Marshal(impact)
		return mcpgo.NewToolResultText(string(jsonBytes)), nil
	}
	if result, err := shared.CheckSafetyGate("rewrite_amend", dryRun, confirmed); result != nil || err != nil {
		return result, err
	}

	// Auto-create backup before amend for undo safety
	_, _ = h.git.CreateBackup("AMEND", domain.StashNone)

	out, err := h.git.Amend(shared.GetStringParam(params, "commit_message", ""), git.SplitPaths(shared.GetStringParam(params, "target_paths", "")))
	if err != nil {
		return shared.JSONErrorResult("AMEND", err)
	}
	return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("AMEND", true, out, "use backup RESTORE to undo if needed")), nil
}

// ─── REVERT ───────────────────────────────────────────────────────────

func (h *Handler) handleRevert(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := shared.ValidateRequiredParam(params, "target_commit", "REVERT"); result != nil || err != nil {
		return result, err
	}
	if result, err := shared.ValidateKnownParams(params, []string{"command", "target_commit", "confirmed", "dry_run"}); result != nil || err != nil {
		return result, err
	}

	dryRun := false
	if v, ok := params["dry_run"].(bool); ok {
		dryRun = v
	}
	confirmed := false
	if v, ok := params["confirmed"].(bool); ok {
		confirmed = v
	}

	if dryRun {
		impact, _ := shared.ComputeImpact("rewrite_revert", params)
		jsonBytes, _ := json.Marshal(impact)
		return mcpgo.NewToolResultText(string(jsonBytes)), nil
	}
	if result, err := shared.CheckSafetyGate("rewrite_revert", dryRun, confirmed); result != nil || err != nil {
		return result, err
	}

	// Auto-create backup before revert for undo safety
	_, _ = h.git.CreateBackup("REVERT", domain.StashNone)

	out, err := h.git.Revert(shared.GetStringParam(params, "target_commit", ""))
	if err != nil {
		return shared.JSONErrorResult("REVERT", err)
	}
	return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("REVERT", true, out, "use backup RESTORE to undo if needed")), nil
}
