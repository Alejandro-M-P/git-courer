package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/shared"
	"github.com/Alejandro-M-P/git-courer/internal/workflow"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// Handler holds dependencies for core domain MCP handlers (status, diff, commit, amend, revert).
type Handler struct {
	git            ports.Git
	commitSvc      *workflow.CommitService
	reviewWorkflow *workflow.Workflow
	llm            ports.LLM
	provider       string // "ollama" for local, anything else for cloud

	jobs sync.Map // job_id → *domain.OperationPlan
}

// NewHandler creates a new core domain Handler.
func NewHandler(
	git ports.Git,
	commitSvc *workflow.CommitService,
	reviewWorkflow *workflow.Workflow,
	llm ports.LLM,
	jobs *sync.Map,
	provider string,
) *Handler {
	h := &Handler{
		git:            git,
		commitSvc:      commitSvc,
		reviewWorkflow: reviewWorkflow,
		llm:            llm,
		provider:       provider,
	}
	if jobs != nil {
		h.jobs = *jobs
	}
	return h
}

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

	result := formatStatusJSON(status, limit, offset, filter)
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
		res.Mode = ".."
		res.Base = current
		res.Target = branch
		result = diffResultJSON(res)
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
		result = diffResultJSON(res)
	} else {
		result, err = h.handleDiffCommand(path, limit, offset, "", filter)
	}

	if err != nil {
		return shared.JSONErrorResult("diff", err)
	}

	return mcpgo.NewToolResultText(result), nil
}

// ─── HandleAmend ─────────────────────────────────────────────────────

func (h *Handler) HandleAmend(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	dryRun := false
	if v, ok := params["dry_run"].(bool); ok {
		dryRun = v
	}
	confirmed := false
	if v, ok := params["confirmed"].(bool); ok {
		confirmed = v
	}

	if dryRun {
		impact, _ := shared.ComputeImpact("amend", params)
		jsonBytes, _ := json.Marshal(impact)
		return mcpgo.NewToolResultText(string(jsonBytes)), nil
	}
	_ = confirmed

	out, err := h.git.Amend(shared.GetStringParam(params, "commit_message", ""), git.SplitPaths(shared.GetStringParam(params, "target_paths", "")))
	if err != nil {
		return shared.JSONErrorResult("AMEND", err)
	}
	return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("AMEND", true, out, "use backup RESTORE to undo if needed")), nil
}

// ─── HandleRevert ────────────────────────────────────────────────────

func (h *Handler) HandleRevert(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)

	if result, err := shared.ValidateRequiredParam(params, "target_commit", "REVERT"); result != nil || err != nil {
		return result, err
	}

	dryRun := false
	if v, ok := params["dry_run"].(bool); ok {
		dryRun = v
	}
	confirmed := false
	if v, ok := params["confirmed"].(bool); ok {
		confirmed = v
	}

	if dryRun {
		impact, _ := shared.ComputeImpact("revert", params)
		jsonBytes, _ := json.Marshal(impact)
		return mcpgo.NewToolResultText(string(jsonBytes)), nil
	}
	_ = confirmed

	out, err := h.git.Revert(shared.GetStringParam(params, "target_commit", ""))
	if err != nil {
		return shared.JSONErrorResult("REVERT", err)
	}
	return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("REVERT", true, out, "use backup RESTORE to undo if needed")), nil
}

// ─── HandleCommit (PREVIEW/APPLY/ABORT/REGENERATE) ──────────────

