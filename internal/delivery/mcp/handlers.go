package mcp

import (
	"context"
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
		mcpgo.NewTool("git_read",
			mcpgo.WithDescription(descGitRead),
			mcpgo.WithString("command", mcpgo.Required()),
			mcpgo.WithString("arg"), // Fallback/Legacy
			mcpgo.WithString("path"),
			mcpgo.WithString("hash"),
			mcpgo.WithString("revision"),
			mcpgo.WithString("pattern"),
			mcpgo.WithNumber("limit"),
			mcpgo.WithNumber("offset"),
			mcpgo.WithString("filter"),
			mcpgo.WithBoolean("compact"),
			mcpgo.WithBoolean("llm"),
			mcpgo.WithNumber("context"),
			mcpgo.WithNumber("before"),
			mcpgo.WithNumber("after"),
		),
		srv.handleGitRead,
	)

	s.AddTool(
		mcpgo.NewTool("git_write",
			mcpgo.WithDescription(descGitWrite),
			mcpgo.WithString("command", mcpgo.Required()),
			mcpgo.WithString("arg"), // Fallback/Legacy
			mcpgo.WithString("paths"),
			mcpgo.WithString("branch"),
			mcpgo.WithString("message"),
			mcpgo.WithString("commit"),
			mcpgo.WithString("name"),
		),
		srv.handleGitWrite,
	)

	s.AddTool(
		mcpgo.NewTool("git_write_review",
			mcpgo.WithDescription(descGitWriteReview),
			mcpgo.WithString("command", mcpgo.Required()),
			mcpgo.WithString("instruction"),
			mcpgo.WithString("branch"),
			mcpgo.WithBoolean("preview"),
			mcpgo.WithString("feedback"),
		),
		srv.handleGitWriteReview,
	)
}

func (s *Server) handleGitWriteReview(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(params["command"].(string))

	if command == "STATUS" {
		status, _ := s.reviewWorkflow.PlanStatus()
		s.sendSuccessNotification("status", "Status retrieved", nil)
		return mcpgo.NewToolResultText(status), nil
	}
	if command == "SUMMARY" {
		status, _ := s.git.Status()
		s.sendSuccessNotification("summary", "Git summary retrieved", nil)
		return mcpgo.NewToolResultText(formatStatus(status)), nil
	}

	op, phase := parseCommand(command)
	if op == "" {
		return mcpgo.NewToolResultError("Invalid command format. Expected {OP}_{PHASE}"), nil
	}

	if op == "commit" {
		return s.handleCommitOperation(ctx, req, phase)
	}
	if op == "release" {
		return s.handleRelease(ctx, req, phase)
	}

	switch phase {
	case "start":
		instruction := req.GetString("instruction", "")
		explicitArgs := extractExplicitArgs(req)

		keepalive := s.startKeepalive("Processing "+op, 10*time.Second)
		res, err := s.reviewWorkflow.Run(context.Background(), op, instruction, explicitArgs)
		close(keepalive)

		if err != nil {
			s.sendErrorNotification(op, op+" failed", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError(op + " failed: " + err.Error()), nil
		}
		if res.Status == "completed" {
			s.sendSuccessNotification(op, op+" completed", res.Summary)
			return mcpgo.NewToolResultText(res.Output), nil
		}
		s.sendSuccessNotification(op, op+" ready for review", res.Summary)
		return mcpgo.NewToolResultText(readyJSON(res.Preview)), nil

	case "apply":
		if !s.reviewWorkflow.HasPendingPlan() {
			s.sendErrorNotification(op, "No pending operation", map[string]any{"hint": "Run " + strings.ToUpper(op) + "_START first"})
			return mcpgo.NewToolResultError("No pending " + op + " operation. Run " + strings.ToUpper(op) + "_START first."), nil
		}
		keepalive := s.startKeepalive("Applying "+op, 10*time.Second)
		res, err := s.applyWithBackup(op, false, func() (workflow.Result, error) {
			return s.reviewWorkflow.Apply(context.Background())
		})
		close(keepalive)
		if err != nil {
			s.reviewWorkflow.Abort()
			s.sendErrorNotification(op, "Execution failed", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError(err.Error()), nil
		}
		s.reviewWorkflow.Abort()
		s.sendSuccessNotification(op, op+" completed", res.Summary)
		return mcpgo.NewToolResultText(res.Output), nil

	case "abort":
		err := s.reviewWorkflow.Abort()
		if err != nil {
			return mcpgo.NewToolResultError(err.Error()), nil
		}
		s.sendSuccessNotification(op, op+" plan aborted", nil)
		return mcpgo.NewToolResultText(op + " plan aborted."), nil

	default:
		return mcpgo.NewToolResultError("Unknown command: " + command + ". Use {OP}_START, {OP}_APPLY, or {OP}_ABORT"), nil
	}
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
