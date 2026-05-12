package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
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
		// Handle non-prefixed commands like REVERT, AMEND
		if command == "REVERT" {
			return s.handleRevert(params)
		}
		if command == "AMEND" {
			return s.handleAmend(params)
		}
		return mcpgo.NewToolResultError("Invalid command format. Expected {OP}_{PHASE} or REVERT/AMEND"), nil
	}

	if op == "commit" {
		return s.handleCommitOperation(ctx, req, phase)
	}
	if op == "release" {
		return s.handleRelease(ctx, req, phase)
	}

	return mcpgo.NewToolResultError("Unknown command: " + command), nil
}

func (s *Server) handleCommitOperation(_ context.Context, req mcpgo.CallToolRequest, phase string) (*mcpgo.CallToolResult, error) {
	return mcpgo.NewToolResultText("Commit operation handler (stub)"), nil
}

func (s *Server) handleRelease(ctx context.Context, req mcpgo.CallToolRequest, phase string) (*mcpgo.CallToolResult, error) {
	return mcpgo.NewToolResultText("Release operation handler (stub)"), nil
}

func (s *Server) handleRevert(params map[string]any) (*mcpgo.CallToolResult, error) {
	if result, err := validateRequiredParam(params, "target_commit", "REVERT"); result != nil || err != nil {
		return result, err
	}

	// Extract safety params
	dryRun := false
	if v, ok := params["dry_run"].(bool); ok {
		dryRun = v
	}
	confirmed := false
	if v, ok := params["confirmed"].(bool); ok {
		confirmed = v
	}

	// Safety gate (revert is not destructive, but supports dry_run preview)
	if dryRun {
		impact, _ := computeImpact("revert", params)
		jsonBytes, _ := json.Marshal(impact)
		return mcpgo.NewToolResultText(string(jsonBytes)), nil
	}
	_ = confirmed // revert doesn't require confirmed, but param exists for future use

	out, err := s.git.Revert(getStringParam(params, "target_commit", ""))
	if err != nil {
		return jsonErrorResult("REVERT", err)
	}
	return mcpgo.NewToolResultText(writeResultJSON("REVERT", true, out)), nil
}

func (s *Server) handleAmend(params map[string]any) (*mcpgo.CallToolResult, error) {
	// Extract safety params
	dryRun := false
	if v, ok := params["dry_run"].(bool); ok {
		dryRun = v
	}
	confirmed := false
	if v, ok := params["confirmed"].(bool); ok {
		confirmed = v
	}

	// Safety gate (amend is not destructive, but supports dry_run preview)
	if dryRun {
		impact, _ := computeImpact("amend", params)
		jsonBytes, _ := json.Marshal(impact)
		return mcpgo.NewToolResultText(string(jsonBytes)), nil
	}
	_ = confirmed // amend doesn't require confirmed, but param exists for future use

	// target_paths is optional for amend (adds files)
	out, err := s.git.Amend(getStringParam(params, "commit_message", ""), git.SplitPaths(getStringParam(params, "target_paths", "")))
	if err != nil {
		return jsonErrorResult("AMEND", err)
	}
	return mcpgo.NewToolResultText(writeResultJSON("AMEND", true, out)), nil
}
