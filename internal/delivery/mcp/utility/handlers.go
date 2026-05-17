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

// ReleaseSvc abstracts the release service methods used by the handler.
// This keeps the handler testable without requiring a full workflow.ReleaseService.
type ReleaseSvc interface {
	Prepare(instruction, userBump string) (*domain.ReleaseIntent, string, []string, error)
	PrepareAndGenerateAsync(instruction, userBump string)
	Execute(intent *domain.ReleaseIntent, changelog string) (string, error)
	SaveIntent(intent *domain.ReleaseIntent)
	LoadIntent() (*domain.ReleaseIntent, error)
	SaveChangelog(changelog string)
	LoadChangelog() (string, error)
	ClearPending()
	SetProgressCallback(fn func(done, total int))
}

type Handler struct {
	git       ports.Git
	cfg       *config.Config
	workDir   string
	releaseSvc ReleaseSvc
	notify    *domain.Backup // reserved for notification integration
}

func NewHandler(git ports.Git, cfg *config.Config, workDir string, releaseSvc ReleaseSvc) *Handler {
	return &Handler{git: git, cfg: cfg, workDir: workDir, releaseSvc: releaseSvc}
}

// HandleConfig returns the configuration and available models together,
// or handles the SET_TEST_COMMAND to update the test command in project-local config.
func (h *Handler) HandleConfig(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	if result, err := shared.ValidateKnownParams(params, []string{"command", "test_command"}); result != nil || err != nil {
		return result, err
	}

	command := shared.GetStringParam(params, "command", "")

	switch command {
	case "SET_TEST_COMMAND":
		testCmd := shared.GetStringParam(params, "test_command", "")

		// Write to project-local config (.git-courer/config.json)
		if err := h.writeProjectTestCommand(testCmd); err != nil {
			return shared.JSONErrorResult("SET_TEST_COMMAND", fmt.Errorf("failed to save project config: %w", err))
		}

		result := shared.WriteHintedResultJSON("SET_TEST_COMMAND", true,
			fmt.Sprintf("test_command set to %q", testCmd),
			"run pr_review to check readiness before creating a PR")
		return mcpgo.NewToolResultText(result), nil
	default:
		// No command or empty command: read-only config display
		configPath := config.GlobalConfigPath()
		result := shared.MustJSON(map[string]interface{}{
			"config_path": configPath,
			"content":     h.cfg,
			"provider":    h.cfg.LLM.Provider,
			"models":      []string{h.cfg.LLM.Model},
			"message":      "Models are configured statically via config file. Showing current configured model.",
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
			Areas:       make(map[string][]string),
		}
	}
	cfg.TestCommand = testCmd
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

// HandleRelease handles the release tool commands (START, APPLY, ABORT, REGENERATE).
func (h *Handler) HandleRelease(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	if result, err := shared.ValidateKnownParams(params, []string{"command", "dry_run", "feedback", "instruction"}); result != nil || err != nil {
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

	instruction := shared.GetStringParam(params, "instruction", "")
	feedback := shared.GetStringParam(params, "feedback", "")

	switch command {
	case "START":
		return h.handleReleaseStart(instruction, dryRun)
	case "APPLY":
		if dryRun {
			impact, _ := shared.ComputeImpact("release_apply", params)
			jsonBytes, _ := json.Marshal(impact)
			return mcpgo.NewToolResultText(string(jsonBytes)), nil
		}
		return h.handleReleaseApply()
	case "ABORT":
		return h.handleReleaseAbort()
	case "REGENERATE":
		return h.handleReleaseRegenerate(instruction, feedback)
	default:
		validCommands := []string{"START", "APPLY", "ABORT", "REGENERATE"}
		hint := shared.SuggestCommand(command, validCommands)
		if hint != "" {
			return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}
}

func (h *Handler) handleReleaseStart(instruction string, dryRun bool) (*mcpgo.CallToolResult, error) {
	intent, commits, warnings, err := h.releaseSvc.Prepare(instruction, "")
	if err != nil {
		return shared.JSONErrorResult("RELEASE_START", err)
	}

	// Dry-run: return the impact preview without storing state
	if dryRun {
		impact, _ := shared.ComputeImpact("release_apply", map[string]any{
			"tag_name": intent.TagName,
		})
		impact["tag_name"] = intent.TagName
		impact["version_bump"] = intent.VersionBump
		impact["is_release"] = intent.IsRelease
		jsonBytes, _ := json.Marshal(impact)
		return mcpgo.NewToolResultText(string(jsonBytes)), nil
	}

	// Store intent for later APPLY
	h.releaseSvc.SaveIntent(intent)

	// Build result with version info
	result := map[string]any{
		"success":       true,
		"operation":    "RELEASE_START",
		"tag_name":      intent.TagName,
		"version_bump":  intent.VersionBump,
		"is_release":     intent.IsRelease,
		"status":        "pending_approval",
		"hint":          "review the proposed version, then call release APPLY to create the tag",
	}
	if len(warnings) > 0 {
		result["warnings"] = warnings
	}

	// Kick off async changelog generation
	// The changelog will be available when the user calls APPLY
	if intent.IsRelease && commits != "" {
		h.releaseSvc.PrepareAndGenerateAsync(instruction, "")
	} else if commits == "" {
		result["message"] = "no new commits since last tag"
	}

	return mcpgo.NewToolResultText(shared.MustJSON(result)), nil
}

func (h *Handler) handleReleaseApply() (*mcpgo.CallToolResult, error) {
	intent, err := h.releaseSvc.LoadIntent()
	if err != nil || intent == nil {
		return shared.JSONErrorResult("RELEASE_APPLY", fmt.Errorf("no release plan found; call START first"))
	}

	changelog, _ := h.releaseSvc.LoadChangelog()

	result, err := h.releaseSvc.Execute(intent, changelog)
	if err != nil {
		return shared.JSONErrorResult("RELEASE_APPLY", err)
	}

	// Clear state after successful execution
	h.releaseSvc.ClearPending()

	return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("RELEASE_APPLY", true,
		result,
		"tag created and pushed; check your remote for the new release")), nil
}

func (h *Handler) handleReleaseAbort() (*mcpgo.CallToolResult, error) {
	h.releaseSvc.ClearPending()
	return mcpgo.NewToolResultText(shared.WriteResultJSON("RELEASE_ABORT", true,
		"Release aborted")), nil
}

func (h *Handler) handleReleaseRegenerate(instruction, feedback string) (*mcpgo.CallToolResult, error) {
	// Re-run Prepare with the original or updated instruction
	intent, commits, warnings, err := h.releaseSvc.Prepare(instruction, "")
	if err != nil {
		return shared.JSONErrorResult("RELEASE_REGENERATE", err)
	}

	// Store updated intent
	h.releaseSvc.SaveIntent(intent)

	msg := "Release version regenerated"
	if feedback != "" {
		msg = "Release version regenerated with feedback: " + feedback
	}

	result := map[string]any{
		"success":       true,
		"operation":    "RELEASE_REGENERATE",
		"tag_name":      intent.TagName,
		"version_bump":  intent.VersionBump,
		"is_release":     intent.IsRelease,
		"message":       msg,
		"status":        "pending_approval",
	}
	if len(warnings) > 0 {
		result["warnings"] = warnings
	}

	// Kick off async changelog generation with updated instructions
	if intent.IsRelease && commits != "" {
		h.releaseSvc.PrepareAndGenerateAsync(instruction, "")
	}

	return mcpgo.NewToolResultText(shared.MustJSON(result)), nil
}