func (h *Handler) HandleCommit(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(shared.GetStringParam(params, "command", "PREVIEW"))
	instruction := shared.GetStringParam(params, "instruction", "")

	switch command {
	case "PREVIEW":
		if h.commitSvc == nil {
			return shared.JSONErrorResult("PREVIEW", fmt.Errorf("commit service not available"))
		}
		plan, err := h.commitSvc.PreparePlan(instruction, "")
		if err != nil {
			return shared.JSONErrorResult("PREVIEW", err)
		}
		jobID := fmt.Sprintf("commit-%d", timeNowMs())
		h.jobs.Store(jobID, plan)
		preview := commitPlanJSON(plan)
		result := fmt.Sprintf(`{"status":"pending","job_id":%q,"plan":%s}`, jobID, preview)
		return mcpgo.NewToolResultText(result), nil

	case "APPLY":
		jobID := shared.GetStringParam(params, "job_id", "")
		if jobID == "" {
			return shared.JSONErrorResult("APPLY", fmt.Errorf("job_id is required for APPLY"))
		}
		v, ok := h.jobs.Load(jobID)
		if !ok {
			return shared.JSONErrorResult("APPLY", fmt.Errorf("job not found: %s", jobID))
		}
		plan := v.(*domain.OperationPlan)
		out, err := h.commitSvc.ExecuteFromPlan(plan.Messages, plan.Chunks, plan.DeletedFiles, plan.Instruction)
		if err != nil {
			return shared.JSONErrorResult("APPLY", err)
		}
		h.jobs.Delete(jobID)
		return mcpgo.NewToolResultText(out), nil

	case "ABORT":
		jobID := shared.GetStringParam(params, "job_id", "")
		if jobID != "" {
			h.jobs.Delete(jobID)
		}
		return mcpgo.NewToolResultText(`{"status":"aborted"}`), nil

	case "REGENERATE":
		jobID := shared.GetStringParam(params, "job_id", "")
		if jobID == "" {
			return shared.JSONErrorResult("REGENERATE", fmt.Errorf("job_id is required for REGENERATE"))
		}
		v, ok := h.jobs.Load(jobID)
		if !ok {
			return shared.JSONErrorResult("REGENERATE", fmt.Errorf("job not found: %s", jobID))
		}
		plan := v.(*domain.OperationPlan)
		feedback := shared.GetStringParam(params, "feedback", "")
		newPlan, err := h.commitSvc.PreparePlan(plan.Instruction, feedback)
		if err != nil {
			return shared.JSONErrorResult("REGENERATE", err)
		}
		h.jobs.Store(jobID, newPlan)
		preview := commitPlanJSON(newPlan)
		result := fmt.Sprintf(`{"status":"regenerated","job_id":%q,"plan":%s}`, jobID, preview)
		return mcpgo.NewToolResultText(result), nil

	default:
		return shared.JSONErrorResult(command, fmt.Errorf("unknown commit command: %s", command))
	}
}

func timeNowMs() int64 { return timeNowMsFn() }

var timeNowMsFn = func() int64 { return 0 }

// commitPlanJSON formats an OperationPlan as a structured JSON preview.
func commitPlanJSON(plan *domain.OperationPlan) string {
	files := gatherFilesFromChunks(plan.Chunks)
	structuredPreview := map[string]interface{}{
		"header":  "Review commit details",
		"sections": commitSections(plan),
		"actions":  commitActions(),
	}
	options := []string{"Execute", "Regenerate message", "Edit message", "Cancel"}

	return shared.MustJSON(map[string]interface{}{
		"status":             "pending_approval",
		"show_to_user":       "IMPORTANT: Display ALL fields below to the user before asking for confirmation. Do not summarize.",
		"preview":            plan.Preview,
		"messages":           plan.Messages,
		"files":              files,
		"reasoning":          plan.Reasoning,
		"options":            options,
		"structured_preview": structuredPreview,
	})
}

func gatherFilesFromChunks(chunks [][]string) []string {
	seen := make(map[string]bool)
	var files []string
	for _, chunk := range chunks {
		for _, f := range chunk {
			if !seen[f] {
				seen[f] = true
				files = append(files, f)
			}
		}
	}
	return files
}

