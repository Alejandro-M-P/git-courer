package history

import (
	"context"
	"fmt"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/shared"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

type Handler struct {
	git ports.Git
}

func NewHandler(git ports.Git) *Handler {
	return &Handler{git: git}
}

func (h *Handler) HandleHistory(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(shared.GetStringParam(params, "command", "LOG"))

	if result, err := shared.ValidateKnownParams(params, []string{
		"command", "target_commit", "target_paths", "pattern", "filter", "limit", "offset",
	}); result != nil || err != nil {
		return result, err
	}

	revision := shared.GetStringParam(params, "target_commit", "")
	path := shared.GetStringParam(params, "target_paths", "")

	limit, offset := shared.ParsePagination(params)
	filter := shared.GetStringParam(params, "filter", "")
	pattern := shared.GetStringParam(params, "pattern", "")

	if limit <= 0 {
		limit = 20
	}

	var result string
	var err error

	switch command {
	case "LOG":
		result, err = h.handleLogCommand(revision, path, pattern, limit, offset, filter)

	case "REFLOG":
		entries, rErr := h.git.Reflog()
		if rErr != nil {
			return shared.JSONErrorResult(command, rErr)
		}
		if limit <= 0 {
			limit = 50
		}
		result = reflogResultJSON(entries, limit, offset)

	default:
		validCommands := []string{"LOG", "REFLOG"}
		hint := shared.SuggestCommand(command, validCommands)
		if hint != "" {
			return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s. Did you mean %s?", command, hint))
		}
		return shared.JSONErrorResult(command, fmt.Errorf("unknown command: %s", command))
	}

	if err != nil {
		return shared.JSONErrorResult(command, err)
	}

	return mcpgo.NewToolResultText(result), nil
}

func (h *Handler) HandleBlame(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	if result, err := shared.ValidateKnownParams(params, []string{"target_paths", "limit", "offset"}); result != nil || err != nil {
		return result, err
	}

	path := shared.GetStringParam(params, "target_paths", "")
	if path == "" {
		return shared.JSONErrorResult("blame", fmt.Errorf("target_paths is required for blame"))
	}

	limit, offset := shared.ParsePagination(params)
	if limit <= 0 {
		limit = 50
	}

	lines, bErr := h.git.Blame(path)
	if bErr != nil {
		return shared.JSONErrorResult("blame", bErr)
	}
	result := blameResultJSON(path, lines, limit, offset)
	return mcpgo.NewToolResultText(result), nil
}

func (h *Handler) handleLogCommand(revision, pathArg, pattern string, limit, offset int, filter string) (string, error) {
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
		raw, err = h.git.Log(limit, pattern, scope)
	} else {
		raw, err = h.git.Log(limit, pattern)
	}

	if err == nil && raw == "" && pattern != "" {
		// Fallback: search across all branches if pattern provided and no results in current scope
		raw, err = h.git.Log(limit, pattern, "--all")
		if err == nil && raw != "" {
			msg = "No results in current branch. Showing results from all branches."
		}
	}

	if err != nil {
		return "", err
	}

	res := shared.SanitizeLog(raw, offset, limit)
	res.Message = msg
	if filter != "" {
		res.Commits = shared.FilterCommits(res.Commits, filter)
	}
	return logResultJSON(res), nil
}