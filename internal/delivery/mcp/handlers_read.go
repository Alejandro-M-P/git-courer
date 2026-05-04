package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/infra/filters"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

type WhatChangedFilter string

const (
	WhatChangedFilterAll      WhatChangedFilter = "all"
	WhatChangedFilterStaged   WhatChangedFilter = "staged"
	WhatChangedFilterUnstaged WhatChangedFilter = "unstaged"
)

func (s *Server) handleGitRead(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(getStringParam(params, "command", ""))

	// Specific parameters override generic 'arg'
	arg := getStringParam(params, "arg", "")
	if p := getStringParam(params, "path", ""); p != "" {
		arg = p
	} else if h := getStringParam(params, "hash", ""); h != "" {
		arg = h
	} else if r := getStringParam(params, "revision", ""); r != "" {
		arg = r
	}

	limit, offset := parsePagination(params)

	// filter param: narrow results by pattern (filepath for diffs, branch name for log, etc.)
	filter := getStringParam(params, "filter", "")

	// pattern param: used for READ_LOG grep
	pattern := getStringParam(params, "pattern", "")

	// context, before, after for READ_SEARCH
	contextSearch, beforeSearch, afterSearch := 0, 0, 0
	if v, ok := params["context"].(float64); ok {
		contextSearch = int(v)
	}
	if v, ok := params["before"].(float64); ok {
		beforeSearch = int(v)
	}
	if v, ok := params["after"].(float64); ok {
		afterSearch = int(v)
	}

	// compact param: only show + and - lines (no file headers, hunk markers)
	compact := false
	if v, ok := params["compact"].(bool); ok {
		compact = v
	}

	// Defaults for limit
	if limit <= 0 {
		switch command {
		case "READ_DIFF", "READ_DIFF_STAGED", "READ_DIFF_ALL":
			limit = 200
		case "READ_LOG":
			limit = 20
		case "READ_STATUS":
			limit = 100
		default:
			limit = 50
		}
	}

	var err error
	var result string

	switch command {
	case "READ_STATUS":
		status, err := s.git.Status()
		if err != nil {
			return jsonErrorResult(command, err)
		}
		result = formatStatusJSON(status, limit, offset, filter)

	case "READ_DIFF":
		result, err = s.handleDiffCommand(arg, limit, offset, "", filter, compact)

	case "READ_DIFF_STATS", "READ_DIFF_STAT":
		result, err = s.handleDiffStatCommand(arg)

	case "READ_DIFF_STAGED":
		result, err = s.handleDiffCommand(arg, limit, offset, "--cached", filter, compact)

	case "READ_DIFF_ALL":
		result, err = s.handleDiffAllCommand(arg, limit, offset, filter, compact)

	case "READ_LOG":
		result, err = s.handleLogCommand(arg, pattern, limit, offset, filter)

	case "READ_BRANCHES":
		branches, err := s.git.ListBranches(arg)
		if err != nil {
			return jsonErrorResult(command, err)
		}
		current, _ := s.git.CurrentBranch()
		list := SanitizeBranchList(branches)
		if filter != "" {
			list = filterStringSlice(list, filter)
		}
		result = formatBranchListJSON(list, current, limit, offset)

	case "READ_TAGS":
		tags, err := s.git.ListTags(arg)
		if err != nil {
			return jsonErrorResult(command, err)
		}
		if filter != "" {
			tags = filterStringSlice(tags, filter)
		}
		result = formatTagListJSON(tags, limit, offset)

	case "CURRENT_BRANCH":
		branch, err := s.git.CurrentBranch()
		if err != nil {
			return jsonErrorResult(command, err)
		}
		result = fmt.Sprintf(`{"current_branch":%q}`, branch)

	case "IS_REPO":
		isRepo := s.git.IsRepo()
		result = fmt.Sprintf(`{"is_repo":%v}`, isRepo)

	case "REMOTE_BRANCH_LIST":
		branches, err := s.git.ListBranches("-r")
		if err != nil {
			return jsonErrorResult(command, err)
		}
		list := SanitizeBranchList(branches)
		if filter != "" {
			list = filterStringSlice(list, filter)
		}
		result = formatRemoteBranchListJSON(list, limit, offset)

	case "REMOTE_TAG_LIST":
		tags, err := s.git.ListTags()
		if err != nil {
			return jsonErrorResult(command, err)
		}
		if filter != "" {
			tags = filterStringSlice(tags, filter)
		}
		result = formatRemoteTagListJSON(tags, limit, offset)

	case "REMOTE_INFO":
		info, err := s.git.RemoteInfo()
		if err != nil {
			return jsonErrorResult(command, err)
		}
		result = fmt.Sprintf(`{"remote_info":%q}`, info)

	case "WHAT_CHANGED":
		filterMode := WhatChangedFilter(getStringParam(params, "filter", "all"))
		useLLM := true
		if v, ok := params["llm"].(bool); ok {
			useLLM = v
		}
		result, err = s.handleWhatChangedCommand(arg, filterMode, useLLM)

	case "BLAME":
		if arg == "" {
			return jsonErrorResult(command, fmt.Errorf("arg (filepath) is required for BLAME"))
		}
		lines, err := s.git.Blame(arg)
		if err != nil {
			return jsonErrorResult(command, err)
		}
		// Default limit for BLAME
		if limit <= 0 {
			limit = 50
		}
		result = blameResultJSON(arg, lines, limit, offset)

	case "SHOW":
		if arg == "" {
			return jsonErrorResult(command, fmt.Errorf("arg (commit hash) is required for SHOW"))
		}
		res, err := s.git.Show(arg)
		if err != nil {
			return jsonErrorResult(command, err)
		}
		result = showResultJSON(res)

	case "REFLOG":
		entries, err := s.git.Reflog()
		if err != nil {
			return jsonErrorResult(command, err)
		}
		// Default limit for REFLOG
		if limit <= 0 {
			limit = 50
		}
		result = reflogResultJSON(entries, limit, offset)

	case "STASH_LIST":
		entries, err := s.git.StashList()
		if err != nil {
			return jsonErrorResult(command, err)
		}
		// Default limit for STASH_LIST
		if limit <= 0 {
			limit = 10
		}
		result = stashListResultJSON(entries, limit, offset)

	case "MERGE_BASE":
		refs := strings.Split(arg, ",")
		if len(refs) < 2 {
			return jsonErrorResult(command, fmt.Errorf("MERGE_BASE requires two refs separated by comma: 'ref1,ref2'"))
		}
		base, err := s.git.MergeBase(strings.TrimSpace(refs[0]), strings.TrimSpace(refs[1]))
		if err != nil {
			return jsonErrorResult(command, err)
		}
		result = mergeBaseResultJSON(base, refs[0], refs[1])

	case "READ_SEARCH":
		if arg == "" {
			return jsonErrorResult(command, fmt.Errorf("arg (pattern) is required for READ_SEARCH"))
		}
		var paths []string
		if filter != "" {
			paths = append(paths, filter)
		}
		res, err := s.git.Search(arg, contextSearch, beforeSearch, afterSearch, paths...)
		if err != nil {
			return jsonErrorResult(command, err)
		}

		lineCount := strings.Count(res, "\n")
		if len(res) > 5000 && filter == "" {
			// Summarize large results
			lines := strings.Split(res, "\n")
			var files []string
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "--") || strings.Contains(line, ":") || strings.Contains(line, "-") {
					continue
				}
				// Heading lines in git grep don't have : or - separator
				files = append(files, line)
			}
			b, _ := json.Marshal(map[string]interface{}{
				"matches":            lineCount,
				"pattern":            arg,
				"files_with_matches": files,
				"message":            "Result too large. Showing summary. Use 'filter' parameter with a filename to see exact lines.",
			})
			result = string(b)
		} else {
			result = fmt.Sprintf(`{"matches":%d, "pattern":%q, "results":%q}`, lineCount, arg, res)
		}

	case "CAT_FILE":
		path := getStringParam(params, "path", "")
		revision := getStringParam(params, "revision", "HEAD")
		if path == "" {
			return jsonErrorResult(command, fmt.Errorf("path is required for CAT_FILE"))
		}
		res, err := s.git.CatFile(revision, path)
		if err != nil {
			return jsonErrorResult(command, err)
		}
		result = fmt.Sprintf(`{"path":%q, "revision":%q, "content":%q}`, path, revision, res)

	case "LIST_TREE":
		path := getStringParam(params, "path", "")
		revision := getStringParam(params, "revision", "HEAD")
		recursive := false
		if v, ok := params["recursive"].(bool); ok {
			recursive = v
		}

		// Improve directory handling
		files, err := s.git.ListTree(revision, path, recursive)
		if err == nil && len(files) == 1 && files[0] == path && path != "" && !strings.HasSuffix(path, "/") {
			files, err = s.git.ListTree(revision, path+"/", recursive)
		}

		if err != nil {
			return jsonErrorResult(command, err)
		}
		result = formatFileListJSON(files, limit, offset)

	case "LIST_BACKUPS":
		backups, err := s.git.ListBackups()
		if err != nil {
			return jsonErrorResult(command, err)
		}
		result = formatBackupListJSON(backups)

	case "STASH_DIFF":
		raw, err := s.git.StashDiff(arg)
		if err != nil {
			return jsonErrorResult(command, err)
		}
		res := SanitizeDiff(raw, offset, limit)
		result = diffResultJSON(res)

	case "READ_CONFIG":
		b, _ := json.Marshal(map[string]interface{}{
			"config_path": config.GlobalConfigPath(),
			"content":     s.cfg,
		})
		result = string(b)

	case "LIST_MODELS":
		b, _ := json.Marshal(map[string]interface{}{
			"provider": s.cfg.LLM.Provider,
			"models":   []string{s.cfg.LLM.Model},
			"message":  "Models are configured statically via config file. Showing current configured model.",
		})
		result = string(b)

	case "JOB_RESULT":
		if arg == "" {
			return jsonErrorResult(command, fmt.Errorf("arg (job_id) is required"))
		}
		j, ok := s.getBgJob(arg)
		if !ok {
			return jsonErrorResult(command, fmt.Errorf("job not found: %s", arg))
		}
		result = bgJobResultJSON(j)

	default:
		return jsonErrorResult("git_read", fmt.Errorf("unknown command: %s", command))
	}

	if err != nil {
		s.sendErrorNotification("git_read", command+" failed", map[string]any{"error": err.Error()})
		return jsonErrorResult(command, err)
	}

	s.sendSuccessNotification("git_read", command+" completed", nil)
	return mcpgo.NewToolResultText(result), nil
}

