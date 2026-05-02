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
			mcpgo.WithDescription("Read-only git operations. All responses are structured JSON. Commands: READ_STATUS | READ_DIFF | READ_DIFF_STAGED | READ_DIFF_ALL | READ_LOG | READ_BRANCHES | READ_TAGS | CURRENT_BRANCH | IS_REPO | REMOTE_BRANCH_LIST | REMOTE_TAG_LIST"),
			mcpgo.WithString("command", mcpgo.Description("READ_STATUS | READ_DIFF | READ_DIFF_STAGED | READ_DIFF_ALL | READ_LOG | READ_BRANCHES | READ_TAGS | CURRENT_BRANCH | IS_REPO | REMOTE_BRANCH_LIST | REMOTE_TAG_LIST"), mcpgo.Required()),
			mcpgo.WithString("arg", mcpgo.Description("Optional filter. For READ_DIFF/READ_DIFF_ALL: '..branch' (tip-to-tip) or '...branch' (merge-base divergence). For READ_LOG: file paths or 'branch' for range. For READ_BRANCHES/READ_TAGS: glob pattern. For others: ignored.")),
			mcpgo.WithNumber("limit", mcpgo.Description("Max items to return (default: 200 for diff, 20 for log, 50 for others)")),
			mcpgo.WithNumber("offset", mcpgo.Description("Start from this index (0-based, default: 0)")),
			mcpgo.WithString("filter", mcpgo.Description("Regex filter for log messages or path patterns for status files")),
		),
		srv.handleGitRead,
	)

	s.AddTool(
		mcpgo.NewTool("git_write",
			mcpgo.WithDescription("Direct write git operations (no LLM). All responses are structured JSON. Commands: ADD | SWITCH | STASH | STASH_POP | PUSH | PULL | FETCH | RM | RESET_SOFT | RENAME_BRANCH | BRANCH_CREATE | BRANCH_DELETE | REMOTE_BRANCH_DELETE | REMOTE_TAG_DELETE | TAG_CREATE | TAG_DELETE | TAG_PUSH | TAG_DELETE_REMOTE"),
			mcpgo.WithString("command", mcpgo.Description("ADD | SWITCH | STASH | STASH_POP | PUSH | PULL | FETCH | RM | RESET_SOFT | RENAME_BRANCH | BRANCH_CREATE | BRANCH_DELETE | REMOTE_BRANCH_DELETE | REMOTE_TAG_DELETE | TAG_CREATE | TAG_DELETE | TAG_PUSH | TAG_DELETE_REMOTE"), mcpgo.Required()),
			mcpgo.WithString("arg", mcpgo.Description("For RESET_SOFT: commit hash or HEAD~N. For RENAME_BRANCH: old_name:new_name. For BRANCH_CREATE/BRANCH_DELETE/REMOTE_BRANCH_DELETE: branch name. For TAG_*: tag name.")),
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
	command := strings.ToUpper(getStringParam(params, "command", ""))
	arg := getStringParam(params, "arg", "")
	limit, offset := parsePagination(params)
	filter := getStringParam(params, "filter", "")

	// Defaults for limit
	if limit <= 0 {
		switch command {
		case "READ_DIFF", "READ_DIFF_STAGED", "READ_DIFF_ALL":
			limit = 200
		case "READ_LOG":
			limit = 20
		default:
			limit = 50
		}
	}

	var err error
	var result string

	switch command {
	case "READ_STATUS":
		status, err := s.git.Status()
		if err != nil {
			return jsonErrorResult(command, err)
		}
		result = formatStatusJSON(status, limit, offset, filter)

	case "READ_DIFF":
		result, err = s.handleDiffCommand(arg, limit, offset, "")

	case "READ_DIFF_STAGED":
		result, err = s.handleDiffCommand(arg, limit, offset, "--cached")

	case "READ_DIFF_ALL":
		result, err = s.handleDiffAllCommand(arg, limit, offset)

	case "READ_LOG":
		result, err = s.handleLogCommand(arg, limit, offset, filter)

	case "READ_BRANCHES":
		branches, err := s.git.ListBranches(arg)
		if err != nil {
			return jsonErrorResult(command, err)
		}
		current, _ := s.git.CurrentBranch()
		list := SanitizeBranchList(branches)
		if filter != "" {
			list = filterStringSlice(list, filter)
		}
		result = formatBranchListJSON(list, current, limit, offset)

	case "READ_TAGS":
		tags, err := s.git.ListTags(arg)
		if err != nil {
			return jsonErrorResult(command, err)
		}
		if filter != "" {
			tags = filterStringSlice(tags, filter)
		}
		result = formatTagListJSON(tags, limit, offset)

	case "CURRENT_BRANCH":
		branch, err := s.git.CurrentBranch()
		if err != nil {
			return jsonErrorResult(command, err)
		}
		result = fmt.Sprintf(`{"current_branch":%q}`, branch)

	case "IS_REPO":
		isRepo := s.git.IsRepo()
		result = fmt.Sprintf(`{"is_repo":%v}`, isRepo)

	case "REMOTE_BRANCH_LIST":
		branches, err := s.git.ListBranches("-r")
		if err != nil {
			return jsonErrorResult(command, err)
		}
		list := SanitizeBranchList(branches)
		if filter != "" {
			list = filterStringSlice(list, filter)
		}
		result = formatRemoteBranchListJSON(list, limit, offset)

	case "REMOTE_TAG_LIST":
		tags, err := s.git.ListTags()
		if err != nil {
			return jsonErrorResult(command, err)
		}
		if filter != "" {
			tags = filterStringSlice(tags, filter)
		}
		result = formatRemoteTagListJSON(tags, limit, offset)

	default:
		return jsonErrorResult("git_read", fmt.Errorf("unknown command: %s", command))
	}

	if err != nil {
		s.sendErrorNotification("git_read", command+" failed", map[string]any{"error": err.Error()})
		return jsonErrorResult(command, err)
	}

	s.sendSuccessNotification("git_read", command+" completed", nil)
	return mcpgo.NewToolResultText(result), nil
}

// handleDiffCommand handles READ_DIFF, READ_DIFF_STAGED with optional range syntax.
func (s *Server) handleDiffCommand(arg string, limit, offset int, cachedFlag string) (string, error) {
	// Check for range syntax: ..target or ...target
	if strings.HasPrefix(arg, "..") || strings.HasPrefix(arg, "...") {
		current, err := s.git.CurrentBranch()
		if err != nil {
			return "", err
		}
		target := arg
		mode := ""
		if strings.HasPrefix(arg, "...") {
			mode = "..."
			target = strings.TrimPrefix(arg, "...")
		} else {
			mode = ".."
			target = strings.TrimPrefix(arg, "..")
		}
		raw, err := s.git.DiffRange(current, target, mode)
		if err != nil {
			return "", err
		}
		res := SanitizeDiff(raw, offset, limit)
		res.Mode = mode
		res.Base = current
		res.Target = target
		return diffResultJSON(res), nil
	}

	var raw string
	var err error
	if arg != "" {
		if cachedFlag != "" {
			raw, err = s.git.DiffStaged(arg)
		} else {
			raw, err = s.git.Diff(arg)
		}
	} else {
		if cachedFlag != "" {
			raw, err = s.git.DiffStaged()
		} else {
			raw, err = s.git.Diff()
		}
	}
	if err != nil {
		return "", err
	}
	res := SanitizeDiff(raw, offset, limit)
	return diffResultJSON(res), nil
}

// handleDiffAllCommand handles READ_DIFF_ALL (staged + unstaged).
func (s *Server) handleDiffAllCommand(arg string, limit, offset int) (string, error) {
	var raw string
	var err error
	if arg != "" {
		raw, err = s.git.DiffAll(arg)
	} else {
		raw, err = s.git.DiffAll()
	}
	if err != nil {
		return "", err
	}
	res := SanitizeDiff(raw, offset, limit)
	res.Mode = "all"
	return diffResultJSON(res), nil
}

// handleLogCommand handles READ_LOG with optional range syntax.
func (s *Server) handleLogCommand(arg string, limit, offset int, filter string) (string, error) {
	var raw string
	var err error
	if arg != "" {
		raw, err = s.git.Log(50, arg)
	} else {
		raw, err = s.git.Log(50)
	}
	if err != nil {
		return "", err
	}
	res := SanitizeLog(raw, offset, limit)
	if filter != "" {
		res.Commits = filterCommits(res.Commits, filter)
	}
	return logResultJSON(res), nil
}

// jsonErrorResult returns a structured JSON error.
func jsonErrorResult(command string, err error) (*mcpgo.CallToolResult, error) {
	errJSON := fmt.Sprintf(`{"status":"error","command":%q,"error":%q}`, command, err.Error())
	return mcpgo.NewToolResultError(errJSON), nil
}

func (s *Server) handleGitWrite(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(getStringParam(params, "command", ""))
	arg := getStringParam(params, "arg", "")

	var result string
	var err error

	switch command {
	case "ADD":
		paths := git.SplitPaths(arg)
		err = s.git.Add(paths)
		result = writeResultJSON("ADD", err == nil, fmt.Sprintf("%d files staged", len(paths)))
	case "RM":
		paths := git.SplitPaths(arg)
		err = s.git.Remove(paths)
		result = writeResultJSON("RM", err == nil, fmt.Sprintf("%d files removed", len(paths)))
	case "SWITCH":
		err = s.git.Switch(arg)
		result = writeResultJSON("SWITCH", err == nil, fmt.Sprintf("Switched to %s", arg))
	case "PUSH":
		_, err = s.git.Push()
		result = writeResultJSON("PUSH", err == nil, "Pushed with upstream")
	case "PULL":
		_, err = s.git.Pull()
		result = writeResultJSON("PULL", err == nil, "Pulled latest changes")
	case "FETCH":
		_, err = s.git.Fetch()
		result = writeResultJSON("FETCH", err == nil, "Fetched from remote")
	case "RESET_SOFT":
		err = s.git.ResetSoft(arg)
		result = writeResultJSON("RESET_SOFT", err == nil, fmt.Sprintf("Soft reset to %s", arg))
	case "RENAME_BRANCH":
		parts := strings.Split(arg, ":")
		if len(parts) != 2 {
			return jsonErrorResult("RENAME_BRANCH", fmt.Errorf("invalid format: expected old_name:new_name, got: %s", arg))
		}
		_, err = s.git.RenameBranch(parts[0], parts[1])
		result = writeResultJSON("RENAME_BRANCH", err == nil, fmt.Sprintf("Renamed %s to %s", parts[0], parts[1]))
	case "BRANCH_CREATE":
		_, err = s.git.Branch(arg)
		result = writeResultJSON("BRANCH_CREATE", err == nil, fmt.Sprintf("Created branch %s", arg))
	case "BRANCH_DELETE":
		_, err = s.git.DeleteBranch(arg)
		result = writeResultJSON("BRANCH_DELETE", err == nil, fmt.Sprintf("Deleted branch %s", arg))
	case "REMOTE_BRANCH_DELETE":
		err = s.git.DeleteRemoteBranch(arg)
		result = writeResultJSON("REMOTE_BRANCH_DELETE", err == nil, fmt.Sprintf("Deleted remote branch %s", arg))
	case "REMOTE_TAG_DELETE":
		err = s.git.DeleteRemoteTag(arg)
		result = writeResultJSON("REMOTE_TAG_DELETE", err == nil, fmt.Sprintf("Deleted remote tag %s", arg))
	case "TAG_CREATE":
		_, err = s.git.Tag(arg, "")
		result = tagResultJSON("created", arg)
	case "TAG_DELETE":
		_, err = s.git.DeleteTag(arg)
		result = tagResultJSON("deleted", arg)
	case "TAG_PUSH":
		_, err = s.git.PushTag(arg)
		result = tagResultJSON("pushed", arg)
	case "TAG_DELETE_REMOTE":
		_, err = s.git.DeleteTagRemote(arg)
		result = tagResultJSON("deleted from remote", arg)
	case "STASH":
		_, err = s.git.Stash()
		result = writeResultJSON("STASH", err == nil, "Changes stashed")
	case "STASH_POP":
		_, err = s.git.StashPop()
		result = writeResultJSON("STASH_POP", err == nil, "Stash restored")
	default:
		return jsonErrorResult("git_write", fmt.Errorf("unknown command: %s", command))
	}

	if err != nil {
		s.sendErrorNotification("git_write", command+" failed", map[string]any{"error": err.Error()})
		return jsonErrorResult(command, err)
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

// parsePagination extracts limit and offset from MCP request params.
func parsePagination(params map[string]any) (limit, offset int) {
	if v, ok := params["limit"].(float64); ok {
		limit = int(v)
	}
	if v, ok := params["offset"].(float64); ok {
		offset = int(v)
	}
	return
}

// getStringParam extracts a string param with default fallback.
func getStringParam(params map[string]any, key, def string) string {
	if v, ok := params[key].(string); ok && v != "" {
		return v
	}
	return def
}
