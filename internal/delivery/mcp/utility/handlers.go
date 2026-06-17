package utility

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/blak0p/git-courer/internal/config"
	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type Handler struct {
	git       ports.Git
	cfg       *config.Config
	workDir   string
	mcpServer *server.MCPServer
}

func NewHandler(git ports.Git, cfg *config.Config, workDir string, mcpServer *server.MCPServer) *Handler {
	return &Handler{git: git, cfg: cfg, workDir: workDir, mcpServer: mcpServer}
}

// HandleBackup handles backup tool commands (RESTORE, LIST).
func (h *Handler) HandleBackup(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	if result, err := shared.ValidateKnownParams(params, []string{"command", "ref"}); result != nil || err != nil {
		return result, err
	}

	command := shared.GetStringParam(params, "command", "")
	if command == "" {
		return shared.JSONErrorResult("backup", fmt.Errorf("command is required for backup"))
	}

	validBackupCommands := []string{"RESTORE", "LIST"}

	switch command {
	case "RESTORE":
		return h.handleBackupRestore(params)
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

func (h *Handler) handleBackupRestore(params map[string]any) (*mcpgo.CallToolResult, error) {
	backups, err := h.git.ListBackups()
	if err != nil {
		return shared.JSONErrorResult("RESTORE", err)
	}

	if len(backups) == 0 {
		return shared.JSONErrorResult("RESTORE", fmt.Errorf("no backups available to restore"))
	}

	ref := shared.GetStringParam(params, "ref", "")

	var target domain.Backup
	if ref != "" {
		found := false
		for _, b := range backups {
			if b.Ref == ref {
				target = b
				found = true
				break
			}
		}
		if !found {
			return shared.JSONErrorResult("RESTORE", fmt.Errorf("unknown backup ref: %s", ref))
		}
	} else {
		target = backups[0]
	}

	if err := h.git.RestoreBackup(target); err != nil {
		return shared.JSONErrorResult("RESTORE", err)
	}

	msg := fmt.Sprintf("Successfully restored %s (%s)", target.Operation, target.Ref)
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