// handleDiffCommand handles READ_DIFF, READ_DIFF_STAGED with optional range syntax.
func (s *Server) handleDiffCommand(arg string, limit, offset int, cachedFlag string, fileFilter string, compact bool) (string, error) {
	if strings.HasPrefix(arg, "..") || strings.HasPrefix(arg, "...") {
		current, err := s.git.CurrentBranch()
		if err != nil {
			return "", err
		}
		target := arg
		mode := ""
		if strings.HasPrefix(arg, "...") {
			mode = "..."
			target = strings.TrimPrefix(arg, "...")
		} else {
			mode = ".."
			target = strings.TrimPrefix(arg, "..")
		}
		raw, err := s.git.DiffRange(current, target, mode)
		if err != nil {
			return "", err
		}
		res := SanitizeDiff(raw, offset, limit)
		res.Mode = mode
		res.Base = current
		res.Target = target
		return diffResultJSON(res), nil
	}

	var raw string
	var err error
	if arg != "" {
		if cachedFlag != "" {
			raw, err = s.git.DiffStaged(arg)
		} else {
			raw, err = s.git.Diff(arg)
		}
	} else {
		if cachedFlag != "" {
			raw, err = s.git.DiffStaged()
		} else {
			raw, err = s.git.Diff()
		}
	}
	if err != nil {
		return "", err
	}

	if fileFilter != "" {
		raw = filterDiffByFile(raw, fileFilter)
	}

	if compact {
		raw = filterDiffCompact(raw)
	}

	res := SanitizeDiff(raw, offset, limit)
	return diffResultJSON(res), nil
}

