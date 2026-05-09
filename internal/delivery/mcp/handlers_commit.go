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
			plan      *domain.OperationPlan
			execResult string
			err       error
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

				messages, chunks, deleted, _, reasoning, err := s.commitSvc.PrepareCommit(instruction)
				if err != nil {
					ch <- runOut{err: err}
					return
				}

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
