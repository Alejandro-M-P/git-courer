package mcp

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/workflow"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	StagedMarker   = "=== STAGED ==="
	UnstagedMarker = "=== UNSTAGED ==="
)

func (s *Server) startKeepalive(operation string, interval time.Duration) chan struct{} {
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				if s.mcpServer == nil {
					return
				}
				s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
					"level":  "info",
					"logger": "git-courer",
					"data":   "⚙️ Processing " + operation + "... analyzing code and context.",
				})
			}
		}
	}()
	return done
}

// sendErrorNotification sends a structured error notification to all clients.
func (s *Server) sendErrorNotification(operation, message string, details map[string]any) {
	if s.mcpServer == nil {
		return
	}
	s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
		"level":     "error",
		"logger":    "git-courer",
		"operation": operation,
		"message":   "❌ Error: " + message,
		"data":      details,
		"hint":      "Verify your git state and configuration. Use ABORT if needed to reset.",
	})
}

// sendSuccessNotification sends a structured success notification to all clients.
func (s *Server) sendSuccessNotification(operation, message string, summary any) {
	data := map[string]any{
		"operation": operation,
		"message":   "✅ Success: " + message,
	}

	if summ, ok := summary.(*workflow.Summary); ok && summ != nil {
		data["operation"] = summ.Operation
		data["files"] = summ.FilesAffected
		data["impact"] = summ.Impact
		data["security"] = summ.SecurityCheck
		if summ.Message != "" {
			data["message"] = "✅ " + summ.Message
		}
		if summ.Reasoning != "" {
			data["reasoning"] = summ.Reasoning
		}
		if len(summ.Messages) > 0 {
			data["messages"] = summ.Messages
		}
	} else if summary != nil {
		data["details"] = summary
	}

	if s.mcpServer == nil {
		return
	}
	s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
		"level":     "info",
		"logger":    "git-courer",
		"operation": operation,
		"message":   data["message"],
		"data":      data,
	})
}

// sendSecurityErrorNotification sends a structured error notification for security blocks.
func (s *Server) sendSecurityErrorNotification(errMsg string) {
	if s.mcpServer == nil {
		return
	}
	s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
		"level":     "error",
		"logger":    "git-courer",
		"operation": "security",
		"error":     "SECRET_DETECTED",
		"message":   "🔒 SECURITY BLOCK: Credentials or secrets detected in the diff.",
		"data": map[string]any{
			"details": errMsg,
			"action":  "Removing sensitive data before committing is a best practice. Use environment variables instead.",
		},
	})
}