// handleDiffStatCommand handles stats-only diff output.
func (s *Server) handleDiffStatCommand(arg string, cachedFlag ...string) (string, error) {
	var raw string
	var err error
	if len(cachedFlag) > 0 && cachedFlag[0] != "" {
		if arg != "" {
			raw, err = s.git.DiffStatStaged(arg)
		} else {
			raw, err = s.git.DiffStatStaged()
		}
	} else {
		if arg != "" {
			raw, err = s.git.DiffStat(arg)
		} else {
			raw, err = s.git.DiffStat()
		}
	}
	if err != nil {
		return "", err
	}
	return diffStatJSON(raw), nil
}

func diffStatJSON(raw string) string {
	files := 0
	additions := 0
	deletions := 0
	var fileList []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "-") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) == 2 {
			files++
			fname := strings.TrimSpace(parts[0])
			fileList = append(fileList, fname)
			stats := strings.TrimSpace(parts[1])
			additions += strings.Count(stats, "+")
			deletions += strings.Count(stats, "-")
		}
	}
	resp, _ := json.Marshal(map[string]interface{}{
		"files":     files,
		"additions": additions,
		"deletions": deletions,
		"file_list": fileList,
	})
	return string(resp)
}

func (s *Server) handleWhatChangedCommand(filterArg string, filterMode WhatChangedFilter, useLLM bool) (string, error) {
	var stagedRaw, unstagedRaw string
	var err error

	switch filterMode {
	case WhatChangedFilterStaged:
		stagedRaw, err = s.git.DiffStaged()
		unstagedRaw = ""
	case WhatChangedFilterUnstaged:
		stagedRaw = ""
		unstagedRaw, err = s.git.Diff()
	case WhatChangedFilterAll, "":
		stagedRaw, err = s.git.DiffStaged()
		if err != nil {
			stagedRaw = ""
		}
		unstagedRaw, err = s.git.Diff()
		if err != nil {
			unstagedRaw = ""
		}
	default:
		return "", fmt.Errorf("invalid filter: %s (use: all, staged, unstaged)", filterMode)
	}
	if err != nil && stagedRaw == "" && unstagedRaw == "" {
		return "", err
	}

	statsRaw, err := s.git.DiffStat()
	if err != nil {
		statsRaw = ""
	}

	var summary string
	var llmUsed bool

	if useLLM && s.llm != nil {
		avail := s.llm.IsAvailable()
		if avail {
			diffRaw := stagedRaw
			if diffRaw != "" && unstagedRaw != "" {
				diffRaw += "\n" + unstagedRaw
			} else if unstagedRaw != "" {
				diffRaw = unstagedRaw
			}

			if diffRaw != "" {
				cleanDiff := filters.FilterDiffNoise(diffRaw)
				lines := strings.Split(cleanDiff, "\n")
				maxLines := 200
				if len(lines) > maxLines {
					cleanDiff = strings.Join(lines[:maxLines], "\n")
					cleanDiff += "\n... truncated"
				}

				done := make(chan string, 1)
				go func() {
					defer func() {
						if r := recover(); r != nil {
							done <- ""
						}
					}()
					chunk := domain.DiffChunk{
						Files: []string{},
						Diff:   cleanDiff,
					}
					result, llmErr := s.llm.GenerateChunkMessage(chunk)
					if llmErr == nil && result != "" {
						done <- extractSummary(result)
					} else {
						done <- ""
					}
				}()

				select {
				case summary = <-done:
					llmUsed = summary != ""
				case <-time.After(10 * time.Second):
					llmUsed = false
				}
			}
		}
	}

	return whatChangedJSONWithSummary(statsRaw, summary, string(filterMode), llmUsed), nil
}

