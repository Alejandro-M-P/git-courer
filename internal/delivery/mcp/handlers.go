package mcp

import (
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
			mcpgo.WithString("command", mcpgo.Required()),
			mcpgo.WithString("branch_name"),
			mcpgo.WithString("new_branch_name"),
			mcpgo.WithString("remote_name"),
			mcpgo.WithBoolean("force"),
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
			mcpgo.WithString("command", mcpgo.Required()),
			mcpgo.WithString("tag_name"),
			mcpgo.WithString("commit_message"),
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
			mcpgo.WithString("command", mcpgo.Required()),
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
			mcpgo.WithString("command", mcpgo.Required()),
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
			mcpgo.WithString("command", mcpgo.Required()),
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
			mcpgo.WithString("command", mcpgo.Required()),
			mcpgo.WithString("remote_name"),
			mcpgo.WithString("branch_name"),
			mcpgo.WithString("target_commit"),
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
			mcpgo.WithString("command", mcpgo.Required()),
			mcpgo.WithString("target_paths"),
			mcpgo.WithString("target_commit"),
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
			mcpgo.WithString("command", mcpgo.Required()),
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
			mcpgo.WithString("command", mcpgo.Required()),
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
			mcpgo.WithString("command", mcpgo.Required()),
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
			mcpgo.WithString("command", mcpgo.Required()),
			mcpgo.WithString("instruction"),
			mcpgo.WithBoolean("preview"),
			mcpgo.WithString("feedback"),
			mcpgo.WithString("job_id"),
			mcpgo.WithString("branch_name"),
		),
		srv.handleGitReview,
	)

	s.AddTool(
		mcpgo.NewTool("git_revert",
			mcpgo.WithDescription("Revert a specific commit. Safer than 'git revert' in bash: auto-backup before mutation."),
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("target_commit", mcpgo.Required()),
		),
		srv.handleGitReview,
	)

	s.AddTool(
		mcpgo.NewTool("git_amend",
			mcpgo.WithDescription("Amend the last commit message or add staged files. Safer than 'git commit --amend' in bash."),
			mcpgo.WithReadOnlyHintAnnotation(false),
			mcpgo.WithDestructiveHintAnnotation(true),
			mcpgo.WithString("commit_message"),
			mcpgo.WithString("target_paths"),
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
