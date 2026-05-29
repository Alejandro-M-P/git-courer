// Package core implements the core domain MCP handlers for git operations.
// This file contains diff and status query handlers (read-only operations).
package core

import (
	"context"
	"strings"

	"github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/shared"
	"github.com/Alejandro-M-P/git-courer/internal/infra/chunkers"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// ─── HandleStatus ────────────────────────────────────────────────────

func (h *Handler) HandleStatus(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	if result, err := shared.ValidateKnownParams(params, []string{"filter", "limit", "offset"}); result != nil || err != nil {
		return result, err
	}

	limit, offset := shared.ParsePagination(params)
	filter := shared.GetStringParam(params, "filter", "")

	if limit <= 0 {
		limit = 100
	}

	status, sErr := h.git.Status()
	if sErr != nil {
		return shared.JSONErrorResult("status", sErr)
	}

	result := shared.FormatStatusJSON(status, limit, offset, filter)
	return mcpgo.NewToolResultText(result), nil
}

// ─── HandleDiff ──────────────────────────────────────────────────────

func (h *Handler) HandleDiff(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	if result, err := shared.ValidateKnownParams(params, []string{"target_paths", "staged", "branch", "filter", "limit", "offset"}); result != nil || err != nil {
		return result, err
	}

	path := shared.GetStringParam(params, "target_paths", "")
	branch := shared.GetStringParam(params, "branch", "")
	staged := false
	if v, ok := params["staged"].(bool); ok {
		staged = v
	}

	limit, offset := shared.ParsePagination(params)
	filter := shared.GetStringParam(params, "filter", "")

	if limit <= 0 {
		limit = 200
	}

	var result string
	var err error

	// If branch is set, compare current branch against target
	if branch != "" {
		current, bErr := h.git.CurrentBranch()
		if bErr != nil {
			return shared.JSONErrorResult("diff", bErr)
		}
		var raw string
		if strings.HasPrefix(branch, "...") || strings.HasPrefix(branch, "..") {
			raw, err = h.git.DiffRange(current, strings.TrimLeft(branch, ". "), strings.TrimLeft(branch, ".")[:3])
		} else {
			raw, err = h.git.DiffRange(current, branch, "..")
		}
		if err != nil {
			return shared.JSONErrorResult("diff", err)
		}
		res := shared.SanitizeDiffForProvider(raw, offset, limit, h.provider)
		if h.contentProvider != nil {
			res.Annotated = chunkers.AnnotateDiffForRead(raw, h.contentProvider)
		}
		result = shared.DiffResultJSON(res)
	} else if staged {
		var raw string
		paths := dropEmpty(strings.Split(path, " "))
		if len(paths) > 0 {
			raw, err = h.git.DiffStaged(paths...)
		} else {
			raw, err = h.git.DiffStaged()
		}
		if err != nil {
			return shared.JSONErrorResult("diff", err)
		}
		res := shared.SanitizeDiffForProvider(raw, offset, limit, h.provider)
		if h.contentProvider != nil {
			res.Annotated = chunkers.AnnotateDiffForRead(raw, h.contentProvider)
		}
		result = shared.DiffResultJSON(res)
	} else {
		result, err = h.handleDiffCommand(path, limit, offset, "", filter)
	}

	if err != nil {
		return shared.JSONErrorResult("diff", err)
	}

	return mcpgo.NewToolResultText(result), nil
}

func (h *Handler) handleDiffCommand(path string, limit, offset int, cachedFlag string, fileFilter string) (string, error) {
	// Handle range syntax: .. or ... prefix means compare against target.
	if strings.HasPrefix(path, "..") || strings.HasPrefix(path, "...") {
		current, err := h.git.CurrentBranch()
		if err != nil {
			return "", err
		}
		target := path
		mode := ""
		if strings.HasPrefix(path, "...") {
			mode = "..."
			target = strings.TrimPrefix(path, "...")
		} else {
			mode = ".."
			target = strings.TrimPrefix(path, "..")
		}
		raw, err := h.git.DiffRange(current, target, mode)
		if err != nil {
			return "", err
		}
		res := shared.SanitizeDiffForProvider(raw, offset, limit, h.provider)
		if h.contentProvider != nil {
			res.Annotated = chunkers.AnnotateDiffForRead(raw, h.contentProvider)
		}
		res.Mode = mode
		res.Base = current
		res.Target = target
		return shared.DiffResultJSON(res), nil
	}

	var raw string
	var err error
	paths := dropEmpty(strings.Split(path, " "))

	if len(paths) > 0 {
		if cachedFlag != "" {
			raw, err = h.git.DiffStaged(paths...)
		} else {
			raw, err = h.git.Diff(paths...)
		}
	} else {
		if cachedFlag != "" {
			raw, err = h.git.DiffStaged()
		} else {
			raw, err = h.git.Diff()
		}
	}
	if err != nil {
		return "", err
	}

	if fileFilter != "" {
		raw = shared.FilterDiffByFile(raw, fileFilter)
	}

	res := shared.SanitizeDiffForProvider(raw, offset, limit, h.provider)
	if h.contentProvider != nil {
		res.Annotated = chunkers.AnnotateDiffForRead(raw, h.contentProvider)
	}
	return shared.DiffResultJSON(res), nil
}

func dropEmpty(in []string) []string {
	var out []string
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}