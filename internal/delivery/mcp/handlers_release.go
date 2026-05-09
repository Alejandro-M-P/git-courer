package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/Alejandro-M-P/git-courer/internal/workflow"
)

// handleRelease handles release operations using ReleaseService.
func (s *Server) handleRelease(_ context.Context, req mcpgo.CallToolRequest, phase string) (*mcpgo.CallToolResult, error) {
	switch phase {
	case "start":
		instruction := req.GetString("instruction", "")
		if instruction == "" {
			instruction = "sacar versión"
		}

		type runOut struct {
			intent    *domain.ReleaseIntent
			changelog string
			warnings  []string
			ghStatus  string
			err       error
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

			// Report progress: starting changelog generation
			s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
				"level":  "info",
				"logger": "git-courer",
				"data":   fmt.Sprintf("⚙️ Found %d commits. Generating changelog with LLM...", len(strings.Split(commits, "\n"))),
			})

			changelog, warningsGen, _, err := s.releaseSvc.Generate(commits)
			if err != nil {
				s.failBgJob(jobID, err.Error())
				ch <- runOut{err: err}
				return
			}

			s.releaseSvc.SaveIntent(intent)
			s.releaseSvc.SaveChangelog(changelog)
			allWarnings := append(warnings, warningsGen...)

			authenticated, _ := s.git.IsGHAuthenticated()
			ghStatus := "authenticated"
			if !authenticated {
				ghStatus = "not authenticated (will create tag only)"
			}

			out := runOut{
				intent:    intent,
				changelog: changelog,
				warnings:  allWarnings,
				ghStatus:  ghStatus,
			}
			
			s.releaseConfirm.CreateBlocker()
			s.finishBgJob(jobID, releasePlanJSON(intent, changelog, allWarnings, ghStatus))
			
			// If we already returned the background JSON, send a success notification
			s.sendSuccessNotification("release", "Release plan ready", &workflow.Summary{
				Operation: "release",
				Message:   fmt.Sprintf("Release plan %s ready [job:%s] → call RELEASE_APPLY", intent.TagName, jobID),
			})

			ch <- out
		}()

		select {
		case out := <-ch:
			close(keepalive)
			if out.err != nil {
				s.sendErrorNotification("release", "Failed to prepare release", map[string]any{"error": out.err.Error()})
				return mcpgo.NewToolResultError("Failed to prepare release: " + out.err.Error()), nil
			}

			return mcpgo.NewToolResultText(releasePlanJSON(out.intent, out.changelog, out.warnings, out.ghStatus)), nil

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
