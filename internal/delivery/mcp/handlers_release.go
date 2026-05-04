package mcp

import (
	"context"
	"fmt"
	"time"

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

		keepalive := s.startKeepalive("Preparing release", 30*time.Second)
		s.releaseSvc.ClearPending()

		intent, commits, warnings, err := s.releaseSvc.Prepare(instruction, "")
		close(keepalive)
		if err != nil {
			s.sendErrorNotification("release", "Failed to prepare release", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError("Failed to prepare release: " + err.Error()), nil
		}

		jobID := s.newBgJob("release-changelog")
		s.releaseSvc.SetProgressCallback(func(done, total int) {
			progress := fmt.Sprintf("%d/%d chunks", done, total)
			s.updateBgJobProgress(jobID, progress)
			s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
				"level":  "info",
				"logger": "git-courer",
				"data":   fmt.Sprintf("⚙️ changelog %s done [job:%s]", progress, jobID),
			})
		})
		s.releaseSvc.SetDoneCallback(func(changelog string) {
			if changelog == "" {
				s.failBgJob(jobID, "no chunks succeeded")
				s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
					"level":  "error",
					"logger": "git-courer",
					"data":   fmt.Sprintf("❌ changelog failed [job:%s]", jobID),
				})
				return
			}
			s.finishBgJob(jobID, changelog)
			s.mcpServer.SendNotificationToAllClients("notifications/message", map[string]any{
				"level":  "info",
				"logger": "git-courer",
				"data":   fmt.Sprintf("✅ changelog ready [job:%s] → call RELEASE_APPLY", jobID),
			})
		})

		keepalive = s.startKeepalive("Generating changelog", 30*time.Second)
		changelog, warningsGen, isBackground, err := s.releaseSvc.Generate(commits)
		close(keepalive)
		if err != nil {
			s.sendErrorNotification("release", "Failed to generate changelog", map[string]any{"error": err.Error()})
			return mcpgo.NewToolResultError("Failed to generate changelog: " + err.Error()), nil
		}

		s.releaseSvc.SaveIntent(intent)
		allWarnings := append(warnings, warningsGen...)

		if isBackground {
			s.releaseConfirm.CreateBlocker()
			return mcpgo.NewToolResultText(fmt.Sprintf(
				`{"status":"background","job_id":%q,"tag":%q,"hint":"Will notify when changelog is ready. Then call RELEASE_APPLY."}`,
				jobID, intent.TagName,
			)), nil
		}

		s.releaseSvc.SaveChangelog(changelog)
		s.releaseConfirm.CreateBlocker()

		authenticated, _ := s.git.IsGHAuthenticated()
		ghStatus := "authenticated"
		if !authenticated {
			ghStatus = "not authenticated (will create tag only)"
		}

		s.sendSuccessNotification("release", "Release plan ready for review", map[string]any{
			"status":      "pending_approval",
			"tag_name":    intent.TagName,
			"version":     intent.VersionBump,
			"changelog":   changelog,
			"github_auth": ghStatus,
			"warnings":    allWarnings,
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
