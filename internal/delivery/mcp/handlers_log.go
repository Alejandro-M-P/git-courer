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

	// Validate known params — no 'arg'
	if result, err := validateKnownParams(params, []string{
		"command", "revision", "path", "pattern", "filter", "limit", "offset", "recursive",
		"context", "before", "after",
	}); result != nil || err != nil {
		return result, err
	}

	revision := getStringParam(params, "revision", "")
	path := getStringParam(params, "path", "")

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
		result, err = s.handleLogCommand(revision, path, pattern, limit, offset, filter)

	case "READ_BRANCHES":
		raw, err := s.git.ListBranches(path)
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
		tags, err := s.git.ListTags(path)
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
		if path == "" {
			return jsonErrorResult(command, fmt.Errorf("path is required for BLAME"))
		}
		lines, err := s.git.Blame(path)
		if err != nil {
			return jsonErrorResult(command, err)
		}
		if limit <= 0 {
			limit = 50
		}
		result = blameResultJSON(path, lines, limit, offset)

	case "SHOW":
		if revision == "" {
			return jsonErrorResult(command, fmt.Errorf("revision (commit hash) is required for SHOW"))
		}
		res, err := s.git.Show(revision)
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
		refs := strings.Split(revision, ",")
		if len(refs) < 2 {
			return jsonErrorResult(command, fmt.Errorf("MERGE_BASE requires two refs separated by comma: 'ref1,ref2'"))
		}
		base, err := s.git.MergeBase(strings.TrimSpace(refs[0]), strings.TrimSpace(refs[1]))
		if err != nil {
			return jsonErrorResult(command, err)
		}
		result = mergeBaseResultJSON(base, refs[0], refs[1])

	case "READ_SEARCH":
		if pattern == "" {
			return jsonErrorResult(command, fmt.Errorf("pattern is required for READ_SEARCH"))
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

		res, err := s.git.Search(pattern, contextSearch, beforeSearch, afterSearch, paths...)
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
				"pattern":            pattern,
				"files_with_matches": files,
				"message":            "Result too large. Showing summary. Use 'filter' parameter with a filename to see exact lines.",
			})
		} else {
			result = fmt.Sprintf(`{"matches":%d, "pattern":%q, "results":%q}`, lineCount, pattern, res)
		}

	case "CAT_FILE":
		if path == "" {
			return jsonErrorResult(command, fmt.Errorf("path is required for CAT_FILE"))
		}
		catRevision := getStringParam(params, "revision", "HEAD")
		res, err := s.git.CatFile(catRevision, path)
		if err != nil {
			return jsonErrorResult(command, err)
		}
		result = fmt.Sprintf(`{"path":%q, "revision":%q, "content":%q}`, path, catRevision, res)

	case "LIST_TREE":
		listPath := getStringParam(params, "path", "")
		listRevision := getStringParam(params, "revision", "HEAD")
		recursive := false
		if v, ok := params["recursive"].(bool); ok {
			recursive = v
		}

		files, err := s.git.ListTree(listRevision, listPath, recursive)
		if err == nil && len(files) == 1 && files[0] == listPath && listPath != "" && !strings.HasSuffix(listPath, "/") {
			files, err = s.git.ListTree(listRevision, listPath+"/", recursive)
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

func (s *Server) handleLogCommand(revision, pathArg, pattern string, limit, offset int, filter string) (string, error) {
	var raw string
	var err error
	var msg string

	// Build the log scope from revision or path
	scope := revision
	if scope == "" {
		scope = pathArg
	}

	if scope != "" {
		if !strings.Contains(scope, "..") {
			scope = scope + "..HEAD"
		}
		raw, err = s.git.Log(limit, pattern, scope)
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
