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
		mcpgo.NewTool("git_status",
			mcpgo.WithDescription("Show the working tree status including staged, unstaged, and untracked files. Replaces: git status."),
			mcpgo.WithString("command", mcpgo.Required()),
			mcpgo.WithString("filter"),
			mcpgo.WithNumber("limit"),
			mcpgo.WithNumber("offset"),
			mcpgo.WithBoolean("llm"),
		),
		srv.handleGitStatus,
	)

	s.AddTool(
		mcpgo.NewTool("git_diff",
			mcpgo.WithDescription("Show changes between commits, commit and working tree, etc. Replaces: git diff."),
			mcpgo.WithString("command", mcpgo.Required()),
			mcpgo.WithString("arg"),
			mcpgo.WithString("path"),
			mcpgo.WithString("filter"),
			mcpgo.WithNumber("limit"),
			mcpgo.WithNumber("offset"),
			mcpgo.WithBoolean("compact"),
		),
		srv.handleGitDiff,
	)

	s.AddTool(
		mcpgo.NewTool("git_log",
			mcpgo.WithDescription("Show commit logs and history. Replaces: git log, git blame, git show."),
			mcpgo.WithString("command", mcpgo.Required()),
			mcpgo.WithString("arg"),
			mcpgo.WithString("revision"),
			mcpgo.WithString("path"),
			mcpgo.WithString("pattern"),
			mcpgo.WithString("filter"),
			mcpgo.WithNumber("limit"),
			mcpgo.WithNumber("offset"),
			mcpgo.WithBoolean("recursive"),
		),
		srv.handleGitLog,
	)

	s.AddTool(
		mcpgo.NewTool("git_stage",
			mcpgo.WithDescription("Add, remove, or reset file contents to the index. Replaces: git add, git rm."),
			mcpgo.WithString("command", mcpgo.Required()),
			mcpgo.WithString("arg"),
			mcpgo.WithString("paths"),
			mcpgo.WithString("commit"),
		),
		srv.handleGitStage,
	)

	s.AddTool(
		mcpgo.NewTool("git_sync",
			mcpgo.WithDescription("Push, pull, fetch, switch, or merge branches. Replaces: git push, git pull, git fetch, git switch, git merge."),
			mcpgo.WithString("command", mcpgo.Required()),
			mcpgo.WithString("arg"),
			mcpgo.WithString("remote"),
			mcpgo.WithString("branch"),
		),
		srv.handleGitSync,
	)

	s.AddTool(
		mcpgo.NewTool("git_manage",
			mcpgo.WithDescription("Manage branches, tags, remotes, and stashes. Replaces: git branch, git tag, git stash."),
			mcpgo.WithString("command", mcpgo.Required()),
			mcpgo.WithString("arg"),
			mcpgo.WithString("name"),
			mcpgo.WithBoolean("force"),
			mcpgo.WithBoolean("all"),
		),
		srv.handleGitManage,
	)

	s.AddTool(
		mcpgo.NewTool("git_review",
			mcpgo.WithDescription("AI-powered commit generation and release management. Replaces: git commit."),
			mcpgo.WithString("command", mcpgo.Required()),
			mcpgo.WithString("arg"),
			mcpgo.WithString("instruction"),
			mcpgo.WithBoolean("preview"),
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
	explicitKeys := []string{"branch", "preview", "feedback"}
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
