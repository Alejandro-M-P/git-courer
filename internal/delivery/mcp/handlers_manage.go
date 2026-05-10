package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleGitManage(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(getStringParam(params, "command", ""))

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

	arg := getStringParam(params, "arg", "")
	if p := getStringParam(params, "paths", ""); p != "" {
		arg = p
	} else if b := getStringParam(params, "branch", ""); b != "" {
		arg = b
	} else if m := getStringParam(params, "message", ""); m != "" {
		arg = m
	} else if n := getStringParam(params, "name", ""); n != "" {
		arg = n
	}

	var err error
	var result string

	// Metadata operations should NOT stash
	// Auto-backup for direct write operations
	// Metadata operations should NOT stash because they don't touch the working tree
	if command != "LIST_BACKUPS" && command != "READ_CONFIG" && command != "LIST_MODELS" {
		backup, bErr := s.git.CreateBackup(command, domain.StashNone)
		if bErr == nil {
			s.lastBackup = backup
		}
	}

	switch command {
	case "UPDATE_CONFIG":
		parts := strings.SplitN(arg, ":", 2)
		if len(parts) != 2 {
			return jsonErrorResult("UPDATE_CONFIG", fmt.Errorf("invalid format: expected key:value, got: %s", arg))
		}
		key, val := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		switch key {
		case "llm.provider":
			s.cfg.LLM.Provider = val
		case "llm.model":
			s.cfg.LLM.Model = val
		case "llm.base_url":
			s.cfg.LLM.BaseURL = val
		case "context.project":
			s.cfg.Context.Project = val
		case "context.style":
			s.cfg.Context.Style = val
		case "preview.enabled":
			s.cfg.Preview.Enabled = (val == "true")
		default:
			return jsonErrorResult("UPDATE_CONFIG", fmt.Errorf("unknown config key: %s", key))
		}
		err = s.cfg.SaveGlobal()
		result = writeResultJSON("UPDATE_CONFIG", err == nil, fmt.Sprintf("Config updated: %s=%s", key, val))
	case "RENAME_BRANCH":
		parts := strings.Split(arg, ":")
		if len(parts) != 2 {
			return jsonErrorResult("RENAME_BRANCH", fmt.Errorf("invalid format: expected old_name:new_name, got: %s", arg))
		}
		_, err = s.git.RenameBranch(parts[0], parts[1])
		result = writeResultJSON("RENAME_BRANCH", err == nil, fmt.Sprintf("Renamed %s to %s", parts[0], parts[1]))
	case "BRANCH_CREATE":
		names := git.SplitPaths(arg)
		var created []string
		for _, name := range names {
			_, err = s.git.Branch(name)
			if err != nil {
				break
			}
			created = append(created, name)
		}
		result = writeResultJSON("BRANCH_CREATE", err == nil, fmt.Sprintf("Created branches: %s", strings.Join(created, ", ")))
	case "BRANCH_DELETE":
		force := false
		if v, ok := params["force"].(bool); ok {
			force = v
		}
		names := git.SplitPaths(arg)
		var deleted []string
		for _, name := range names {
			_, err = s.git.DeleteBranch(name, force)
			if err != nil {
				break
			}
			deleted = append(deleted, name)
		}
		result = writeResultJSON("BRANCH_DELETE", err == nil, fmt.Sprintf("Deleted branches: %s", strings.Join(deleted, ", ")))
	case "REMOTE_BRANCH_DELETE":
		names := git.SplitPaths(arg)
		var deleted []string
		for _, name := range names {
			err = s.git.DeleteRemoteBranch(name)
			if err != nil {
				break
			}
			deleted = append(deleted, name)
		}
		result = writeResultJSON("REMOTE_BRANCH_DELETE", err == nil, fmt.Sprintf("Deleted remote branches: %s", strings.Join(deleted, ", ")))
	case "REMOTE_TAG_DELETE":
		names := git.SplitPaths(arg)
		var deleted []string
		for _, name := range names {
			_, err = s.git.DeleteTagRemote(name)
			if err != nil {
				break
			}
			deleted = append(deleted, name)
		}
		result = writeResultJSON("REMOTE_TAG_DELETE", err == nil, fmt.Sprintf("Deleted remote tags: %s", strings.Join(deleted, ", ")))
	case "TAG_CREATE":
		names := git.SplitPaths(arg)
		var created []string
		for _, name := range names {
			_, err = s.git.Tag(name, "")
			if err != nil {
				break
			}
			created = append(created, name)
		}
		result = tagResultJSON("created", strings.Join(created, ", "))
	case "TAG_DELETE":
		names := git.SplitPaths(arg)
		var deleted []string
		for _, name := range names {
			_, err = s.git.DeleteTag(name)
			if err != nil {
				break
			}
			deleted = append(deleted, name)
		}
		result = tagResultJSON("deleted", strings.Join(deleted, ", "))
	case "TAG_PUSH":
		names := git.SplitPaths(arg)
		var pushed []string
		for _, name := range names {
			_, err = s.git.PushTag(name)
			if err != nil {
				break
			}
			pushed = append(pushed, name)
		}
		result = tagResultJSON("pushed", strings.Join(pushed, ", "))
	case "TAG_DELETE_REMOTE":
		names := git.SplitPaths(arg)
		var deleted []string
		for _, name := range names {
			_, err = s.git.DeleteTagRemote(name)
			if err != nil {
				break
			}
			deleted = append(deleted, name)
		}
		result = tagResultJSON("deleted from remote", strings.Join(deleted, ", "))
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
	case "STASH_APPLY":
		_, err = s.git.StashApply(arg)
		result = writeResultJSON("STASH_APPLY", err == nil, "Stash applied (kept in stash list)")
	case "STASH_DROP":
		if arg == "" {
			return jsonErrorResult("STASH_DROP", fmt.Errorf("arg (index) required, e.g. arg=0"))
		}
		_, err = s.git.StashDrop(arg)
		result = writeResultJSON("STASH_DROP", err == nil, "Stash entry dropped")
	case "STASH_CLEAR":
		_, err = s.git.StashClear()
		result = writeResultJSON("STASH_CLEAR", err == nil, "All stash entries cleared")
	case "PRUNE_BACKUPS":
		days := 7
		if v, ok := params["days"].(float64); ok {
			days = int(v)
		}
		err = s.git.PruneBackups(time.Duration(days) * 24 * time.Hour)
		result = writeResultJSON("PRUNE_BACKUPS", err == nil, fmt.Sprintf("Backups older than %d days deleted", days))
	case "LIST_BACKUPS":
		backups, err := s.git.ListBackups()
		if err != nil {
			return jsonErrorResult(command, err)
		}
		result = formatBackupListJSON(backups)
	case "READ_CONFIG":
		result = mustJSON(map[string]interface{}{
			"config_path": config.GlobalConfigPath(),
			"content":     s.cfg,
		})
	case "LIST_MODELS":
		result = mustJSON(map[string]interface{}{
			"provider": s.cfg.LLM.Provider,
			"models":   []string{s.cfg.LLM.Model},
			"message":  "Models are configured statically via config file. Showing current configured model.",
		})
	default:
		return jsonErrorResult("git_manage", fmt.Errorf("unknown command: %s", command))
	}

	if err != nil {
		s.sendErrorNotification("git_manage", command+" failed", map[string]any{"error": err.Error()})
		return jsonErrorResult(command, err)
	}
	s.sendSuccessNotification("git_manage", command+" completed", nil)
	return mcpgo.NewToolResultText(result), nil
}
