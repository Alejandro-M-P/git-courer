package stage

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blak0p/git-courer/internal/adapters/git"
	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

type Handler struct {
	git    ports.Git
	notify *domain.Backup // reserved for notification integration
}

func NewHandler(git ports.Git, notify *domain.Backup) *Handler {
	return &Handler{git: git, notify: notify}
}

func (h *Handler) HandleStage(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(shared.GetStringParam(params, "command", ""))

	dryRun := false
	if v, ok := params["dry_run"].(bool); ok {
		dryRun = v
	}
	confirmed := false
	if v, ok := params["confirmed"].(bool); ok {
		confirmed = v
	}

	if result, err := shared.ValidateKnownParams(params, []string{"command", "target_paths", "target_commit", "dry_run", "confirmed"}); result != nil || err != nil {
		return result, err
	}

	validCommands := []string{"RM", "RESTORE", "CLEAN"}
	valid := false
	for _, c := range validCommands {
		if command == c {
			valid = true
			break
		}
	}
	if !valid {
		hint := shared.SuggestCommand(command, validCommands)
		if hint != "" {
			return shared.JSONErrorResult("stage", fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return shared.JSONErrorResult("stage", fmt.Errorf("unknown command: %s", command))
	}

	// Validate required params
	switch command {
	case "RM", "RESTORE":
		if result, err := shared.ValidateRequiredParam(params, "target_paths", command); result != nil || err != nil {
			return result, err
		}
	}

	// Safety gate for CLEAN
	switch command {
	case "CLEAN":
		if result, err := shared.CheckSafetyGate("clean", dryRun, confirmed); result != nil || err != nil {
			return result, err
		}
		if dryRun {
			impact, _ := shared.ComputeImpact("clean", params)
			jsonBytes, _ := json.Marshal(impact)
			return mcpgo.NewToolResultText(string(jsonBytes)), nil
		}
	}

	paths := shared.GetStringParam(params, "target_paths", "")

	// Auto-create backup before destructive stage operations for undo safety
	_, _ = h.git.CreateBackup(command, domain.StashNone)

	var err error
	var result string

	switch command {
	case "RM":
		pathList := git.SplitPaths(paths)
		err = h.git.Remove(pathList)
		result = shared.WriteResultJSON("RM", err == nil, fmt.Sprintf("%d files removed", len(pathList)))
	case "RESTORE":
		pathList := git.SplitPaths(paths)
		err = h.git.Restore(pathList)
		result = shared.WriteHintedResultJSON("RESTORE", err == nil, fmt.Sprintf("%d files restored", len(pathList)), "consider calling status to verify working tree")
	case "CLEAN":
		err = h.git.Clean()
		result = shared.WriteResultJSON("CLEAN", err == nil, "Untracked files cleaned")
	}

	if err != nil {
		return shared.JSONErrorResult(command, err)
	}

	return mcpgo.NewToolResultText(result), nil
}

// HandleStash handles the stash tool commands (SAVE, POP, SHOW).
func (h *Handler) HandleStash(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	if result, err := shared.ValidateKnownParams(params, []string{"command", "commit_message", "stash_index", "include_untracked", "diff"}); result != nil || err != nil {
		return result, err
	}

	command := shared.GetStringParam(params, "command", "")
	if command == "" {
		return shared.JSONErrorResult("stash", fmt.Errorf("command is required for stash"))
	}

	validStashCommands := []string{"SAVE", "POP", "SHOW"}

	switch command {
	case "SAVE":
		// Auto-create backup before stash save for undo safety
		_, _ = h.git.CreateBackup(command, domain.StashNone)
		return h.handleStashSave(params)
	case "POP":
		// Auto-create backup before stash pop for undo safety
		_, _ = h.git.CreateBackup(command, domain.StashNone)
		return h.handleStashPop(params)
	case "SHOW":
		return h.handleStashShow(params)
	default:
		hint := shared.SuggestCommand(command, validStashCommands)
		if hint != "" {
			return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}
}

func (h *Handler) handleStashSave(params map[string]any) (*mcpgo.CallToolResult, error) {
	message := shared.GetStringParam(params, "commit_message", "")
	includeUntracked := false
	if v, ok := params["include_untracked"].(bool); ok {
		includeUntracked = v
	}

	var err error
	if includeUntracked {
		_, err = h.git.StashWithUntracked(message)
	} else if message != "" {
		_, err = h.git.Stash(message)
	} else {
		_, err = h.git.Stash()
	}

	if err != nil {
		return shared.JSONErrorResult("SAVE", err)
	}
	return mcpgo.NewToolResultText(shared.WriteResultJSON("STASH_SAVE", true, "Changes stashed")), nil
}

func (h *Handler) handleStashPop(params map[string]any) (*mcpgo.CallToolResult, error) {
	stashIndex := shared.GetStringParam(params, "stash_index", "")
	if stashIndex != "" {
		// stash_index provided: use StashApply instead of StashPop
		_, err := h.git.StashApply(stashIndex)
		if err != nil {
			if strings.Contains(err.Error(), "STASH_POP_UNTRACKED:") {
				return mcpgo.NewToolResultText(`{"error":"Stash apply failed: untracked files conflict","hint":"Use 'SAVE' command with 'include_untracked' parameter set to true next time"}`), nil
			}
			return shared.JSONErrorResult("POP", err)
		}
		return mcpgo.NewToolResultText(shared.WriteResultJSON("STASH_POP", true, fmt.Sprintf("Stash@{%s} applied", stashIndex))), nil
	}

	_, err := h.git.StashPop()
	if err != nil {
		if strings.Contains(err.Error(), "STASH_POP_UNTRACKED:") {
			return mcpgo.NewToolResultText(`{"error":"Stash pop failed: untracked files conflict","hint":"Use 'SAVE' command with 'include_untracked' parameter set to true next time"}`), nil
		}
		return shared.JSONErrorResult("POP", err)
	}
	return mcpgo.NewToolResultText(shared.WriteResultJSON("STASH_POP", true, "Stash restored")), nil
}

func (h *Handler) handleStashShow(params map[string]any) (*mcpgo.CallToolResult, error) {
	output, err := h.git.StashShow()
	if err != nil {
		return shared.JSONErrorResult("SHOW", err)
	}
	return mcpgo.NewToolResultText(output), nil
}
