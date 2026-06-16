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
	notify    *domain.Backup // reserved for notification integration
}

func NewHandler(git ports.Git, cfg *config.Config, workDir string, mcpServer *server.MCPServer) *Handler {
	return &Handler{git: git, cfg: cfg, workDir: workDir, mcpServer: mcpServer}
}

// HandleConfig returns the configuration and available models together,
// or handles config commands: SET_TEST_COMMAND, SET_USER_NAME, SET_USER_EMAIL, SET_SIGNING_KEY, GET.
func (h *Handler) HandleConfig(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	if result, err := shared.ValidateKnownParams(params, []string{"command", "test_command", "value"}); result != nil || err != nil {
		return result, err
	}

	command := shared.GetStringParam(params, "command", "")

	switch command {
	case "SET_TEST_COMMAND":
		testCmd := shared.GetStringParam(params, "test_command", "")

		// Write to project-local config (.git/git-courer/config.json)
		if err := h.writeProjectTestCommand(testCmd); err != nil {
			return shared.JSONErrorResult("SET_TEST_COMMAND", fmt.Errorf("failed to save project config: %w", err))
		}

		result := shared.WriteHintedResultJSON("SET_TEST_COMMAND", true,
			fmt.Sprintf("test_command set to %q", testCmd),
			"run pr_review to check readiness before creating a PR")
		return mcpgo.NewToolResultText(result), nil

	case "SET_USER_NAME":
		value := shared.GetStringParam(params, "value", "")
		if value == "" {
			return shared.JSONErrorResult("SET_USER_NAME", fmt.Errorf("value is required for SET_USER_NAME"))
		}
		if err := h.writeProjectConfigField("user_name", value); err != nil {
			return shared.JSONErrorResult("SET_USER_NAME", fmt.Errorf("failed to save project config: %w", err))
		}
		// Sychronize with real git config
		if _, err := h.git.ConfigSet("user.name", value); err != nil {
			return shared.JSONErrorResult("SET_USER_NAME", fmt.Errorf("failed to sync git config: %w", err))
		}
		return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("SET_USER_NAME", true,
			fmt.Sprintf("user_name set to %q", value),
			"this sets the local project git user.name")), nil

	case "SET_USER_EMAIL":
		value := shared.GetStringParam(params, "value", "")
		if value == "" {
			return shared.JSONErrorResult("SET_USER_EMAIL", fmt.Errorf("value is required for SET_USER_EMAIL"))
		}
		if err := h.writeProjectConfigField("user_email", value); err != nil {
			return shared.JSONErrorResult("SET_USER_EMAIL", fmt.Errorf("failed to save project config: %w", err))
		}
		// Synchronize with real git config
		if _, err := h.git.ConfigSet("user.email", value); err != nil {
			return shared.JSONErrorResult("SET_USER_EMAIL", fmt.Errorf("failed to sync git config: %w", err))
		}
		return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("SET_USER_EMAIL", true,
			fmt.Sprintf("user_email set to %q", value),
			"this sets the local project git user.email")), nil

	case "SET_SIGNING_KEY":
		value := shared.GetStringParam(params, "value", "")
		if value == "" {
			return shared.JSONErrorResult("SET_SIGNING_KEY", fmt.Errorf("value is required for SET_SIGNING_KEY"))
		}
		if err := h.writeProjectConfigField("signing_key", value); err != nil {
			return shared.JSONErrorResult("SET_SIGNING_KEY", fmt.Errorf("failed to save project config: %w", err))
		}
		// Synchronize with real git config
		if _, err := h.git.ConfigSet("user.signingkey", value); err != nil {
			return shared.JSONErrorResult("SET_SIGNING_KEY", fmt.Errorf("failed to sync git config: %w", err))
		}
		if _, err := h.git.ConfigSet("commit.gpgsign", "true"); err != nil {
			return shared.JSONErrorResult("SET_SIGNING_KEY", fmt.Errorf("failed to enable gpgsign: %w", err))
		}
		return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("SET_SIGNING_KEY", true,
			fmt.Sprintf("signing_key set to %q", value),
			"this enables signed commits with the specified GPG key")), nil

	case "GET":
		projCfg, err := config.LoadProjectConfig(h.workDir)
		if err != nil {
			// If no project config exists, return global config only
			configPath := config.GlobalConfigPath()
			result := shared.MustJSON(map[string]interface{}{
				"config_path": configPath,
				"content":     h.cfg,
				"provider":    h.cfg.LLM.Provider,
				"models":      []string{h.cfg.LLM.Model},
				"message":     "Models are configured statically via config file. Showing current configured model.",
				"project":     nil,
			})
			return mcpgo.NewToolResultText(result), nil
		}
		configPath := config.GlobalConfigPath()
		result := shared.MustJSON(map[string]interface{}{
			"config_path": configPath,
			"content":     h.cfg,
			"provider":    h.cfg.LLM.Provider,
			"models":      []string{h.cfg.LLM.Model},
			"message":     "Models are configured statically via config file. Showing current configured model.",
			"project":     projCfg,
		})
		return mcpgo.NewToolResultText(result), nil

	default:
		// No command or empty command: read-only config display
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
}

