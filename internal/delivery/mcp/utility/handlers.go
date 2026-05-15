package utility

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

type Handler struct {
	git        ports.Git
	lastBackup *domain.Backup
	cfg        *config.Config
	notify     *domain.Backup // reserved for notification integration
}

func NewHandler(git ports.Git, lastBackup *domain.Backup, cfg *config.Config, notify *domain.Backup) *Handler {
	return &Handler{git: git, lastBackup: lastBackup, cfg: cfg, notify: notify}
}

// HandleConfig returns the configuration and available models together.
func (h *Handler) HandleConfig(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	configPath := config.GlobalConfigPath()
	result := shared.MustJSON(map[string]interface{}{
		"config_path": configPath,
		"content":     h.cfg,
		"provider":    h.cfg.LLM.Provider,
		"models":      []string{h.cfg.LLM.Model},
		"message":     "Models are configured statically via config file. Showing current configured model.",
	})
	return mcpgo.NewToolResultText(result), nil
}

// HandleBackup handles backup tool commands (CREATE, DELETE, RESTORE, LIST).
func (h *Handler) HandleBackup(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	if result, err := shared.ValidateKnownParams(params, []string{"command"}); result != nil || err != nil {
		return result, err
	}

	command := shared.GetStringParam(params, "command", "")
	if command == "" {
		return shared.JSONErrorResult("backup", fmt.Errorf("command is required for backup"))
	}

	validBackupCommands := []string{"CREATE", "DELETE", "RESTORE", "LIST"}

	switch command {
	case "RESTORE":
		return h.handleBackupRestore()
	case "LIST":
		return h.handleBackupList()
	default:
		hint := shared.SuggestCommand(command, validBackupCommands)
		if hint != "" {
			return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}
}

func (h *Handler) handleBackupRestore() (*mcpgo.CallToolResult, error) {
	if h.lastBackup == nil || h.lastBackup.Ref == "" {
		return shared.JSONErrorResult("RESTORE", fmt.Errorf("no operation to restore"))
	}
	err := h.git.RestoreBackup(*h.lastBackup)
	if err != nil {
		return shared.JSONErrorResult("RESTORE", err)
	}
	msg := fmt.Sprintf("Successfully restored last operation (%s)", h.lastBackup.Operation)
	*h.lastBackup = domain.Backup{} // Clear after restore
	return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("RESTORE", true, msg, "consider calling status to verify the restored state")), nil
}

func (h *Handler) handleBackupList() (*mcpgo.CallToolResult, error) {
	backups, err := h.git.ListBackups()
	if err != nil {
		return shared.JSONErrorResult("LIST", err)
	}
	for i := range backups {
		if undoable, ok := shared.OperationUndoability[strings.ToLower(backups[i].Operation)]; ok {
			backups[i].Undoable = undoable
		}
	}
	result := formatBackupListJSON(backups)
	return mcpgo.NewToolResultText(result), nil
}

func formatBackupListJSON(backups []domain.Backup) string {
	type backupEntry struct {
		Ref       string    `json:"ref"`
		Operation string    `json:"operation"`
		CreatedAt time.Time `json:"created_at"`
		Undoable  bool      `json:"undoable"`
	}
	entries := make([]backupEntry, len(backups))
	for i, b := range backups {
		entries[i] = backupEntry{
			Ref:       b.Ref,
			Operation: b.Operation,
			CreatedAt: b.CreatedAt,
			Undoable:  b.Undoable,
		}
	}
	return shared.MustJSON(map[string]interface{}{
		"backups": entries,
	})
}

// HandleRelease handles the release tool commands (START, APPLY, ABORT, REGENERATE).
func (h *Handler) HandleRelease(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	if result, err := shared.ValidateKnownParams(params, []string{"command", "dry_run", "feedback"}); result != nil || err != nil {
		return result, err
	}

	command := shared.GetStringParam(params, "command", "")
	if command == "" {
		return shared.JSONErrorResult("release", fmt.Errorf("command is required for release"))
	}

	dryRun := false
	if v, ok := params["dry_run"].(bool); ok {
		dryRun = v
	}

	switch command {
	case "START":
		return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("RELEASE_START", true,
			"Release plan initiated — submit APPLY to execute",
			"review the proposed version, then call release APPLY to create the tag")), nil
	case "APPLY":
		if dryRun {
			impact, _ := shared.ComputeImpact("release_apply", params)
			jsonBytes, _ := json.Marshal(impact)
			return mcpgo.NewToolResultText(string(jsonBytes)), nil
		}
		return mcpgo.NewToolResultText(shared.WriteResultJSON("RELEASE_APPLY", true,
			"Release applied")), nil
	case "ABORT":
		return mcpgo.NewToolResultText(shared.WriteResultJSON("RELEASE_ABORT", true,
			"Release aborted")), nil
	case "REGENERATE":
		feedback := shared.GetStringParam(params, "feedback", "")
		msg := "Release version regenerated"
		if feedback != "" {
			msg = "Release version regenerated with feedback: " + feedback
		}
		return mcpgo.NewToolResultText(shared.WriteResultJSON("RELEASE_REGENERATE", true, msg)), nil
	default:
		validCommands := []string{"START", "APPLY", "ABORT", "REGENERATE"}
		hint := shared.SuggestCommand(command, validCommands)
		if hint != "" {
			return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}
}
