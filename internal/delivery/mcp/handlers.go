package mcp

import (
	"context"
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
			s.sendErrorNotification(op, "Execution failed", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError(err.Error()), nil
		}
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
				return mcpgo.NewToolResultText(commitPlanJSON(existingPlan)), nil
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
			var files []string
			for i, c := range chunks {
				chunkFiles[i] = c.Files
				files = append(files, c.Files...)
			}
			files = append(files, deleted...)

			plan := domain.OperationPlan{
				Operation:    "commit",
				Messages:     messages,
				Files:        files,
				Chunks:       chunkFiles,
				DeletedFiles: deleted,
				Instruction:  instruction,
				Reasoning:    reasoning,
				Preview:      strings.Join(messages, "\n"),
			}

			// Send preview via notification
			s.sendSuccessNotification("commit", "Commit plan ready for review", &workflow.Summary{
				Operation:     "commit",
				FilesAffected: files,
				Impact:        "Medium",
				SecurityCheck: "Passed",
				Message:       "Commit plan ready for review",
				Reasoning:     reasoning,
				Messages:      messages,
			})

			if err := s.commitConfirm.WritePlan(plan); err != nil {
				s.sendErrorNotification("commit", "Failed to save plan", map[string]any{"error": err.Error()})
				return mcpgo.NewToolResultError("Failed to save plan: " + err.Error()), nil
			}
			if err := s.commitConfirm.CreateBlocker(); err != nil {
				log.Printf("[DEBUG] COMMIT_START: CreateBlocker error: %v", err)
			}

			return mcpgo.NewToolResultText(commitPlanJSON(&plan)), nil
		}

		// Non-preview: execute directly with keepalive
		keepalive := s.startKeepalive("Running commit", 10*time.Second)
		result, err := s.commitSvc.Execute(instruction, false)
		close(keepalive)
		if err != nil {
			s.sendErrorNotification("commit", "Commit execution failed", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError("Commit execution failed: " + err.Error()), nil
		}
		s.sendSuccessNotification("commit", "Commit completed successfully", nil)
		return mcpgo.NewToolResultText(result), nil

	case "apply":
		// For commit operations, use commit service
		if !s.commitConfirm.HasBlocker() {
			s.sendErrorNotification("commit", "No pending commit plan", map[string]any{"hint": "Run COMMIT_START first"})
			return mcpgo.NewToolResultError("No active commit plan. Run COMMIT_START first."), nil
		}
		plan, err := s.commitConfirm.ReadPlan()
		if err != nil || plan == nil {
			s.commitConfirm.RemoveBlocker()
			s.sendErrorNotification("commit", "Failed to load plan", map[string]any{"error": "Plan expired or missing"})
			return mcpgo.NewToolResultError("Failed to load plan. Run COMMIT_START again."), nil
		}
		keepalive := s.startKeepalive("Executing commit", 10*time.Second)
		result, err := s.commitSvc.ExecuteFromPlan(plan.Messages, plan.Chunks, plan.DeletedFiles, plan.Instruction)
		close(keepalive)
		if err != nil {
			s.sendErrorNotification("commit", "Commit execution failed", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError("Commit execution failed: " + err.Error()), nil
		}
		s.sendSuccessNotification("commit", "Commit completed successfully", &workflow.Summary{
			Operation:     "commit",
			FilesAffected: plan.Files,
			Impact:        "Medium",
			SecurityCheck: "Verified",
			Message:       "Commit completed successfully",
			Reasoning:     plan.Reasoning,
			Messages:      plan.Messages,
		})
		return mcpgo.NewToolResultText(result), nil

	case "abort":
		err := s.commitConfirm.DeletePlan()
		s.llm.ClearRetryContext()
		if err != nil {
			return mcpgo.NewToolResultError(err.Error()), nil
		}
		s.sendSuccessNotification("commit", "Commit plan aborted", nil)
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
				Diff:  "", // We don't have the diff stored
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

		s.sendSuccessNotification("commit", "Commit messages regenerated", &workflow.Summary{
			Operation:     "commit",
			FilesAffected: plan.Files,
			Impact:        "Medium",
			SecurityCheck: "Verified",
			Message:       "Commit messages regenerated based on feedback",
			Reasoning:     plan.Reasoning,
			Messages:      newMessages,
		})
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

// Use config from ReleaseService (initialized from config.yaml)
		// This ensures config-driven behavior by default
		createGitHubRelease := s.releaseSvc.GetConfig().CreateGitHubRelease

		res, err := s.applyWithBackup("release", false, func() (workflow.Result, error) {
			output, execErr := s.releaseSvc.Execute(intent, changelog, createGitHubRelease)
			if execErr != nil {
				return workflow.Result{}, execErr
			}
			return workflow.Result{
				Status: workflow.StatusCompleted,
				Output: output,
				Summary: &workflow.Summary{
					Operation: "release",
					Impact:    "High",
					Message:   "Release created successfully",
				},
			}, nil
		})
		if err != nil {
			s.sendErrorNotification("release", "Release execution failed", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError(err.Error()), nil
		}

		s.releaseSvc.ClearPending()
		s.sendSuccessNotification("release", "Release completed", res.Summary)
		return mcpgo.NewToolResultText(fmt.Sprintf("Release created: %s", res.Output)), nil

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

func processingJSON(message string) string {
	structuredPreview := StructuredPreview{
		Header:   "Processing operation",
		Sections: processingSections(message),
		Actions:  []Action{},
	}
	
	resp, _ := json.Marshal(map[string]interface{}{
		"status":            "pending_approval",
		"show_to_user":      "IMPORTANT: Display ALL fields below to the user before asking for confirmation. Do not summarize.",
		"preview":           message,
		"structured_preview": structuredPreview,
	})
	return string(resp)
}

func readyJSON(preview string) string {
	structuredPreview := StructuredPreview{
		Header:   "Review operation details",
		Sections: genericSections("Generic git operation", preview),
		Actions:  genericActions(),
	}
	
	options := make([]string, 0, len(structuredPreview.Actions))
	for _, action := range structuredPreview.Actions {
		options = append(options, action.Label)
	}
	
	resp, _ := json.Marshal(map[string]interface{}{
		"status":            "pending_approval",
		"show_to_user":      "IMPORTANT: Display ALL fields below to the user before asking for confirmation. Do not summarize.",
		"preview":           preview,
		"options":           options,
		"structured_preview": structuredPreview,
	})
	return string(resp)
}

func formatStatus(s domain.Status) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Branch: %s\n", s.Branch))
	if len(s.Files) == 0 {
		sb.WriteString("Working tree clean\n")
		return sb.String()
	}
	for _, f := range s.Files {
		sb.WriteString(fmt.Sprintf("%s%s\n", f.Status, f.Path))
	}
	return sb.String()
}

func releasePlanJSON(intent *domain.ReleaseIntent, changelog string, warnings []string, ghAuth string) string {
	structuredPreview := StructuredPreview{
		Header:   "Review release details",
		Sections: releaseSections(intent, changelog, warnings, ghAuth),
		Actions:  releaseActions(),
	}
	
	options := make([]string, 0, len(structuredPreview.Actions))
	for _, action := range structuredPreview.Actions {
		options = append(options, action.Label)
	}
	
	// Calculate impact based on warnings
	impact := "Medium - Standard release operation"
	if len(warnings) > 0 {
		impact = fmt.Sprintf("High - %d warning(s) detected", len(warnings))
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"status":            "pending_approval",
		"show_to_user":      "IMPORTANT: Display ALL fields below to the user before asking for confirmation. Do not summarize.",
		"tag_name":          intent.TagName,
		"version":           intent.VersionBump,
		"changelog":         changelog,
		"github_auth":       ghAuth,
		"warnings":          warnings,
		"impact":            impact,
		"options":           options,
		"structured_preview": structuredPreview,
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

func tagResultJSON(op, tag string) string {
	return fmt.Sprintf(`{"operation": "tag_%s", "tag": %q, "status": "success"}`, op, tag)
}

func commitPlanJSON(plan *domain.OperationPlan) string {
	structuredPreview := StructuredPreview{
		Header:   "Review commit details",
		Sections: commitSections(plan),
		Actions:  commitActions(),
	}
	
	options := make([]string, 0, len(structuredPreview.Actions))
	for _, action := range structuredPreview.Actions {
		options = append(options, action.Label)
	}
	
	resp, _ := json.Marshal(map[string]interface{}{
		"status":            "pending_approval",
		"show_to_user":      "IMPORTANT: Display ALL fields below to the user before asking for confirmation. Do not summarize.",
		"preview":           plan.Preview,
		"messages":          plan.Messages,
		"files":             gatherFilesFromChunks(plan.Chunks),
		"reasoning":         plan.Reasoning,
		"options":           options,
		"structured_preview": structuredPreview,
	})
	return string(resp)
}