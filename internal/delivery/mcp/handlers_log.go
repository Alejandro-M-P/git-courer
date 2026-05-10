package mcp

import (
	"context"
	"fmt"
	"strings"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func (s *Server) handleGitLog(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(getStringParam(params, "command", "READ_LOG"))

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
	filter := getStringParam(params, "filter", "")
	pattern := getStringParam(params, "pattern", "")

	if limit <= 0 {
		limit = 20
	}

	var result string
	var err error

	switch command {
	case "READ_LOG":
		result, err = s.handleLogCommand(arg, pattern, limit, offset, filter)

	case "READ_BRANCHES":
		raw, err := s.git.ListBranches(arg)
		if err != nil {
			return jsonErrorResult(command, err)
		}
		current, _ := s.git.CurrentBranch()
		list := SanitizeBranchList(raw)
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

	case "REMOTE_BRANCH_LIST":
		raw, err := s.git.ListBranches("-r")
		if err != nil {
			return jsonErrorResult(command, err)
		}
		list := SanitizeBranchList(raw)
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

	case "BLAME":
		if arg == "" {
			return jsonErrorResult(command, fmt.Errorf("arg (filepath) is required for BLAME"))
		}
		lines, err := s.git.Blame(arg)
		if err != nil {
			return jsonErrorResult(command, err)
		}
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
		if limit <= 0 {
			limit = 50
		}
		result = reflogResultJSON(entries, limit, offset)

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

		res, err := s.git.Search(arg, contextSearch, beforeSearch, afterSearch, paths...)
		if err != nil {
			return jsonErrorResult(command, err)
		}

		lineCount := strings.Count(res, "\n")
		if len(res) > 5000 && filter == "" {
			lines := strings.Split(res, "\n")
			var files []string
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "--") || strings.Contains(line, ":") || strings.Contains(line, "-") {
					continue
				}
				files = append(files, line)
			}
			result = mustJSON(map[string]interface{}{
				"matches":            lineCount,
				"pattern":            arg,
				"files_with_matches": files,
				"message":            "Result too large. Showing summary. Use 'filter' parameter with a filename to see exact lines.",
			})
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

		files, err := s.git.ListTree(revision, path, recursive)
		if err == nil && len(files) == 1 && files[0] == path && path != "" && !strings.HasSuffix(path, "/") {
			files, err = s.git.ListTree(revision, path+"/", recursive)
		}

		if err != nil {
			return jsonErrorResult(command, err)
		}
		result = formatFileListJSON(files, limit, offset)

	default:
		return jsonErrorResult("git_log", fmt.Errorf("unknown command: %s", command))
	}

	if err != nil {
		s.sendErrorNotification("git_log", command+" failed", map[string]any{"error": err.Error()})
		return jsonErrorResult(command, err)
	}

	s.sendSuccessNotification("git_log", command+" completed", nil)
	return mcpgo.NewToolResultText(result), nil
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