// writeProjectTestCommand loads the existing project config, updates test_command, and saves it back.
func (h *Handler) writeProjectTestCommand(testCmd string) error {
	// Load existing project config (if any)
	cfg, err := config.LoadProjectConfig(h.workDir)
	if err != nil {
		// If no project config exists, create a new one
		cfg = &config.ProjectConfig{
			Description: "",
		}
	}
	cfg.TestCommand = testCmd
	return config.SaveProjectConfig(h.workDir, cfg)
}

// writeProjectConfigField loads the existing project config, updates a single field, and saves it back.
func (h *Handler) writeProjectConfigField(field, value string) error {
	cfg, err := config.LoadProjectConfig(h.workDir)
	if err != nil {
		cfg = &config.ProjectConfig{}
	}
	switch field {
	case "user_name":
		cfg.UserName = value
	case "user_email":
		cfg.UserEmail = value
	case "signing_key":
		cfg.SigningKey = value
	default:
		return fmt.Errorf("unknown config field: %s", field)
	}
	return config.SaveProjectConfig(h.workDir, cfg)
}

// HandleBackup handles backup tool commands (CREATE, DELETE, RESTORE, LIST).
func (h *Handler) HandleBackup(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	if result, err := shared.ValidateKnownParams(params, []string{"command", "ref", "confirmed"}); result != nil || err != nil {
		return result, err
	}

	command := shared.GetStringParam(params, "command", "")
	if command == "" {
		return shared.JSONErrorResult("backup", fmt.Errorf("command is required for backup"))
	}

	validBackupCommands := []string{"CREATE", "DELETE", "RESTORE", "LIST"}

	switch command {
	case "CREATE":
		return h.handleBackupCreate(params)
	case "DELETE":
		return h.handleBackupDelete(params)
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

func (h *Handler) handleBackupCreate(params map[string]any) (*mcpgo.CallToolResult, error) {
	operation := shared.GetStringParam(params, "ref", "MANUAL")
	backup, err := h.git.CreateBackup(operation, domain.StashNone)
	if err != nil {
		return shared.JSONErrorResult("CREATE", err)
	}
	result := shared.WriteHintedResultJSON("CREATE", true,
		fmt.Sprintf("Backup created: %s (%s)", backup.Ref, backup.Operation),
		"use backup RESTORE to undo if needed")
	return mcpgo.NewToolResultText(result), nil
}

func (h *Handler) handleBackupDelete(params map[string]any) (*mcpgo.CallToolResult, error) {
	ref := shared.GetStringParam(params, "ref", "")
	if ref == "" {
		return shared.JSONErrorResult("DELETE", fmt.Errorf("ref is required for DELETE"))
	}

	backups, err := h.git.ListBackups()
	if err != nil {
		return shared.JSONErrorResult("DELETE", err)
	}

	var target *domain.Backup
	for i := range backups {
		if backups[i].Ref == ref {
			target = &backups[i]
			break
		}
	}
	if target == nil {
		return shared.JSONErrorResult("DELETE", fmt.Errorf("unknown backup ref: %s", ref))
	}

	if err := h.git.DeleteBackup(*target); err != nil {
		return shared.JSONErrorResult("DELETE", err)
	}

	return mcpgo.NewToolResultText(shared.WriteResultJSON("DELETE", true, fmt.Sprintf("Backup %s deleted", ref))), nil
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
		// Find the specific backup by ref
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
		// Default to most recent backup (list is sorted newest-first by ListBackups)
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

// HandleUndo is an alias for backup RESTORE — restores the most recent backup.
func (h *Handler) HandleUndo(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	if result, err := shared.ValidateKnownParams(params, []string{}); result != nil || err != nil {
		return result, err
	}

	// Delegate to backup RESTORE with no ref (= most recent backup)
	return h.handleBackupRestore(params)
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
