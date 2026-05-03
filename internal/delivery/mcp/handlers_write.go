package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleGitWrite(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(getStringParam(params, "command", ""))

	// UNDO is a special command that doesn't create a new backup
	if command == "UNDO" {
		if s.lastBackup.Ref == "" {
			return jsonErrorResult("UNDO", fmt.Errorf("no operation to undo"))
		}
		err := s.git.RestoreBackup(s.lastBackup)
		if err != nil {
			return jsonErrorResult("UNDO", err)
		}
		msg := fmt.Sprintf("Successfully reverted last operation (%s)", s.lastBackup.Operation)
		s.lastBackup = domain.Backup{} // Clear after undo
		return mcpgo.NewToolResultText(writeResultJSON("UNDO", true, msg)), nil
	}

	// Specific parameters override generic 'arg'
	arg := getStringParam(params, "arg", "")
	if p := getStringParam(params, "paths", ""); p != "" {
		arg = p
	} else if b := getStringParam(params, "branch", ""); b != "" {
		arg = b
	} else if m := getStringParam(params, "message", ""); m != "" {
		arg = m
	} else if c := getStringParam(params, "commit", ""); c != "" {
		arg = c
	} else if n := getStringParam(params, "name", ""); n != "" {
		arg = n
	}

	var err error
	var result string

	// Auto-backup for direct write operations
	// ADD and RM should NOT stash because they need the files in the working tree
	mode := domain.StashAll
	if command == "ADD" || command == "RM" || command == "STASH" || command == "STASH_POP" {
		mode = domain.StashNone
	}
	backup, bErr := s.git.CreateBackup(command, mode)
	if bErr == nil {
		s.lastBackup = backup
	}

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
		remote := getStringParam(params, "remote", "origin")
		branch := getStringParam(params, "branch", "")
		if branch == "" {
			_, err = s.git.PushTo(remote)
		} else {
			// This might need a new port method if we want full flexibility, 
			// but PushTo already handles custom remote.
			_, err = s.git.PushTo(remote) 
		}
		result = writeResultJSON("PUSH", err == nil, "Pushed to "+remote)
	case "PULL":
		remote := getStringParam(params, "remote", "origin")
		if arg != "" {
			remote = arg
		}
		_, err = s.git.PullFrom(remote)
		if err != nil && strings.Contains(err.Error(), "NO_UPSTREAM") {
			result = `{"error":"No upstream configured","hint":"Push first or specify remote/branch"}`
			err = nil
			break
		}
		result = writeResultJSON("PULL", err == nil, "Pulled from "+remote)
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
		force := false
		if v, ok := params["force"].(bool); ok {
			force = v
		}
		_, err = s.git.DeleteBranch(arg, force)
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
	case "MERGE":
		_, err = s.git.Merge(arg)
		result = writeResultJSON("MERGE", err == nil, fmt.Sprintf("Merged %s", arg))
	case "STASH":
		msg := getStringParam(params, "message", "")
		if msg != "" {
			_, err = s.git.Stash(msg)
		} else {
			_, err = s.git.Stash()
		}
		result = writeResultJSON("STASH", err == nil, "Changes stashed")
	case "STASH_POP":
		_, err = s.git.StashPop()
		if err != nil && strings.Contains(err.Error(), "STASH_POP_UNTRACKED:") {
			result = `{"error":"Stash pop failed: untracked files conflict","hint":"Use 'STASH' command with -u flag to include untracked files next time"}`
			err = nil
			break
		}
		result = writeResultJSON("STASH_POP", err == nil, "Stash restored")

	case "PRUNE_BACKUPS":
		days := 7
		if v, ok := params["days"].(float64); ok {
			days = int(v)
		}
		err = s.git.PruneBackups(time.Duration(days) * 24 * time.Hour)
		result = writeResultJSON("PRUNE_BACKUPS", err == nil, fmt.Sprintf("Backups older than %d days deleted", days))

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
