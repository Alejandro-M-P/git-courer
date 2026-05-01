package mcp

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/workflow"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// startKeepalive sends notifications during long blocking operations to inform the user.
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
// summary can be any type or nil.
func (s *Server) sendSuccessNotification(operation, message string, summary any) {
	data := map[string]any{
		"operation": operation,
		"message":   "✅ Success: " + message,
	}

	// Enrich data if summary is a workflow.Summary
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
			mcpgo.WithDescription("Read-only git operations. IMPORTANT: When result contains delimited output (>>>> or ═══), you MUST show the ENTIRE output to the user. Do NOT summarize. Commands: READ_STATUS | READ_DIFF | READ_DIFF_STAGED | READ_LOG | READ_BRANCHES | READ_TAGS"),
			mcpgo.WithString("command", mcpgo.Description("READ_STATUS | READ_DIFF | READ_DIFF_STAGED | READ_LOG | READ_BRANCHES | READ_TAGS"), mcpgo.Required()),
			mcpgo.WithString("arg", mcpgo.Description("Optional filter. For READ_DIFF/READ_DIFF_STAGED/READ_LOG: file paths. For READ_BRANCHES/READ_TAGS: glob pattern (e.g. 'feat/*', 'v1.*')")),
		),
		srv.handleGitRead,
	)

	s.AddTool(
		mcpgo.NewTool("git_write",
			mcpgo.WithDescription("Direct write git operations (no LLM). Commands: ADD | SWITCH | STASH | STASH_POP | PUSH | PULL | FETCH | RM | TAG_CREATE | TAG_DELETE | TAG_PUSH | TAG_DELETE_REMOTE"),
			mcpgo.WithString("command", mcpgo.Description("ADD | SWITCH | STASH | STASH_POP | PUSH | PULL | FETCH | RM | TAG_CREATE | TAG_DELETE | TAG_PUSH | TAG_DELETE_REMOTE"), mcpgo.Required()),
			mcpgo.WithString("arg", mcpgo.Description("Path, branch name, or tag name depending on command")),
		),
		srv.handleGitWrite,
	)

	s.AddTool(
		mcpgo.NewTool("git_write_review",
			mcpgo.WithDescription("Write git operations with confirmation. IMPORTANT: When preview contains delimited output (>>>> or ═══), you MUST show the ENTIRE delimited block to the user BEFORE asking for confirmation. Do NOT summarize. Copy-paste the full content. Three-phase protocol: {OP}_START → {OP}_APPLY | {OP}_ABORT. Ops: COMMIT, RELEASE, BRANCH_CREATE, BRANCH_DELETE, MERGE. Special: STATUS, SUMMARY."),
			mcpgo.WithString("command", mcpgo.Description("e.g. COMMIT_START | COMMIT_APPLY | COMMIT_REGENERATE | BRANCH_CREATE_START | BRANCH_CREATE_APPLY | BRANCH_DELETE_START | MERGE_START"), mcpgo.Required()),
			mcpgo.WithString("instruction", mcpgo.Description("Natural language instruction for START phase")),
			mcpgo.WithString("branch", mcpgo.Description("Branch name")),
			mcpgo.WithBoolean("preview", mcpgo.Description(fmt.Sprintf("If true, show preview before executing (default: %v)", srv.cfg.Validation.RequireConfirmation))),
			mcpgo.WithString("feedback", mcpgo.Description("Feedback for message regeneration (used with COMMIT_REGENERATE)")),
		),
		srv.handleGitWriteReview,
	)
}

