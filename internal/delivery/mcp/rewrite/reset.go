package rewrite

import (
	"encoding/json"
	"fmt"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// ─── RESET SOFT ───────────────────────────────────────────────────────

func (h *Handler) handleResetSoft(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := shared.ValidateRequiredParam(params, "target_commit", "SOFT"); result != nil || err != nil {
		return result, err
	}
	if result, err := shared.ValidateKnownParams(params, []string{"command", "target_commit"}); result != nil || err != nil {
		return result, err
	}

	commit := shared.GetStringParam(params, "target_commit", "")

	// Auto-create backup before destructive reset for undo safety
	_, _ = h.git.CreateBackup("SOFT", domain.StashNone)

	err := h.git.ResetSoft(commit)
	if err != nil {
		return shared.JSONErrorResult("SOFT", err)
	}
	return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("RESET_SOFT", true, fmt.Sprintf("Soft reset to %s", commit), "use backup RESTORE to undo if needed")), nil
}

// ─── RESET HARD ───────────────────────────────────────────────────────

func (h *Handler) handleResetHard(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := shared.ValidateRequiredParam(params, "target_commit", "HARD"); result != nil || err != nil {
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

	if result, err := shared.CheckSafetyGate("rewrite_hard", dryRun, confirmed); result != nil || err != nil {
		return result, err
	}
	if dryRun {
		impact, _ := shared.ComputeImpact("rewrite_hard", params)
		jsonBytes, _ := json.Marshal(impact)
		return mcpgo.NewToolResultText(string(jsonBytes)), nil
	}

	commit := shared.GetStringParam(params, "target_commit", "")

	// Auto-create backup before destructive reset for undo safety
	_, _ = h.git.CreateBackup("HARD", domain.StashNone)

	_, err := h.git.Reset("--hard", commit)
	if err != nil {
		return shared.JSONErrorResult("HARD", err)
	}
	return mcpgo.NewToolResultText(shared.WriteResultJSON("RESET_HARD", true, fmt.Sprintf("Hard reset to %s", commit))), nil
}
