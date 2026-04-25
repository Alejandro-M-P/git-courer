package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// startKeepalive sends notifications during long blocking operations.
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
					"data":   operation + "...",
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
		"message":   message,
		"data":      details,
	})
}

// sendSuccessNotification sends a structured success notification to all clients.
func (s *Server) sendSuccessNotification(operation, message string, details map[string]any) {
	s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
		"level":     "info",
		"logger":    "git-courer",
		"operation": operation,
		"message":   message,
		"data":      details,
	})
}

// sendSecurityErrorNotification sends a structured error notification for security blocks.
func (s *Server) sendSecurityErrorNotification(errMsg string) {
	var detections []map[string]string
	lines := strings.Split(errMsg, "\n")
	for _, line := range lines {
		if strings.Contains(line, "- ") {
			trimmed := strings.TrimPrefix(line, "  - ")
			parts := strings.Split(trimmed, " - ")
			if len(parts) >= 2 {
				location := parts[0]
				rest := parts[1]
				typeMsg := strings.Split(rest, " (")
				detections = append(detections, map[string]string{
					"location": location,
					"type":     strings.TrimSuffix(typeMsg[0], ")"),
				})
			}
		}
	}
	s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
		"level":     "error",
		"logger":    "git-courer",
		"operation": "commit",
		"error":     "SECRET_DETECTED",
		"message":   "Commit blocked: potential secret detected",
		"data": map[string]any{
			"detections": detections,
			"hint":       "Remove or review the detected secrets before committing",
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

// handleGitRead routes read-only git operations directly to the git adapter.
func (s *Server) handleGitRead(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	command := req.GetString("command", "")
	if command == "" {
		s.sendErrorNotification("git_read", "command is required", map[string]any{})
		return mcpgo.NewToolResultError("command is required"), nil
	}

	arg := req.GetString("arg", "")
	paths := strings.Fields(arg)

	var result string
	var err error

	switch command {
	case "READ_STATUS":
		status, sErr := s.git.Status()
		if sErr != nil {
			s.sendErrorNotification("git_read", "status failed", map[string]any{"error": sErr.Error()})
			return mcpgo.NewToolResultError("status failed: " + sErr.Error()), nil
		}
		result = formatStatus(status)
	case "READ_DIFF":
		result, err = s.git.Diff(paths...)
	case "READ_DIFF_STAGED":
		result, err = s.git.DiffStaged(paths...)
	case "READ_LOG":
		result, err = s.git.Log(20, paths...)
	case "READ_BRANCHES":
		current, _ := s.git.CurrentBranch()
		var branches string
		if arg != "" {
			branches, err = s.git.ListBranches(arg)
		} else {
			branches, err = s.git.ListBranches()
		}
		if err != nil {
			s.sendErrorNotification("git_read", "branches failed", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError("branches failed: " + err.Error()), nil
		}
		result = "Current: " + current + "\n\n" + branches
	case "READ_TAGS":
		var tags []string
		if arg != "" {
			tags, err = s.git.ListTags(arg)
		} else {
			tags, err = s.git.ListTags()
		}
		if err != nil {
			s.sendErrorNotification("git_read", "tags failed", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError("tags failed: " + err.Error()), nil
		}
		result = strings.Join(tags, "\n")
	default:
		s.sendErrorNotification("git_read", "Unknown command", map[string]any{"command": command})
		return mcpgo.NewToolResultError("Unknown command: " + command), nil
	}

	if err != nil {
		s.sendErrorNotification("git_read", command+" failed", map[string]any{"error": err.Error()})
		return mcpgo.NewToolResultError(command + " failed: " + err.Error()), nil
	}
	s.sendSuccessNotification("git_read", command+" completed", map[string]any{"result": result})
	return mcpgo.NewToolResultText(result), nil
}

// handleGitWrite routes direct write operations to the git adapter (no LLM).
func (s *Server) handleGitWrite(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	command := req.GetString("command", "")
	if command == "" {
		s.sendErrorNotification("git_write", "command is required", map[string]any{})
		return mcpgo.NewToolResultError("command is required"), nil
	}
	arg := req.GetString("arg", "")

	var result string
	var err error

	switch command {
	case "ADD":
		paths := []string{"."}
		if arg != "" {
			paths = strings.Fields(arg)
		}
		err = s.git.Add(paths)
		result = "Files added"
	case "RM":
		err = s.git.Remove(strings.Fields(arg))
		result = "Files removed"
	case "SWITCH":
		err = s.git.Switch(arg)
		result = "Switched to: " + arg
	case "STASH":
		result, err = s.git.Stash()
		if result == "" {
			result = "Changes stashed"
		}
	case "STASH_POP":
		result, err = s.git.StashPop()
		if result == "" {
			result = "Stashed changes restored"
		}
	case "PUSH":
		result, err = s.git.Push()
	case "PULL":
		result, err = s.git.Pull()
	case "FETCH":
		result, err = s.git.Fetch()
	case "TAG_CREATE":
		_, err = s.git.Tag(arg)
		if err == nil {
			s.sendSuccessNotification("tag_create", "Tag created successfully", map[string]any{"tag": arg})
			return mcpgo.NewToolResultText(tagResultJSON("created", arg)), nil
		}
	case "TAG_DELETE":
		_, err = s.git.DeleteTag(arg)
		if err == nil {
			s.sendSuccessNotification("tag_delete", "Tag deleted successfully", map[string]any{"tag": arg})
			return mcpgo.NewToolResultText(tagResultJSON("deleted", arg)), nil
		}
	case "TAG_PUSH":
		_, err = s.git.PushTag(arg)
		if err == nil {
			s.sendSuccessNotification("tag_push", "Tag pushed successfully", map[string]any{"tag": arg})
			return mcpgo.NewToolResultText(tagResultJSON("pushed", arg)), nil
		}
	case "TAG_DELETE_REMOTE":
		_, err = s.git.DeleteTagRemote(arg)
		if err == nil {
			s.sendSuccessNotification("tag_delete_remote", "Tag deleted from remote", map[string]any{"tag": arg})
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
	s.sendSuccessNotification("git_write", command+" completed", map[string]any{"result": result})
	return mcpgo.NewToolResultText(result), nil
}

// handleGitWriteReview handles review operations using the three-phase workflow protocol.
func (s *Server) handleGitWriteReview(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	command := req.GetString("command", "")
	if command == "" {
		return mcpgo.NewToolResultError("command is required"), nil
	}

	// Utility commands (no phase)
	switch command {
	case "STATUS":
		plan, err := s.reviewWorkflow.PlanStatus()
		if err != nil {
			s.sendErrorNotification("status", "status failed", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError("status failed: " + err.Error()), nil
		}
		s.sendSuccessNotification("status", "Status retrieved", map[string]any{"plan": plan})
		return mcpgo.NewToolResultText(plan), nil
	case "SUMMARY":
		status, err := s.git.Status()
		if err != nil {
			s.sendErrorNotification("summary", "status failed", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError("status failed: " + err.Error()), nil
		}
		s.sendSuccessNotification("summary", "Git summary retrieved", map[string]any{"status": formatStatus(status)})
		return mcpgo.NewToolResultText(formatStatus(status)), nil
	}

	// Parse phase from command suffix
	op, phase := parseCommand(command)

	// Special case: commit uses CommitService
	if op == "commit" {
		return s.handleCommitOperation(ctx, req, phase)
	}

	// Special case: release uses ReleaseService
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
			s.sendSuccessNotification(op, op+" completed", map[string]any{"result": res.Output})
			return mcpgo.NewToolResultText(res.Output), nil
		}
		// Preview ready - send notification with full structure
		details := map[string]any{
			"status":  res.Status,
			"preview": res.Preview,
			"hint":    "Show the user the preview before confirming. To execute: " + strings.ToUpper(op) + "_APPLY. To cancel: " + strings.ToUpper(op) + "_ABORT.",
		}
		if len(res.Args) > 0 {
			details["args"] = res.Args
		}
		s.sendSuccessNotification(op, op+" ready for review", details)
		// Return JSON that includes all fields (readyJSON already has preview and status)
		return mcpgo.NewToolResultText(readyJSON(res.Preview)), nil

	case "apply":
		if !s.reviewWorkflow.HasPendingPlan() {
			s.sendErrorNotification(op, "No pending operation", map[string]any{"hint": "Run " + strings.ToUpper(op) + "_START first"})
			return mcpgo.NewToolResultError("No pending " + op + " operation. Run " + strings.ToUpper(op) + "_START first."), nil
		}
		keepalive := s.startKeepalive("Applying "+op, 10*time.Second)
		res, err := s.applyWithBackup(op, false, func() (string, error) {
			r, applyErr := s.reviewWorkflow.Apply(context.Background())
			if applyErr != nil {
				return "", applyErr
			}
			return r.Output, nil
		})
		close(keepalive)
		if err != nil {
			s.sendErrorNotification(op, op+" failed", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError(err.Error()), nil
		}
		s.sendSuccessNotification(op, op+" completed successfully", map[string]any{"result": res})
		return mcpgo.NewToolResultText(res), nil

	case "abort":
		if err := s.reviewWorkflow.Abort(); err != nil {
			s.sendErrorNotification(op, "abort failed", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError("abort failed: " + err.Error()), nil
		}
		s.sendSuccessNotification(op, op+" aborted", map[string]any{})
		return mcpgo.NewToolResultText("Operation aborted"), nil

	default:
		return mcpgo.NewToolResultError("Unknown command: " + command + ". Use {OP}_START, {OP}_APPLY, or {OP}_ABORT"), nil
	}
}

// handleCommitOperation handles commit operations using CommitService.
func (s *Server) handleCommitOperation(_ context.Context, req mcpgo.CallToolRequest, phase string) (*mcpgo.CallToolResult, error) {
	requireConfirmation := s.cfg.Validation.RequireConfirmation
	preview := req.GetBool("preview", requireConfirmation)

	switch phase {
	case "start":
		instruction := req.GetString("instruction", "")

		// Check if there's an existing plan that's still valid (not rejected)
		if s.commitConfirm.HasBlocker() {
			if existingPlan, _ := s.commitConfirm.ReadPlan(); existingPlan != nil && existingPlan.RejectedMessage == "" {
				// Existing plan exists and wasn't rejected - return it directly
				log.Printf("[DEBUG] handleCommitOperation: returning existing plan with %d messages", len(existingPlan.Messages))
				return mcpgo.NewToolResultText(readyJSON(existingPlan.Preview)), nil
			}
		}

		if preview {
			var rejectedMessage string
			if s.commitConfirm.HasBlocker() {
				if existingPlan, _ := s.commitConfirm.ReadPlan(); existingPlan != nil {
					rejectedMessage = existingPlan.RejectedMessage
					if rejectedMessage == "" && len(existingPlan.Messages) > 0 {
						rejectedMessage = existingPlan.Messages[0]
					}
				}
			}
			s.llm.SetRetryContext(rejectedMessage)

			s.commitConfirm.RemoveBlocker()

			// Block synchronously with keepalive notifications
			keepalive := s.startKeepalive("Preparing commit", 10*time.Second)
			messages, chunks, deleted, _, reasoning, err := s.commitSvc.PrepareCommit(instruction)
			close(keepalive)

			if err != nil {
				if strings.Contains(err.Error(), "[SECURITY]") {
					s.sendSecurityErrorNotification(err.Error())
				}
				s.sendErrorNotification("commit", "Failed to prepare commit", map[string]any{"error": err.Error()})
				return mcpgo.NewToolResultError("Failed to prepare commit: " + err.Error()), nil
			}

			chunkFiles := make([][]string, len(chunks))
			for i, c := range chunks {
				chunkFiles[i] = c.Files
			}
			plan := domain.OperationPlan{
				Operation:    "commit",
				Messages:     messages,
				Chunks:       chunkFiles,
				DeletedFiles: deleted,
				Instruction:  instruction,
				Reasoning:    reasoning,
				Preview:      strings.Join(messages, "\n"),
			}

			// Send preview via notification
			s.sendSuccessNotification("commit", "Commit plan ready for review", map[string]any{
				"status":    "pending_approval",
				"messages":  messages,
				"files":     gatherFilesFromChunks(chunkFiles),
				"reasoning": reasoning,
				"hint":      "Show the user the commit messages, affected files, and reasoning before confirming. To execute: COMMIT_APPLY. To cancel: COMMIT_ABORT.",
			})

			if err := s.commitConfirm.WritePlan(plan); err != nil {
				s.sendErrorNotification("commit", "Failed to save plan", map[string]any{"error": err.Error()})
				return mcpgo.NewToolResultError("Failed to save plan: " + err.Error()), nil
			}
			if err := s.commitConfirm.CreateBlocker(); err != nil {
				log.Printf("[DEBUG] COMMIT_START: CreateBlocker error: %v", err)
			}
			log.Printf("[DEBUG] COMMIT_START: plan written, blocker created, HasBlocker=%v", s.commitConfirm.HasBlocker())

			return mcpgo.NewToolResultText(commitPlanJSON(&plan)), nil
		}

		// Non-preview: execute directly with keepalive
		keepalive := s.startKeepalive("Running commit", 10*time.Second)
		result, err := s.commitSvc.Execute(instruction, false)
		close(keepalive)

		if err != nil {
			if strings.Contains(err.Error(), "[SECURITY]") {
				s.sendSecurityErrorNotification(err.Error())
			}
			s.sendErrorNotification("commit", "Commit failed", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError(err.Error()), nil
		}

		s.sendSuccessNotification("commit", "Commit completed successfully", map[string]any{"result": result})
		return mcpgo.NewToolResultText(result), nil

	case "apply":
		log.Printf("[DEBUG] COMMIT_APPLY: HasBlocker=%v", s.commitConfirm.HasBlocker())
		if !s.commitConfirm.HasBlocker() {
			s.sendErrorNotification("commit", "No pending commit plan", map[string]any{"hint": "Run COMMIT_START first"})
			return mcpgo.NewToolResultError("No active commit plan. Run COMMIT_START first."), nil
		}

		plan, err := s.commitConfirm.ReadPlan()
		log.Printf("[DEBUG] COMMIT_APPLY: ReadPlan plan=%v, err=%v", plan != nil, err)
		if err != nil || plan == nil {
			s.commitConfirm.RemoveBlocker()
			errMsg := "plan file not found"
			if err != nil {
				errMsg = err.Error()
			}
			s.sendErrorNotification("commit", "Failed to read plan", map[string]any{"error": errMsg})
			return mcpgo.NewToolResultError("Failed to read plan. Run COMMIT_START again."), nil
		}

		instruction := req.GetString("instruction", "")

		if instruction != "" {
			keepalive := s.startKeepalive("Running commit", 10*time.Second)
			result, err := s.applyWithBackup("commit", true, func() (string, error) {
				return s.commitSvc.Execute(instruction, false)
			})
			close(keepalive)
			if err != nil {
				s.commitConfirm.RemoveBlocker()
				if strings.Contains(err.Error(), "[SECURITY]") {
					s.sendSecurityErrorNotification(err.Error())
				}
				s.sendErrorNotification("commit", "Commit failed", map[string]any{"error": err.Error()})
				return mcpgo.NewToolResultError(err.Error()), nil
			}
			s.commitConfirm.DeletePlan()
			s.llm.ClearRetryContext()
			s.sendSuccessNotification("commit", "Commit completed successfully", map[string]any{"result": result})
			return mcpgo.NewToolResultText(result), nil
		}

		keepalive := s.startKeepalive("Running commit", 10*time.Second)
		log.Printf("[DEBUG] handlers: about to call ExecuteFromPlan")
		result, err := s.commitSvc.ExecuteFromPlan(plan.Messages, plan.Chunks, plan.DeletedFiles, plan.Instruction)
		log.Printf("[DEBUG] handlers: ExecuteFromPlan returned, err=%v", err)
		close(keepalive)

		if err != nil {
			s.commitConfirm.RemoveBlocker()
			if strings.Contains(err.Error(), "[SECURITY]") {
				s.sendSecurityErrorNotification(err.Error())
			}
			s.sendErrorNotification("commit", "Commit failed", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError(err.Error()), nil
		}

		log.Printf("[DEBUG] handlers: about to DeletePlan")
		s.commitConfirm.DeletePlan()
		log.Printf("[DEBUG] handlers: DeletePlan done, about to ClearRetryContext")
		s.llm.ClearRetryContext()
		log.Printf("[DEBUG] handlers: about to sendSuccessNotification")
		s.sendSuccessNotification("commit", "Commit completed successfully", map[string]any{"result": result})
		log.Printf("[DEBUG] handlers: about to return")
		return mcpgo.NewToolResultText(result), nil

	case "abort":
		s.commitConfirm.DeletePlan()
		s.llm.ClearRetryContext()
		s.sendSuccessNotification("commit", "Commit plan aborted", map[string]any{})
		return mcpgo.NewToolResultText("Commit plan aborted."), nil

	case "regenerate":
		if !s.commitConfirm.HasBlocker() {
			s.sendErrorNotification("commit", "No pending commit plan", map[string]any{"hint": "Run COMMIT_START first"})
			return mcpgo.NewToolResultError("No active commit plan. Run COMMIT_START first."), nil
		}

		plan, err := s.commitConfirm.ReadPlan()
		if err != nil || plan == nil {
			s.commitConfirm.RemoveBlocker()
			errMsg := "plan file not found"
			if err != nil {
				errMsg = err.Error()
			}
			s.sendErrorNotification("commit", "Failed to read plan", map[string]any{"error": errMsg})
			return mcpgo.NewToolResultError("Failed to read plan. Run COMMIT_START again."), nil
		}

		feedback := req.GetString("feedback", "")
		if feedback == "" {
			feedback = "Improve the commit messages"
		}

		// Convert chunk files back to DiffChunks for regeneration
		chunks := make([]domain.DiffChunk, len(plan.Chunks))
		for i, files := range plan.Chunks {
			chunks[i] = domain.DiffChunk{
				Files: files,
				Diff:  "", // We don't have the diff stored, but RegenerateMessage can work without it
			}
		}

		keepalive := s.startKeepalive("Regenerating commit messages", 10*time.Second)
		newMessages, err := s.llm.RegenerateMessage(plan.Messages, feedback, chunks)
		close(keepalive)

		if err != nil {
			s.sendErrorNotification("commit", "Failed to regenerate messages", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError("Failed to regenerate messages: " + err.Error()), nil
		}

		// Update the plan with new messages
		plan.Messages = newMessages
		plan.Preview = strings.Join(newMessages, "\n")
		
		if err := s.commitConfirm.WritePlan(*plan); err != nil {
			s.sendErrorNotification("commit", "Failed to update plan", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError("Failed to update plan: " + err.Error()), nil
		}

		s.sendSuccessNotification("commit", "Commit messages regenerated", map[string]any{"messages": newMessages, "feedback": feedback})
		return mcpgo.NewToolResultText(commitPlanJSON(plan)), nil

	default:
		return mcpgo.NewToolResultError("Unknown commit phase: " + phase + ". Use COMMIT_START, COMMIT_APPLY, or COMMIT_ABORT"), nil
	}
}

// handleRelease handles release operations using ReleaseService.
func (s *Server) handleRelease(_ context.Context, req mcpgo.CallToolRequest, phase string) (*mcpgo.CallToolResult, error) {
	switch phase {
	case "start":
		instruction := req.GetString("instruction", "")
		if instruction == "" {
			instruction = "sacar versión"
		}

		keepalive := s.startKeepalive("Preparing release", 30*time.Second)
		s.releaseSvc.ClearPending()

		intent, commits, warnings, err := s.releaseSvc.Prepare(instruction, "")
		close(keepalive)
		if err != nil {
			s.sendErrorNotification("release", "Failed to prepare release", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError("Failed to prepare release: " + err.Error()), nil
		}

		keepalive = s.startKeepalive("Generating changelog", 30*time.Second)
		changelog, warningsGen, err := s.releaseSvc.Generate(commits)
		close(keepalive)
		if err != nil {
			s.sendErrorNotification("release", "Failed to generate changelog", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError("Failed to generate changelog: " + err.Error()), nil
		}

		s.releaseSvc.SaveIntent(intent)
		s.releaseSvc.SaveChangelog(changelog)

		allWarnings := append(warnings, warningsGen...)
		s.releaseConfirm.CreateBlocker()

		authenticated, _ := s.git.IsGHAuthenticated()
		ghStatus := "authenticated"
		if !authenticated {
			ghStatus = "not authenticated (will create tag only)"
		}

		s.sendSuccessNotification("release", "Release plan ready for review", map[string]any{
			"status":        "pending_approval",
			"tag_name":      intent.TagName,
			"version":      intent.VersionBump,
			"changelog":     changelog,
			"github_auth":   ghStatus,
			"warnings":      allWarnings,
				"hint":          "Show the user the tag name, version bump, changelog, and GitHub auth status before confirming. To execute: RELEASE_APPLY. To cancel: RELEASE_ABORT.",
		})

		return mcpgo.NewToolResultText(releasePlanJSON(intent, changelog, allWarnings, ghStatus)), nil

	case "apply":
		if !s.releaseConfirm.HasBlocker() {
			s.sendErrorNotification("release", "No pending release plan", map[string]any{"hint": "Run RELEASE_START first"})
			return mcpgo.NewToolResultError("No pending release. Run RELEASE_START first."), nil
		}

		intent, err := s.releaseSvc.LoadIntent()
		if err != nil {
			s.releaseConfirm.RemoveBlocker()
			s.sendErrorNotification("release", "Failed to load intent", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError("Failed to load intent: " + err.Error()), nil
		}

		changelog, err := s.releaseSvc.LoadChangelog()
		if err != nil {
			s.releaseConfirm.RemoveBlocker()
			s.sendErrorNotification("release", "Failed to load changelog", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError("Failed to load changelog: " + err.Error()), nil
		}

		createGitHubRelease := req.GetBool("create_github_release", true)

		if createGitHubRelease {
			authenticated, _ := s.git.IsGHAuthenticated()
			if !authenticated {
				s.sendErrorNotification("release", "GitHub CLI not authenticated", map[string]any{
					"hint": "Run 'gh auth login' first, or call RELEASE_APPLY with create_github_release: false to create only the tag",
				})
				return mcpgo.NewToolResultError("GitHub CLI not authenticated. Run 'gh auth login' first."), nil
			}
		}

		tagResult, err := s.applyWithBackup("release", false, func() (string, error) {
			return s.releaseSvc.Execute(intent, changelog, createGitHubRelease)
		})
		if err != nil {
			return mcpgo.NewToolResultError(err.Error()), nil
		}

		s.releaseSvc.ClearPending()

		return mcpgo.NewToolResultText(fmt.Sprintf("Release created: %s", tagResult)), nil

	case "abort":
		s.releaseConfirm.RemoveBlocker()
		s.releaseSvc.ClearPending()
		s.sendSuccessNotification("release", "Release cancelled", map[string]any{})
		return mcpgo.NewToolResultText("Release cancelled"), nil

	default:
		return mcpgo.NewToolResultError("Unknown release phase: " + phase + ". Use START, APPLY, or ABORT."), nil
	}
}

// --- Backup helpers ---

// applyWithBackup wraps a destructive operation with automatic backup and restore.
//   - Before executing: creates a git ref + optional stash (copia de seguridad).
//   - On success: deletes the backup (no saturar el repo).
//   - On failure: auto-restores to the pre-operation state and notifies the caller.
//
// keepIndex=true stashes only unstaged changes, leaving staged files intact (for commit).
// If backup is disabled in config, fn runs directly without any backup logic.
func (s *Server) applyWithBackup(operation string, keepIndex bool, fn func() (string, error)) (string, error) {
	if !s.cfg.Backup.Enabled {
		return fn()
	}

	backup, bErr := s.git.CreateBackup(operation, keepIndex)
	if bErr != nil {
		log.Printf("⚠ backup creation failed for %s: %v — proceeding without backup", operation, bErr)
		return fn()
	}

	result, fnErr := fn()
	if fnErr != nil {
		// Operation failed — auto-restore and notify
		if rErr := s.git.RestoreBackup(backup); rErr != nil {
			if dErr := s.git.DeleteBackup(backup); dErr != nil {
				log.Printf("⚠ could not delete backup ref %s after restore failure: %v", backup.Ref, dErr)
			}
			return "", fmt.Errorf(
				"la operación '%s' falló y la restauración automática también falló.\n"+
					"  Error original: %v\n"+
					"  Error de restauración: %v\n"+
					"  Restauración manual: git reset --hard %s",
				operation, fnErr, rErr, backup.Ref,
			)
		}
		if dErr := s.git.DeleteBackup(backup); dErr != nil {
			log.Printf("⚠ could not delete backup ref %s after failed operation: %v", backup.Ref, dErr)
		}
		return "", fmt.Errorf(
			"⚠ la operación '%s' falló — el repo fue restaurado automáticamente al estado anterior.\n  Error: %v",
			operation, fnErr,
		)
	}

	// Success — remove the backup to keep the repo clean
	if err := s.git.DeleteBackup(backup); err != nil {
		log.Printf("⚠ could not delete backup ref %s: %v", backup.Ref, err)
	}
	return result, nil
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
	op = strings.ToLower(command)
	phase = "unknown"
	return
}

func extractExplicitArgs(req mcpgo.CallToolRequest) map[string]string {
	args := make(map[string]string)
	for _, key := range []string{"branch", "arg"} {
		if v := req.GetString(key, ""); v != "" {
			args[key] = v
		}
	}
	if v, ok := args["arg"]; ok {
		args["url"] = v
		args["name"] = v
	}
	return args
}

func processingJSON(message string) string {
	resp, _ := json.Marshal(map[string]interface{}{
		"status":        "pending_approval",
		"show_to_user":  "IMPORTANT: Display ALL fields below to the user before asking for confirmation. Do not summarize.",
		"preview":       message,
	})
	return string(resp)
}

func readyJSON(preview string) string {
	resp, _ := json.Marshal(map[string]interface{}{
		"status":        "pending_approval",
		"show_to_user":  "IMPORTANT: Display ALL fields below to the user before asking for confirmation. Do not summarize.",
		"preview":       preview,
	})
	return string(resp)
}

func commitPlanJSON(plan *domain.OperationPlan) string {
	seen := make(map[string]bool)
	var allFiles []string
	for _, files := range plan.Chunks {
		for _, f := range files {
			if !seen[f] {
				seen[f] = true
				allFiles = append(allFiles, f)
			}
		}
	}
	options := []string{
		"Execute",
		"Regenerate message",
		"Edit manually",
		"Cancel",
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"status":        "pending_approval",
		"show_to_user":  "IMPORTANT: Display ALL fields below to the user before asking for confirmation. Do NOT summarize.",
		"messages":      plan.Messages,
		"files":         allFiles,
		"reasoning":     plan.Reasoning,
		"preview":       plan.Preview,
		"options":       options,
	})
	return string(resp)
}

func releasePlanJSON(intent *domain.ReleaseIntent, changelog string, warnings []string, ghAuth string) string {
	resp, _ := json.Marshal(map[string]interface{}{
		"status":        "pending_approval",
		"show_to_user":  "IMPORTANT: Display ALL fields below to the user before asking for confirmation. Do not summarize.",
		"tag_name":      intent.TagName,
		"version":       intent.VersionBump,
		"changelog":     changelog,
		"github_auth":   ghAuth,
		"warnings":      warnings,
		"hint":          "Show the user the tag name, version bump, changelog, and GitHub auth status before confirming. To execute: RELEASE_APPLY. To cancel: RELEASE_ABORT.",
	})
	return string(resp)
}

func gatherFilesFromChunks(chunks [][]string) []string {
	seen := make(map[string]bool)
	var files []string
	for _, chunk := range chunks {
		for _, f := range chunk {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
	}
	return files
}

// formatStatus formats domain.Status as a human-readable string.
func tagResultJSON(action, tag string) string {
	resp, _ := json.Marshal(map[string]interface{}{
		"status":        "completed",
		"show_to_user":  "IMPORTANT: Display this to the user. Do not summarize.",
		"action":        action,
		"tag":           tag,
	})
	return string(resp)
}

func formatStatus(status domain.Status) string {
	if status.IsClean {
		return fmt.Sprintf("Branch: %s\nWorking tree clean", status.Branch)
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Branch: %s\n", status.Branch))
	b.WriteString(fmt.Sprintf("Staged: %d  Modified: %d  Untracked: %d\n\n", status.Staged, status.Modified, status.Untracked))
	for _, f := range status.Files {
		b.WriteString(fmt.Sprintf("  %s  %s\n", f.Status, f.Path))
	}
	return b.String()
}