func commitSections(plan *domain.OperationPlan) []map[string]interface{} {
	sections := []map[string]interface{}{
		{"title": "Operation", "content": fmt.Sprintf("Commit with %d message(s)", len(plan.Messages)), "type": "text"},
	}

	if len(plan.Messages) > 0 {
		sections = append(sections, map[string]interface{}{
			"title": "Messages", "content": strings.Join(plan.Messages, "\n"), "type": "code",
		})
	}

	files := gatherFilesFromChunks(plan.Chunks)
	if len(files) > 0 {
		sections = append(sections, map[string]interface{}{
			"title": "Files", "content": strings.Join(files, "\n"), "type": "list",
		})
	}
	if plan.Reasoning != "" {
		sections = append(sections, map[string]interface{}{
			"title": "Reasoning", "content": plan.Reasoning, "type": "text",
		})
	}

	sections = append(sections, map[string]interface{}{
		"title": "Impact", "content": fmt.Sprintf("This commit will modify %d file(s).", len(files)), "type": "warning",
	})
	return sections
}

func commitActions() []map[string]interface{} {
	return []map[string]interface{}{
		{"label": "Execute", "key": "apply"},
		{"label": "Regenerate message", "key": "regenerate"},
		{"label": "Edit message", "key": "edit"},
		{"label": "Cancel", "key": "abort"},
	}
}

// ─── Helpers ported from handlers_read_diff.go & handlers_json_*.go ─────

func formatStatusJSON(s domain.Status, limit, offset int, filter string) string {
	files := s.Files
	if filter != "" {
		var filtered []domain.FileStatus
		for _, f := range files {
			if shared.MatchesFilter(f.Path, filter) {
				filtered = append(filtered, f)
			}
		}
		files = filtered
	}

	total := len(files)
	if offset < 0 {
		offset = 0
	}
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := files[offset:end]
	truncated := end < total

	nextOffset := 0
	if truncated {
		nextOffset = end
	}

	type fileItem struct {
		Path   string `json:"path"`
		Status string `json:"status"`
		Staged bool   `json:"staged"`
	}

	fItems := make([]fileItem, 0, len(page))
	for _, f := range page {
		fItems = append(fItems, fileItem{
			Path:   f.Path,
			Status: f.Status,
			Staged: f.Staged,
		})
	}

	resp, _ := json.Marshal(map[string]interface{}{
		"branch":       s.Branch,
		"ahead":        s.Ahead,
		"behind":       s.Behind,
		"has_upstream": s.HasUpstream,
		"clean":        s.IsClean,
		"total":        total,
		"returned":     len(page),
		"offset":       offset,
		"truncated":    truncated,
		"next_offset":  nextOffset,
		"staged":       s.Staged,
		"modified":     s.Modified,
		"untracked":    s.Untracked,
		"files":        fItems,
	})
	return string(resp)
}

func diffResultJSON(res shared.DiffResult) string {
	resp, _ := json.Marshal(map[string]interface{}{
		"diff":                res.Diff,
		"total_lines":         res.TotalLines,
		"lines_shown":         res.LinesShown,
		"offset":              res.Offset,
		"truncated":           res.Truncated,
		"next_offset":         res.NextOffset,
		"filtered_file":       res.Filtered,
		"noise_lines_removed": res.NoiseLinesRemoved,
		"mode":                res.Mode,
		"base":                res.Base,
		"target":              res.Target,
	})
	return string(resp)
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
		res.Mode = mode
		res.Base = current
		res.Target = target
		return diffResultJSON(res), nil
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
		raw = filterDiffByFile(raw, fileFilter)
	}

	res := shared.SanitizeDiffForProvider(raw, offset, limit, h.provider)
	return diffResultJSON(res), nil
}

// filterDiffByFile filters diff output to lines matching a given file pattern.
func filterDiffByFile(diff string, fileFilter string) string {
	if diff == "" || fileFilter == "" {
		return diff
	}
	lines := strings.Split(diff, "\n")
	var sb strings.Builder
	inFile := false

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") || strings.HasPrefix(line, "index ") {
			if strings.Contains(line, fileFilter) {
				inFile = true
				sb.WriteString(line)
				sb.WriteByte('\n')
			} else {
				inFile = false
			}
			continue
		}
		if inFile {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
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