func (s *Server) handleDiffAllCommand(arg string, limit, offset int, fileFilter string, compact bool) (string, error) {
	var stagedRaw, unstagedRaw string
	var err error
	if arg != "" {
		stagedRaw, err = s.git.DiffStaged(arg)
		if err != nil {
			stagedRaw = ""
		}
		unstagedRaw, err = s.git.Diff(arg)
		if err != nil {
			unstagedRaw = ""
		}
	} else {
		stagedRaw, err = s.git.DiffStaged()
		if err != nil {
			stagedRaw = ""
		}
		unstagedRaw, err = s.git.Diff()
		if err != nil {
			unstagedRaw = ""
		}
	}

	var sb strings.Builder
	if stagedRaw != "" {
		sb.WriteString(StagedMarker + "\n")
		sb.WriteString(stagedRaw)
		sb.WriteString("\n")
	}
	if sb.Len() > 0 && unstagedRaw != "" {
		sb.WriteString(UnstagedMarker + "\n")
	}
	if unstagedRaw != "" {
		sb.WriteString(unstagedRaw)
	}

	raw := sb.String()

	if fileFilter != "" {
		raw = filterDiffByFile(raw, fileFilter)
	}

	if compact {
		raw = filterDiffCompact(raw)
	}

	res := SanitizeDiff(raw, offset, limit)
	res.Mode = "all"
	return diffResultJSON(res), nil
}

func (s *Server) handleLogCommand(arg, pattern string, limit, offset int, filter string) (string, error) {
	var raw string
	var err error
	var msg string

	if arg != "" {
		if !strings.Contains(arg, "..") {
			arg = arg + "..HEAD"
		}
		raw, err = s.git.Log(limit, pattern, arg)
	} else {
		raw, err = s.git.Log(limit, pattern)
	}

	if err == nil && raw == "" && pattern != "" {
		// Fallback: search across all branches if pattern provided and no results in current scope
		raw, err = s.git.Log(limit, pattern, "--all")
		if err == nil && raw != "" {
			msg = "No results in current branch. Showing results from all branches."
		}
	}

	if err != nil {
		return "", err
	}

	res := SanitizeLog(raw, offset, limit)
	res.Message = msg
	if filter != "" {
		res.Commits = filterCommits(res.Commits, filter)
	}
	return logResultJSON(res), nil
}