// registerTools registers the MCP tools on the server.
func registerTools(s *server.MCPServer, srv *Server) {
	s.AddTool(
		mcpgo.NewTool("git_branch",
			mcpgo.WithDescription(descGitBranch),
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithIdempotentHintAnnotation(false),
			mcpgo.WithOpenWorldHintAnnotation(false),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("CREATE", "DELETE", "RENAME", "REMOTE_DELETE", "SET_UPSTREAM", "UNSET_UPSTREAM")),
			mcpgo.WithString("branch_name"),
			mcpgo.WithString("new_branch_name"),
			mcpgo.WithString("remote_name"),
			mcpgo.WithBoolean("force"),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Confirm destructive execution")),
		),
		srv.handleGitBranch,
	)

	s.AddTool(
		mcpgo.NewTool("git_tag",
			mcpgo.WithDescription(descGitTag),
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithIdempotentHintAnnotation(false),
			mcpgo.WithOpenWorldHintAnnotation(false),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("CREATE", "DELETE", "PUSH", "DELETE_REMOTE")),
			mcpgo.WithString("tag_name"),
			mcpgo.WithString("commit_message"),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Confirm destructive execution")),
		),
		srv.handleGitTag,
	)

	s.AddTool(
		mcpgo.NewTool("git_stash",
			mcpgo.WithDescription(descGitStash),
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithIdempotentHintAnnotation(false),
			mcpgo.WithOpenWorldHintAnnotation(false),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("SAVE", "POP", "APPLY", "DROP", "CLEAR", "SHOW")),
			mcpgo.WithString("commit_message"),
			mcpgo.WithString("stash_index"),
			mcpgo.WithBoolean("include_untracked"),
		),
		srv.handleGitStash,
	)

		s.AddTool(
		mcpgo.NewTool("git_backup",
			mcpgo.WithDescription(descGitBackup),
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(false),
			mcpgo.WithIdempotentHintAnnotation(false),
			mcpgo.WithOpenWorldHintAnnotation(false),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("RESTORE", "UNDO", "LIST", "PRUNE")),
			mcpgo.WithNumber("days"),
		),
		srv.handleGitBackup,
	)

	s.AddTool(
		mcpgo.NewTool("git_config",
			mcpgo.WithDescription(descGitConfig),
			mcpgo.WithReadOnlyHintAnnotation(true),
			mcpgo.WithDestructiveHintAnnotation(false),
			mcpgo.WithIdempotentHintAnnotation(true),
			mcpgo.WithOpenWorldHintAnnotation(false),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("READ", "LIST_MODELS")),
		),
		srv.handleGitConfig,
	)

	s.AddTool(
		mcpgo.NewTool("git_sync",
			mcpgo.WithDescription(descGitSync),
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithIdempotentHintAnnotation(false),
			mcpgo.WithOpenWorldHintAnnotation(false),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("FETCH", "PULL", "PUSH", "MERGE", "MERGE_ABORT", "SWITCH", "REBASE", "REBASE_ABORT", "REBASE_CONTINUE", "CHERRY_PICK", "ADD_REMOTE", "REMOVE_REMOTE")),
			mcpgo.WithString("remote_name"),
			mcpgo.WithString("branch_name"),
			mcpgo.WithString("target_commit"),
			mcpgo.WithBoolean("dry_run", mcpgo.Description("Preview impact without executing")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Confirm destructive execution")),
		),
		srv.handleGitSync,
	)

	s.AddTool(
		mcpgo.NewTool("git_stage",
			mcpgo.WithDescription(descGitStage),
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithIdempotentHintAnnotation(false),
			mcpgo.WithOpenWorldHintAnnotation(false),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("ADD", "RM", "RESTORE", "RESET_SOFT", "RESET_MIXED", "RESET_HARD", "CLEAN")),
			mcpgo.WithString("target_paths"),
			mcpgo.WithString("target_commit"),
			mcpgo.WithBoolean("dry_run", mcpgo.Description("Preview impact without executing")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Confirm destructive execution")),
		),
		srv.handleGitStage,
	)

	s.AddTool(
		mcpgo.NewTool("git_diff",
			mcpgo.WithDescription(descGitDiff),
			mcpgo.WithReadOnlyHintAnnotation(true),
			mcpgo.WithDestructiveHintAnnotation(false),
			mcpgo.WithIdempotentHintAnnotation(true),
			mcpgo.WithOpenWorldHintAnnotation(false),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("READ_DIFF", "READ_DIFF_STATS", "READ_DIFF_STAT", "READ_DIFF_STAGED", "READ_DIFF_ALL", "STASH_DIFF")),
			mcpgo.WithString("target_paths"),
			mcpgo.WithString("filter"),
			mcpgo.WithNumber("limit"),
			mcpgo.WithNumber("offset"),
			mcpgo.WithBoolean("compact"),
		),
		srv.handleGitDiff,
	)

	s.AddTool(
		mcpgo.NewTool("git_log",
			mcpgo.WithDescription(descGitLog),
			mcpgo.WithReadOnlyHintAnnotation(true),
			mcpgo.WithDestructiveHintAnnotation(false),
			mcpgo.WithIdempotentHintAnnotation(true),
			mcpgo.WithOpenWorldHintAnnotation(false),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("READ_LOG", "READ_BRANCHES", "READ_TAGS", "REMOTE_BRANCH_LIST", "REMOTE_TAG_LIST", "BLAME", "SHOW", "REFLOG", "MERGE_BASE", "READ_SEARCH", "CAT_FILE", "LIST_TREE")),
			mcpgo.WithString("target_commit"),
			mcpgo.WithString("target_paths"),
			mcpgo.WithString("pattern"),
			mcpgo.WithString("filter"),
			mcpgo.WithNumber("limit"),
			mcpgo.WithNumber("offset"),
			mcpgo.WithBoolean("recursive"),
			mcpgo.WithNumber("context"),
			mcpgo.WithNumber("before"),
			mcpgo.WithNumber("after"),
		),
		srv.handleGitLog,
	)

	s.AddTool(
		mcpgo.NewTool("git_status",
			mcpgo.WithDescription(descGitStatus),
			mcpgo.WithReadOnlyHintAnnotation(true),
			mcpgo.WithDestructiveHintAnnotation(false),
			mcpgo.WithIdempotentHintAnnotation(true),
			mcpgo.WithOpenWorldHintAnnotation(false),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("READ_STATUS", "CURRENT_BRANCH", "IS_REPO", "REMOTE_INFO", "WHAT_CHANGED")),
			mcpgo.WithString("filter"),
			mcpgo.WithNumber("limit"),
			mcpgo.WithNumber("offset"),
			mcpgo.WithBoolean("llm"),
		),
		srv.handleGitStatus,
	)

	s.AddTool(
		mcpgo.NewTool("git_review",
			mcpgo.WithDescription(descGitReview),
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithIdempotentHintAnnotation(false),
			mcpgo.WithOpenWorldHintAnnotation(false),
			mcpgo.WithString("command", mcpgo.Required(), mcpgo.Enum("STATUS", "SUMMARY", "JOB_RESULT", "REVERT", "AMEND", "COMMIT_START", "COMMIT_APPLY", "COMMIT_ABORT", "COMMIT_REGENERATE", "RELEASE_START", "RELEASE_APPLY", "RELEASE_ABORT")),
			mcpgo.WithString("instruction"),
			mcpgo.WithBoolean("preview"),
			mcpgo.WithString("feedback"),
			mcpgo.WithString("job_id"),
			mcpgo.WithString("branch_name"),
			mcpgo.WithBoolean("dry_run", mcpgo.Description("Preview impact without executing")),
		),
		srv.handleGitReview,
	)

	s.AddTool(
		mcpgo.NewTool("git_revert",
			mcpgo.WithDescription(descGitRevert),
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("target_commit", mcpgo.Required()),
			mcpgo.WithBoolean("dry_run", mcpgo.Description("Preview impact without executing")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Confirm destructive execution")),
		),
		srv.handleGitReview,
	)

	s.AddTool(
		mcpgo.NewTool("git_amend",
			mcpgo.WithDescription(descGitAmend),
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("commit_message"),
			mcpgo.WithString("target_paths"),
			mcpgo.WithBoolean("dry_run", mcpgo.Description("Preview impact without executing")),
			mcpgo.WithBoolean("confirmed", mcpgo.Description("Confirm destructive execution")),
		),
		srv.handleGitReview,
	)
}

