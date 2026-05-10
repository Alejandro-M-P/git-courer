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
)

func (s *Server) handleGitReview(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, ok := req.Params.Arguments.(map[string]any)
	if !ok || params == nil {
		return mcpgo.NewToolResultError("invalid request: params are required"), nil
	}
	command, ok := params["command"].(string)
	if !ok || command == "" {
		return mcpgo.NewToolResultError("invalid request: command is required"), nil
	}
	command = strings.ToUpper(command)

	if command == "STATUS" {
		status, _ := s.reviewWorkflow.PlanStatus()
		s.sendSuccessNotification("status", "Status retrieved", nil)
		return mcpgo.NewToolResultText(status), nil
	}
	if command == "SUMMARY" {
		status, err := s.git.Status()
		if err != nil {
			return jsonErrorResult(command, err)
		}
		s.sendSuccessNotification("summary", "Git summary retrieved", nil)
		return mcpgo.NewToolResultText(formatStatus(status)), nil
	}
	if command == "JOB_RESULT" {
		arg := getStringParam(params, "arg", "")
		if arg == "" {
			return jsonErrorResult(command, fmt.Errorf("arg (job_id) is required"))
		}
		j, ok := s.getBgJob(arg)
		if !ok {
			return jsonErrorResult(command, fmt.Errorf("job not found: %s", arg))
		}
		return mcpgo.NewToolResultText(bgJobResultJSON(j)), nil
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

	return mcpgo.NewToolResultError("Unknown command: " + command), nil
}

// handleCommitOperation handles commit operations using CommitService.
func (s *Server) handleCommitOperation(_ context.Context, req mcpgo.CallToolRequest, phase string) (*mcpgo.CallToolResult, error) {
	preview := req.GetBool("preview", s.cfg.Preview.Enabled)

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

		type runOut struct {
			plan       *domain.OperationPlan
			execResult string
			err        error
		}

		opCtx, opCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		ch := make(chan runOut, 1)
		keepalive := s.startKeepalive("Preparing commit", 10*time.Second)

		go func() {
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

				// Report progress: starting preparation
				s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
					"level":  "info",
					"logger": "git-courer",
					"data":   "⚙️ Analyzing staged changes and preparing diff chunks...",
				})

				messages, chunks, deleted, _, reasoning, err := s.commitSvc.PrepareCommit(instruction)
				if err != nil {
					ch <- runOut{err: err}
					return
				}

				// Report progress: starting LLM generation
				s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
					"level":  "info",
					"logger": "git-courer",
					"data":   fmt.Sprintf("⚙️ Generated %d semantic chunks. Generating commit messages with LLM...", len(chunks)),
				})

				chunkFiles := make([][]string, len(chunks))
				var files []string
				for i, c := range chunks {
					chunkFiles[i] = c.Files
					files = append(files, c.Files...)
				}
				files = append(files, deleted...)

				plan := &domain.OperationPlan{
					Operation:    "commit",
					Messages:     messages,
					Files:        files,
					Chunks:       chunkFiles,
					DeletedFiles: deleted,
					Instruction:  instruction,
					Reasoning:    reasoning,
					Preview:      strings.Join(messages, "\n"),
				}
				ch <- runOut{plan: plan}
			} else {
				result, err := s.commitSvc.Execute(instruction, false)
				ch <- runOut{execResult: result, err: err}
			}
		}()

		select {
		case out := <-ch:
			opCancel()
			close(keepalive)
			if out.err != nil {
				if strings.Contains(out.err.Error(), "[SECURITY]") {
					s.sendSecurityErrorNotification(out.err.Error())
				}
				s.sendErrorNotification("commit", "Commit operation failed", map[string]any{"error": out.err.Error()})
				return mcpgo.NewToolResultError("Commit operation failed: " + out.err.Error()), nil
			}

			if preview && out.plan != nil {
				// Send preview via notification
				s.sendSuccessNotification("commit", "Commit plan ready for review", &workflow.Summary{
					Operation:     "commit",
					FilesAffected: out.plan.Files,
					Impact:        "Medium",
					SecurityCheck: "Passed",
					Message:       "Commit plan ready for review",
					Reasoning:     out.plan.Reasoning,
					Messages:      out.plan.Messages,
				})

				if err := s.commitConfirm.WritePlan(*out.plan); err != nil {
					s.sendErrorNotification("commit", "Failed to save plan", map[string]any{"error": err.Error()})
					return mcpgo.NewToolResultError("Failed to save plan: " + err.Error()), nil
				}
				if err := s.commitConfirm.CreateBlocker(); err != nil {
					log.Printf("[DEBUG] COMMIT_START: CreateBlocker error: %v", err)
				}
				return mcpgo.NewToolResultText(commitPlanJSON(out.plan)), nil
			}

			s.sendSuccessNotification("commit", "Commit completed successfully", nil)
			return mcpgo.NewToolResultText(out.execResult), nil

		case <-time.After(45 * time.Second):
			close(keepalive)
			jobID := s.newBgJob("commit")
			go func() {
				defer opCancel()
				select {
				case out := <-ch:
					if out.err != nil {
						s.failBgJob(jobID, out.err.Error())
						s.sendErrorNotification("commit", "Commit failed (background)", map[string]any{
							"job_id": jobID,
							"error":  out.err.Error(),
						})
						return
					}
					if preview && out.plan != nil {
						if err := s.commitConfirm.WritePlan(*out.plan); err == nil {
							s.commitConfirm.CreateBlocker()
						}
						s.finishBgJob(jobID, commitPlanJSON(out.plan))
						s.sendSuccessNotification("commit", "Commit plan ready (background)", &workflow.Summary{
							Operation:     "commit",
							FilesAffected: out.plan.Files,
							Impact:        "Medium",
							SecurityCheck: "Passed",
							Message:       fmt.Sprintf("Commit plan ready [job:%s]", jobID),
							Reasoning:     out.plan.Reasoning,
							Messages:      out.plan.Messages,
						})
					} else {
						s.finishBgJob(jobID, out.execResult)
						s.sendSuccessNotification("commit", "Commit completed (background)", &workflow.Summary{
							Operation: "commit",
							Message:   fmt.Sprintf("Commit done [job:%s]", jobID),
						})
					}
				case <-opCtx.Done():
					s.failBgJob(jobID, "commit timed out after 10 minutes")
					s.sendErrorNotification("commit", "Commit timed out", map[string]any{
						"job_id": jobID,
						"error":  "exceeded 10 minute limit",
					})
				}
			}()
			return mcpgo.NewToolResultText(fmt.Sprintf(
				`{"status":"background","job_id":%q,"op":"commit","hint":"Will notify when done. Use JOB_RESULT to retrieve result."}`,
				jobID,
			)), nil
		}

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

		// Clean up plan after successful execution
		if err := s.commitConfirm.DeletePlan(); err != nil {
			log.Printf("[WARN] Failed to delete commit plan after apply: %v", err)
		}
		s.commitConfirm.RemoveBlocker()
		s.llm.ClearRetryContext()

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
		dryRun := req.GetBool("dry_run", false)

		type runOut struct {
			intent       *domain.ReleaseIntent
			changelog    string
			warnings     []string
			ghStatus     string
			commitsCount int
			dryRun       bool
			err          error
		}

		jobID := s.newBgJob("release")
		opCtx, opCancel := context.WithTimeout(context.Background(), 10*time.Minute)
		ch := make(chan runOut, 1)
		keepalive := s.startKeepalive("Preparing release", 10*time.Second)

		go func() {
			defer opCancel()
			select {
			case <-opCtx.Done():
				s.failBgJob(jobID, "release timed out after 10 minutes")
				s.sendErrorNotification("release", "Release timed out", map[string]any{
					"job_id": jobID,
					"error":  "exceeded 10 minute limit",
				})
				return
			default:
			}

			s.releaseSvc.ClearPending()

			s.releaseSvc.SetProgressCallback(func(done, total int) {
				progress := fmt.Sprintf("%d/%d chunks", done, total)
				s.updateBgJobProgress(jobID, progress)
				s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
					"level":  "info",
					"logger": "git-courer",
					"data":   fmt.Sprintf("⚙️ generating changelog: %s processed", progress),
				})
			})

			intent, commits, warnings, err := s.releaseSvc.Prepare(instruction, "")
			if err != nil {
				s.failBgJob(jobID, err.Error())
				ch <- runOut{err: err}
				return
			}

			commitsCount := len(strings.Split(commits, "\n"))

			// Report progress: starting changelog generation
			s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
				"level":  "info",
				"logger": "git-courer",
				"data":   fmt.Sprintf("⚙️ Found %d commits. Generating changelog with LLM...", commitsCount),
			})

			changelog, warningsGen, _, err := s.releaseSvc.Generate(commits)
			if err != nil {
				s.failBgJob(jobID, err.Error())
				ch <- runOut{err: err}
				return
			}

			allWarnings := append(warnings, warningsGen...)

			authenticated, _ := s.git.IsGHAuthenticated()
			ghStatus := "authenticated"
			if !authenticated {
				ghStatus = "not authenticated (will create tag only)"
			}

			out := runOut{
				intent:       intent,
				changelog:    changelog,
				warnings:     allWarnings,
				ghStatus:     ghStatus,
				commitsCount: commitsCount,
				dryRun:       dryRun,
			}

			if dryRun {
				// Dry-run: return preview without persisting state or creating blocker
				s.finishBgJob(jobID, releasePlanJSON(intent, changelog, allWarnings, ghStatus, true, commitsCount))
				s.sendSuccessNotification("release", "Release preview (dry run)", &workflow.Summary{
					Operation: "release",
					Message:   fmt.Sprintf("Release preview %s [job:%s] (dry run)", intent.TagName, jobID),
				})
			} else {
				s.releaseSvc.SaveIntent(intent)
				s.releaseSvc.SaveChangelog(changelog)
				s.releaseConfirm.CreateBlocker()
				s.finishBgJob(jobID, releasePlanJSON(intent, changelog, allWarnings, ghStatus, false, 0))

				// If we already returned the background JSON, send a success notification
				s.sendSuccessNotification("release", "Release plan ready", &workflow.Summary{
					Operation: "release",
					Message:   fmt.Sprintf("Release plan %s ready [job:%s] → call RELEASE_APPLY", intent.TagName, jobID),
				})
			}

			ch <- out
		}()

		select {
		case out := <-ch:
			close(keepalive)
			if out.err != nil {
				s.sendErrorNotification("release", "Failed to prepare release", map[string]any{"error": out.err.Error()})
				return mcpgo.NewToolResultError("Failed to prepare release: " + out.err.Error()), nil
			}

			return mcpgo.NewToolResultText(releasePlanJSON(out.intent, out.changelog, out.warnings, out.ghStatus, out.dryRun, out.commitsCount)), nil

		case <-time.After(45 * time.Second):
			close(keepalive)
			return mcpgo.NewToolResultText(fmt.Sprintf(
				`{"status":"background","job_id":%q,"op":"release","hint":"Generating changelog. Will notify when done. Use JOB_RESULT to retrieve result."}`,
				jobID,
			)), nil
		}

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
		if changelog == "" {
			return mcpgo.NewToolResultError(`{"error":"changelog not ready","hint":"Background generation still running. Wait for the done notification."}`), nil
		}

		res, err := s.applyWithBackup("release", false, func() (workflow.Result, error) {
			output, execErr := s.releaseSvc.Execute(intent, changelog)
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
			s.releaseConfirm.RemoveBlocker()
			s.sendErrorNotification("release", "Release execution failed", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError(err.Error()), nil
		}

		s.releaseSvc.ClearPending()
		s.releaseConfirm.RemoveBlocker()
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
