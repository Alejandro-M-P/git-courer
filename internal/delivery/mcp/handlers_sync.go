package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleGitSync(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(getStringParam(params, "command", ""))

	// Extract safety params early
	dryRun := false
	if v, ok := params["dry_run"].(bool); ok {
		dryRun = v
	}
	confirmed := false
	if v, ok := params["confirmed"].(bool); ok {
		confirmed = v
	}

	// Validate known params — no more 'arg' fallback
	if result, err := validateKnownParams(params, []string{"command", "remote_name", "branch_name", "target_commit", "url", "dry_run", "confirmed"}); result != nil || err != nil {
		return result, err
	}

	// Validate command before backup
	validCommands := []string{"FETCH", "PULL", "PUSH", "MERGE", "SWITCH", "REBASE", "REBASE_ABORT", "REBASE_CONTINUE", "CHERRY_PICK", "MERGE_ABORT", "ADD_REMOTE", "REMOVE_REMOTE"}
	valid := false
	for _, c := range validCommands {
		if command == c {
			valid = true
			break
		}
	}
	if !valid {
		hint := suggestCommand(command, validCommands)
		if hint != "" {
			return jsonErrorResult("git_sync", fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return jsonErrorResult("git_sync", fmt.Errorf("unknown command: %s", command))
	}

	remote := getStringParam(params, "remote_name", "origin")
	branch := getStringParam(params, "branch_name", "")
	commit := getStringParam(params, "target_commit", "")

	// Validate required params before any side effects
	switch command {
	case "MERGE", "SWITCH", "REBASE":
		if branch == "" {
			return jsonErrorResult(command, fmt.Errorf("branch_name is required for %s", command))
		}
	case "CHERRY_PICK":
		if commit == "" {
			return jsonErrorResult(command, fmt.Errorf("target_commit is required for CHERRY_PICK"))
		}
	case "ADD_REMOTE":
		if remote == "" || getStringParam(params, "url", "") == "" {
			return jsonErrorResult(command, fmt.Errorf("remote_name and url are required for ADD_REMOTE"))
		}
	}

	// Safety gate for destructive commands (PUSH, MERGE, REBASE, CHERRY_PICK)
	switch command {
	case "PUSH":
		if result, err := checkSafetyGate("push", dryRun, confirmed); result != nil || err != nil {
			return result, err
		}
		if dryRun {
			impact, _ := computeImpact("push", params)
			jsonBytes, _ := json.Marshal(impact)
			return mcpgo.NewToolResultText(string(jsonBytes)), nil
		}
	case "MERGE":
		if result, err := checkSafetyGate("merge", dryRun, confirmed); result != nil || err != nil {
			return result, err
		}
		if dryRun {
			impact, _ := computeImpact("merge", params)
			jsonBytes, _ := json.Marshal(impact)
			return mcpgo.NewToolResultText(string(jsonBytes)), nil
		}
	case "REBASE":
		if result, err := checkSafetyGate("rebase", dryRun, confirmed); result != nil || err != nil {
			return result, err
		}
		if dryRun {
			impact, _ := computeImpact("rebase", params)
			jsonBytes, _ := json.Marshal(impact)
			return mcpgo.NewToolResultText(string(jsonBytes)), nil
		}
	case "CHERRY_PICK":
		if result, err := checkSafetyGate("cherry_pick", dryRun, confirmed); result != nil || err != nil {
			return result, err
		}
		if dryRun {
			impact, _ := computeImpact("cherry_pick", params)
			jsonBytes, _ := json.Marshal(impact)
			return mcpgo.NewToolResultText(string(jsonBytes)), nil
		}
	}

	var err error
	var result string

	// FETCH, PULL, PUSH, MERGE, SWITCH don't stash by default in this implementation's logic
	backup, bErr := s.git.CreateBackup(command, domain.StashNone)
	if bErr == nil {
		s.lastBackup = backup
	}

	switch command {
	case "FETCH":
		_, err = s.git.Fetch()
		result = writeResultJSON("FETCH", err == nil, "Fetched from remote")
	case "PULL":
		_, err = s.git.PullFrom(remote)
		if err != nil && strings.Contains(err.Error(), "NO_UPSTREAM") {
			result = `{"error":"No upstream configured","hint":"Push first or specify remote/branch"}`
			err = nil
			break
		}
		result = writeResultJSON("PULL", err == nil, "Pulled from "+remote)
	case "PUSH":
		_, err = s.git.PushTo(remote)
		result = writeResultJSON("PUSH", err == nil, "Pushed to "+remote)
	case "MERGE":
		_, err = s.git.Merge(branch)
		if err != nil && s.isConflictError(err) {
			conflictFiles := s.getConflictedFiles()
			return mcpgo.NewToolResultText(conflictResultJSON(conflictFiles, true, "Resolve conflicts then use MERGE_ABORT")), nil
		}
		result = writeResultJSON("MERGE", err == nil, fmt.Sprintf("Merged %s", branch))
	case "MERGE_ABORT":
		_, err = s.git.MergeAbort()
		result = writeResultJSON("MERGE_ABORT", err == nil, "Merge aborted")
	case "SWITCH":
		err = s.git.Switch(branch)
		result = writeResultJSON("SWITCH", err == nil, fmt.Sprintf("Switched to %s", branch))
	case "REBASE":
		_, err = s.git.Rebase(branch)
		if err != nil && s.isConflictError(err) {
			conflictFiles := s.getConflictedFiles()
			return mcpgo.NewToolResultText(conflictResultJSON(conflictFiles, true, "Resolve conflicts then use REBASE_ABORT")), nil
		}
		result = writeResultJSON("REBASE", err == nil, fmt.Sprintf("Rebased onto %s", branch))
	case "REBASE_ABORT":
		_, err = s.git.RebaseAbort()
		result = writeResultJSON("REBASE_ABORT", err == nil, "Rebase aborted")
	case "REBASE_CONTINUE":
		_, err = s.git.RebaseContinue()
		result = writeResultJSON("REBASE_CONTINUE", err == nil, "Rebase continued")
	case "CHERRY_PICK":
		_, err = s.git.CherryPick(commit)
		if err != nil && s.isConflictError(err) {
			conflictFiles := s.getConflictedFiles()
			return mcpgo.NewToolResultText(conflictResultJSON(conflictFiles, true, "Resolve conflicts then use MERGE_ABORT")), nil
		}
		result = writeResultJSON("CHERRY_PICK", err == nil, fmt.Sprintf("Cherry-picked %s", commit))
	case "ADD_REMOTE":
		_, err = s.git.RemoteAdd(remote, getStringParam(params, "url", ""))
		result = writeResultJSON("ADD_REMOTE", err == nil, "Remote added")
	case "REMOVE_REMOTE":
		_, err = s.git.RemoteRemove(remote)
		result = writeResultJSON("REMOVE_REMOTE", err == nil, "Remote removed")
	}

	if err != nil {
		s.sendErrorNotification("git_sync", command+" failed", map[string]any{"error": err.Error()})
		return jsonErrorResult(command, err)
	}
	s.sendSuccessNotification("git_sync", command+" completed", nil)
	return mcpgo.NewToolResultText(result), nil
}

// isConflictError checks if a git error message indicates a merge conflict.
func (s *Server) isConflictError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "CONFLICT") || strings.Contains(msg, "conflict")
}

// getConflictedFiles returns a list of file paths with conflict markers from git status.
func (s *Server) getConflictedFiles() []string {
	status, err := s.git.Status()
	if err != nil {
		return []string{}
	}
	var files []string
	for _, f := range status.Files {
		if f.Status == "UU" || f.Status == "AA" || f.Status == "DU" || f.Status == "UD" {
			files = append(files, f.Path)
		}
	}
	if len(files) == 0 {
		return []string{"(conflicted files)"}
	}
	return files
}