func (s *Server) applyWithBackup(operation string, stashUntracked bool, fn func() (workflow.Result, error)) (workflow.Result, error) {
	mode := domain.StashUnstaged
	if stashUntracked {
		mode = domain.StashAll
	}
	backup, bErr := s.git.CreateBackup(operation, mode)
	if bErr != nil {
		log.Printf("⚠ backup creation failed for %s: %v — proceeding without backup", operation, bErr)
		return fn()
	}

	res, fnErr := fn()
	if fnErr != nil {
		if rErr := s.git.RestoreBackup(backup); rErr != nil {
			s.git.DeleteBackup(backup)
			return workflow.Result{}, fmt.Errorf("operation failed and restore also failed: %v (restore: %v)", fnErr, rErr)
		}
		s.git.DeleteBackup(backup)
		return workflow.Result{}, fmt.Errorf("operation failed - repo restored: %v", fnErr)
	}

	s.git.DeleteBackup(backup)
	return res, nil
}

// jsonErrorResult returns a structured JSON error.
func jsonErrorResult(command string, err error) (*mcpgo.CallToolResult, error) {
	errJSON := fmt.Sprintf(`{"status":"error","command":%q,"error":%q}`, command, err.Error())
	return mcpgo.NewToolResultError(errJSON), nil
}

func parseCommand(command string) (op, phase string) {
	for _, suffix := range []string{"_START", "_APPLY", "_ABORT", "_REGENERATE"} {
		if strings.HasSuffix(strings.ToUpper(command), suffix) {
			phase = strings.ToLower(suffix[1:])
			opRaw := command[:len(command)-len(suffix)]
			op = strings.ToLower(opRaw)
			return
		}
	}
	return "", ""
}

func extractExplicitArgs(req mcpgo.CallToolRequest) map[string]string {
	params, ok := req.Params.Arguments.(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]string)
	explicitKeys := []string{"branch_name", "preview", "feedback"}
	for _, key := range explicitKeys {
		val := params[key]
		if val == nil {
			continue
		}
		switch v := val.(type) {
		case string:
			result[key] = v
		case bool:
			result[key] = fmt.Sprintf("%v", v)
		default:
			result[key] = fmt.Sprintf("%v", v)
		}
	}
	return result
}

func parsePagination(params map[string]any) (limit, offset int) {
	if v, ok := params["limit"].(float64); ok {
		limit = int(v)
	}
	if v, ok := params["offset"].(float64); ok {
		offset = int(v)
	}
	return
}

func getStringParam(params map[string]any, key, def string) string {
	if v, ok := params[key].(string); ok && v != "" {
		return v
	}
	return def
}


