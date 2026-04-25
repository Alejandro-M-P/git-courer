package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
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
func (s *Server) sendSuccessNotification(operation, message string, summary *workflow.Summary) {
	data := map[string]any{
		"operation": operation,
		"message":   "✅ Success: " + message,
	}
	if summary != nil {
		data["summary"] = summary
	}
	s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
		"level":  "info",
		"logger": "git-courer",
		"data":   data,
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
			mcpgo.WithString("command", mcpgo.Description("e.g. COMMIT_START | COMMIT_APPLY | BRANCH_CREATE_START | BRANCH_CREATE_APPLY | BRANCH_DELETE_START | MERGE_START"), mcpgo.Required()),
			mcpgo.WithString("instruction", mcpgo.Description("Natural language instruction for START phase")),
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
		paths := strings.Split(arg, ",")
		err = s.git.Add(paths)
		result = "Files staged"
	case "RM":
		paths := strings.Split(arg, ",")
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
		_, err = s.git.Tag(arg)
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
	instruction := ""
	if v, ok := params["instruction"].(string); ok {
		instruction = v
	}

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

	switch phase {
	case "START":
		keep := s.startKeepalive(op, 3*time.Second)
		res, err := s.reviewWorkflow.Run(ctx, op, instruction, nil)
		close(keep)
		if err != nil {
			s.sendErrorNotification(op, "Generation failed", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError(err.Error()), nil
		}
		if res.Status == "blocked" {
			s.sendSecurityErrorNotification(res.Preview)
			return mcpgo.NewToolResultText(res.Preview), nil
		}
		s.sendSuccessNotification(op, "Plan ready for review", res.Summary)
		return mcpgo.NewToolResultText(res.Preview + "\n\n⚠️  ACTION REQUIRED: Please show the preview above to the user for final validation.\n- To proceed: call " + strings.ToUpper(op) + "_APPLY\n- To cancel: call " + strings.ToUpper(op) + "_ABORT"), nil

	case "APPLY":
		keep := s.startKeepalive(op, 2*time.Second)
		res, err := s.reviewWorkflow.Apply(ctx)
		close(keep)
		if err != nil {
			s.sendErrorNotification(op, "Execution failed", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError(err.Error()), nil
		}
		s.sendSuccessNotification(op, "Operation completed", res.Summary)
		return mcpgo.NewToolResultText(res.Output), nil

	case "ABORT":
		err := s.reviewWorkflow.Abort()
		if err != nil {
			return mcpgo.NewToolResultError(err.Error()), nil
		}
		s.sendSuccessNotification(op, "Plan aborted and repository reset", nil)
		return mcpgo.NewToolResultText("Operation cancelled. State restored."), nil

	default:
		return mcpgo.NewToolResultError("Unknown phase: " + phase), nil
	}
}

func parseCommand(command string) (string, string) {
	parts := strings.Split(command, "_")
	if len(parts) < 2 {
		return "", ""
	}
	return strings.ToLower(parts[0]), strings.ToUpper(parts[1])
}

func processingJSON(msg string) string {
	return fmt.Sprintf(`{"status": "processing", "message": %q}`, msg)
}

func formatStatus(s domain.Status) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Branch: %s\n", s.Branch))
	if len(s.Files) == 0 {
		sb.WriteString("Working tree clean\n")
		return sb.String()
	}
	for _, f := range s.Files {
		sb.WriteString(fmt.Sprintf("%s %s\n", f.Status, f.Path))
	}
	return sb.String()
}

func tagResultJSON(op, tag string) string {
	return fmt.Sprintf(`{"operation": "tag_%s", "tag": %q, "status": "success"}`, op, tag)
}