func (s *Server) handleGitRead(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(params["command"].(string))
	arg := ""
	if v, ok := params["arg"].(string); ok {
		arg = v
	}

	var result string
	var err error

	switch command {
	case "READ_STATUS":
		status, err := s.git.Status()
		if err == nil {
			result = formatStatus(status)
		}
	case "READ_DIFF":
		result, err = s.git.Diff(arg)
	case "READ_DIFF_STAGED":
		result, err = s.git.DiffStaged(arg)
	case "READ_LOG":
		result, err = s.git.Log(5, arg)
	case "READ_BRANCHES":
		result, err = s.git.ListBranches(arg)
	case "READ_TAGS":
		tags, tErr := s.git.ListTags(arg)
		if tErr == nil {
			result = strings.Join(tags, "\n")
		} else {
			err = tErr
		}
	default:
		return mcpgo.NewToolResultError("Unknown command: " + command), nil
	}

	if err != nil {
		s.sendErrorNotification("git_read", command+" failed", map[string]any{"error": err.Error()})
		return mcpgo.NewToolResultError(command + " failed: " + err.Error()), nil
	}
	s.sendSuccessNotification("git_read", command+" completed", nil)
	return mcpgo.NewToolResultText(result), nil
}

func (s *Server) handleGitWrite(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(params["command"].(string))
	arg := ""
	if v, ok := params["arg"].(string); ok {
		arg = v
	}

	var result string
	var err error

	switch command {
	case "ADD":
		paths := git.SplitPaths(arg)
		err = s.git.Add(paths)
		result = "Files staged"
	case "RM":
		paths := git.SplitPaths(arg)
		err = s.git.Remove(paths)
		result = "Files removed"
	case "SWITCH":
		err = s.git.Switch(arg)
		result = "Switched to " + arg
	case "PUSH":
		result, err = s.git.Push()
	case "PULL":
		result, err = s.git.Pull()
	case "FETCH":
		result, err = s.git.Fetch()
	case "TAG_CREATE":
		_, err = s.git.Tag(arg, "")
		if err == nil {
			s.sendSuccessNotification("tag_create", "Tag created successfully", nil)
			return mcpgo.NewToolResultText(tagResultJSON("created", arg)), nil
		}
	case "TAG_DELETE":
		_, err = s.git.DeleteTag(arg)
		if err == nil {
			s.sendSuccessNotification("tag_delete", "Tag deleted successfully", nil)
			return mcpgo.NewToolResultText(tagResultJSON("deleted", arg)), nil
		}
	case "TAG_PUSH":
		_, err = s.git.PushTag(arg)
		if err == nil {
			s.sendSuccessNotification("tag_push", "Tag pushed successfully", nil)
			return mcpgo.NewToolResultText(tagResultJSON("pushed", arg)), nil
		}
	case "TAG_DELETE_REMOTE":
		_, err = s.git.DeleteTagRemote(arg)
		if err == nil {
			s.sendSuccessNotification("tag_delete_remote", "Tag deleted from remote", nil)
			return mcpgo.NewToolResultText(tagResultJSON("deleted from remote", arg)), nil
		}
	default:
		s.sendErrorNotification("git_write", "Unknown command", map[string]any{"command": command})
		return mcpgo.NewToolResultError("Unknown command: " + command), nil
	}

	if err != nil {
		s.sendErrorNotification("git_write", command+" failed", map[string]any{"error": err.Error()})
		return mcpgo.NewToolResultError(command + " failed: " + err.Error()), nil
	}
	s.sendSuccessNotification("git_write", command+" completed", nil)
	return mcpgo.NewToolResultText(result), nil
}

func (s *Server) handleGitWriteReview(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(params["command"].(string))

	// Handle special metadata commands
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

	// Route specialized operations to their dedicated handlers
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
		// Preview ready - send notification with full structure
		s.sendSuccessNotification(op, op+" ready for review", res.Summary)
		// Return JSON that includes all fields (readyJSON already has preview and status)
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
		s.reviewWorkflow.Abort() // Clean up pending plan after successful apply
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

// --- Backup helpers ---

func (s *Server) applyWithBackup(operation string, stashUntracked bool, fn func() (workflow.Result, error)) (workflow.Result, error) {
	if !s.cfg.Backup.Enabled {
		return fn()
	}

	backup, bErr := s.git.CreateBackup(operation, stashUntracked)
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

// --- Helpers ---

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

// extractExplicitArgs extracts explicit arguments from the MCP request.
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
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			result[key] = fmt.Sprintf("%v", v)
		default:
			result[key] = fmt.Sprintf("%v", v)
		}
	}
	return result
}