var operationUndoability = map[string]bool{
	"commit":          true,
	"merge":           true,
	"rebase":          true,
	"cherry_pick":     true,
	"branch_create":   true,
	"branch_rename":   true,
	"branch_set_upstream":   true,
	"branch_unset_upstream": true,
	"tag_create":      true,
	"tag_push":        false,
	"push":            false,
	"remote_remove":   false,
	"remove_remote":   false,
	"branch_delete":   false,
	"remote_delete":   false,
	"tag_delete":      false,
	"delete_remote":   false,
	"delete_tag_remote": false,
	"release_apply":   false,
	"reset_soft":      true,
	"reset_mixed":     true,
	"reset_hard":      false,
	"clean":           false,
	"revert":          true,
	"amend":           true,
	"add":             true,
	"rm":              true,
	"restore":         true,
	"stash_save":      true,
	"stash_pop":       true,
	"stash_drop":      false,
	"stash_clear":     false,
}

// destructiveCommands lists operations that require confirmed=true when dry_run=false.
var destructiveCommands = map[string]bool{
	"push":          true,
	"remote_remove": true,
	"remove_remote": true,
	"branch_delete": true,
	"remote_delete": true,
	"tag_delete":    true,
	"delete_remote": true,
	"reset_hard":    true,
	"clean":         true,
}

// checkSafetyGate validates dry_run and confirmed for destructive commands.
// Returns nil when execution may proceed, otherwise an MCP result with an error message.
func checkSafetyGate(cmd string, dryRun, confirmed bool) (*mcpgo.CallToolResult, error) {
	// If dry_run is true, allow preview (computeImpact will be handled by caller).
	if dryRun {
		return nil, nil
	}
	// If command is not flagged as destructive, proceed without confirmation.
	if !destructiveCommands[cmd] {
		return nil, nil
	}
	// Destructive command requires confirmed=true.
	if !confirmed {
		impact, _ := computeImpact(cmd, nil)
		impact["status"] = "blocked"
		impact["reason"] = fmt.Sprintf("%s requires explicit confirmation. Set confirmed=true to proceed.", cmd)
		jsonBytes, err := json.Marshal(impact)
		if err != nil {
			return nil, err
		}
		return mcpgo.NewToolResultError(string(jsonBytes)), nil
	}
	return nil, nil
}

// computeImpact returns a preview JSON map describing the operation's effects.
func computeImpact(cmd string, params map[string]any) (map[string]any, error) {
	result := map[string]any{
		"operation": cmd,
		"undoable":  operationUndoability[cmd],
	}

	switch cmd {
	case "push":
		result["affected_refs"] = []string{"HEAD"}
		result["remote"] = "origin"
		result["hint"] = "Will push local commits to remote. Not undoable via git_backup."
	case "clean":
		result["affected_files"] = "untracked"
		result["hint"] = "Will remove all untracked files. Not undoable via git_backup."
	case "reset_hard":
		result["affected_refs"] = []string{"HEAD", "working tree"}
		result["hint"] = "Will reset working tree and index. Not undoable via git_backup."
	case "branch_delete":
		result["affected_refs"] = []string{"local branch"}
		result["hint"] = "Will delete a local branch. Not undoable via git_backup."
	case "remote_delete":
		result["affected_refs"] = []string{"remote branch"}
		result["remote"] = "origin"
		result["hint"] = "Will delete a remote branch. Not undoable via git_backup."
	case "tag_delete":
		result["affected_refs"] = []string{"local tag"}
		result["hint"] = "Will delete a local tag. Not undoable via git_backup."
	case "delete_remote":
		result["affected_refs"] = []string{"remote tag"}
		result["remote"] = "origin"
		result["hint"] = "Will delete a remote tag. Not undoable via git_backup."
	case "remote_remove":
		result["affected_refs"] = []string{"remote"}
		result["hint"] = "Will remove a remote. Not undoable via git_backup."
	case "merge":
		result["affected_refs"] = []string{"HEAD", "branch"}
		result["hint"] = "Will merge the specified branch. Undoable via git_backup RESTORE."
	case "rebase":
		result["affected_refs"] = []string{"HEAD", "branch"}
		result["hint"] = "Will rebase current branch onto target. Undoable via git_backup RESTORE. Use REBASE_ABORT if conflicts arise."
	case "cherry_pick":
		result["affected_refs"] = []string{"HEAD"}
		result["hint"] = "Will apply commit on top of current branch. Undoable via git_backup RESTORE."
	case "revert":
		result["affected_refs"] = []string{"HEAD"}
		result["hint"] = "Will create a revert commit. Undoable via git_backup RESTORE."
	case "amend":
		result["affected_refs"] = []string{"HEAD"}
		result["hint"] = "Will amend the last commit. Undoable via git_backup RESTORE."
	default:
		result["hint"] = "Operation preview not available. Proceed with caution."
	}

	return result, nil
}

