package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/adapters/git"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/delivery/mcp/shared"
	"github.com/Alejandro-M-P/git-courer/internal/workflow"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// ─── CommitJob tracks the state of an async commit PREVIEW operation ───

// CommitJobStatus represents the lifecycle of a background commit job.
type CommitJobStatus string

const (
	JobRunning   CommitJobStatus = "running"
	JobDone      CommitJobStatus = "done"
	JobFailed    CommitJobStatus = "failed"
	JobAborted   CommitJobStatus = "aborted"
)

// CommitJob holds the state for an in-flight commit preview or execution.
type CommitJob struct {
	mu        sync.Mutex
	ID        string
	Status    CommitJobStatus
	Progress  string
	Plan      *domain.OperationPlan
	Result    string
	Error     string
	StartedAt time.Time
}

// StatusJSON returns the current job state as a JSON string for MCP responses.
func (j *CommitJob) StatusJSON() string {
	j.mu.Lock()
	defer j.mu.Unlock()

	m := map[string]any{
		"job_id":  j.ID,
		"op":      "commit",
		"status":  string(j.Status),
		"elapsed": time.Since(j.StartedAt).Round(time.Second).String(),
	}
	if j.Progress != "" {
		m["progress"] = j.Progress
	}
	if j.Result != "" {
		m["result"] = json.RawMessage(j.Result)
	}
	if j.Error != "" {
		m["error"] = j.Error
	}
	if j.Plan != nil {
		// Include plan preview when the job is done but hasn't been applied yet
		m["preview"] = json.RawMessage(commitPlanJSON(j.Plan))
	}
	return shared.MustJSON(m)
}

// Handler holds dependencies for core domain MCP handlers (status, diff, commit, amend, revert).
type Handler struct {
	git            ports.Git
	commitSvc      *workflow.CommitService
	reviewWorkflow *workflow.Workflow
	llm            ports.LLM
	provider       string // "ollama" for local, anything else for cloud

	jobs sync.Map // job_id → *CommitJob
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
	// If a shared jobs map is provided (for cross-handler job visibility),
	// adopt it. Otherwise the handler uses its own zero-value sync.Map.
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
		res.Mode = ".."
		res.Base = current
		res.Target = branch
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
		result = shared.DiffResultJSON(res)
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

	if result, err := shared.ValidateKnownParams(params, []string{"commit_message", "target_paths", "confirmed", "dry_run"}); result != nil || err != nil {
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
	if result, err := shared.ValidateKnownParams(params, []string{"target_commit", "confirmed", "dry_run"}); result != nil || err != nil {
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

// ─── HandleCommit (PREVIEW/APPLY/ABORT/REGENERATE/STATUS) ──────────────
//
// The commit pipeline uses an async job pattern for PREVIEW:
//   - PREVIEW: If the commit service is available, starts async plan generation.
//     Returns job_id immediately so the client can poll STATUS.
//     If preparation is fast enough (< 1s threshold for synchronous path), returns result inline.
//   - STATUS:  Polls the state of an in-flight commit job.
//   - APPLY:   Executes a completed plan by job_id.
//   - ABORT:   Cancels a running or pending job.
//   - REGENERATE: Re-runs plan generation with feedback, returns new plan under same job_id.

func (h *Handler) HandleCommit(_ context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(shared.GetStringParam(params, "command", "PREVIEW"))
	instruction := shared.GetStringParam(params, "instruction", "")

	switch command {
	case "PREVIEW":
		return h.handlePreview(instruction)
	case "STATUS":
		return h.handleStatus(params)
	case "APPLY":
		return h.handleApply(params)
	case "ABORT":
		return h.handleAbort(params)
	case "REGENERATE":
		return h.handleRegenerate(params)
	default:
		return shared.JSONErrorResult(command, fmt.Errorf("unknown commit command: %s", command))
	}
}

// previewTimeout is the duration we wait for PREVIEW to complete synchronously.
// If it takes longer, we fall back to async — returns a job_id and the client polls STATUS.
// 45 seconds matches the typical MCP client timeout threshold.
const previewTimeout = 45 * time.Second

// handlePreview tries to generate the commit plan synchronously first.
// If it takes longer than previewTimeout, it falls back to async — returns a
// job_id and the client polls STATUS.
func (h *Handler) handlePreview(instruction string) (*mcpgo.CallToolResult, error) {
	if h.commitSvc == nil {
		return shared.JSONErrorResult("PREVIEW", fmt.Errorf("commit service not available"))
	}

	// Try synchronous path first: run plan generation with a timeout.
	// If it completes within the timeout, return the plan directly — no extra round-trip.
	type planResult struct {
		plan *domain.OperationPlan
		err  error
	}
	ch := make(chan planResult, 1)
	go func() {
		plan, err := h.commitSvc.PreparePlan(instruction, "")
		ch <- planResult{plan: plan, err: err}
	}()

	select {
	case res := <-ch:
		// Fast path: plan generation completed within the timeout.
		if res.err != nil {
			return shared.JSONErrorResult("PREVIEW", res.err)
		}
		jobID := fmt.Sprintf("commit-%d", timeNowMs())
		h.jobs.Store(jobID, &CommitJob{
			ID:        jobID,
			Status:    JobDone,
			Plan:      res.plan,
			StartedAt: time.Now(),
			Progress:  "Plan ready — call APPLY to execute or REGENERATE to revise",
		})
		preview := commitPlanJSON(res.plan)
		result := fmt.Sprintf(`{"status":"pending","job_id":%q,"plan":%s}`, jobID, preview)
		return mcpgo.NewToolResultText(result), nil

	case <-time.After(previewTimeout):
		// Slow path: plan generation is taking too long — fall back to async bg job.
		// Create a running job now; the goroutine above will populate it when done.
		jobID := fmt.Sprintf("commit-%d", timeNowMs())
		job := &CommitJob{
			ID:        jobID,
			Status:    JobRunning,
			StartedAt: time.Now(),
			Progress:  "Generating commit plan...",
		}
		h.jobs.Store(jobID, job)

		// Wire the running goroutine to update this job when it completes.
		go func() {
			res := <-ch // Wait for the original goroutine to finish.
			job.mu.Lock()
			defer job.mu.Unlock()
			if res.err != nil {
				job.Status = JobFailed
				job.Error = res.err.Error()
				log.Printf("[commit] job %s failed: %v", jobID, res.err)
				return
			}
			job.Status = JobDone
			job.Plan = res.plan
			job.Progress = "Plan ready — call APPLY to execute or REGENERATE to revise"
			log.Printf("[commit] job %s done (async): %d messages generated", jobID, len(res.plan.Messages))
		}()

		result := fmt.Sprintf(`{"status":"processing","job_id":%q,"message":"Commit plan is taking longer than expected. Poll STATUS with this job_id to get the result."}`, jobID)
		return mcpgo.NewToolResultText(result), nil
	}
}

// handleStatus returns the current state of a commit job.
func (h *Handler) handleStatus(params map[string]any) (*mcpgo.CallToolResult, error) {
	jobID := shared.GetStringParam(params, "job_id", "")
	if jobID == "" {
		return shared.JSONErrorResult("STATUS", fmt.Errorf("job_id is required for STATUS"))
	}

	v, ok := h.jobs.Load(jobID)
	if !ok {
		return shared.JSONErrorResult("STATUS", fmt.Errorf("job not found: %s", jobID))
	}
	job, ok := v.(*CommitJob)
	if !ok {
		return shared.JSONErrorResult("STATUS", fmt.Errorf("invalid job type for: %s", jobID))
	}

	return mcpgo.NewToolResultText(job.StatusJSON()), nil
}

// handleApply executes a completed commit plan.
func (h *Handler) handleApply(params map[string]any) (*mcpgo.CallToolResult, error) {
	jobID := shared.GetStringParam(params, "job_id", "")
	if jobID == "" {
		return shared.JSONErrorResult("APPLY", fmt.Errorf("job_id is required for APPLY"))
	}

	v, ok := h.jobs.Load(jobID)
	if !ok {
		return shared.JSONErrorResult("APPLY", fmt.Errorf("job not found: %s", jobID))
	}
	job, ok := v.(*CommitJob)
	if !ok {
		return shared.JSONErrorResult("APPLY", fmt.Errorf("invalid job type for: %s", jobID))
	}

	job.mu.Lock()
	switch job.Status {
	case JobRunning:
		job.mu.Unlock()
		// Job still running — tell client to poll STATUS.
		return mcpgo.NewToolResultText(fmt.Sprintf(
			`{"status":"processing","job_id":%q,"message":"Plan still generating. Poll STATUS to wait for completion."}`,
			jobID,
		)), nil
	case JobFailed:
		errMsg := job.Error
		job.mu.Unlock()
		h.jobs.Delete(jobID)
		return shared.JSONErrorResult("APPLY", fmt.Errorf("plan generation failed: %s", errMsg))
	case JobAborted:
		job.mu.Unlock()
		h.jobs.Delete(jobID)
		return shared.JSONErrorResult("APPLY", fmt.Errorf("job was aborted: %s", jobID))
	case JobDone:
		// proceed below
	}
	plan := job.Plan
	job.mu.Unlock()

	if plan == nil {
		h.jobs.Delete(jobID)
		return shared.JSONErrorResult("APPLY", fmt.Errorf("plan is nil for job: %s", jobID))
	}

	out, err := h.commitSvc.ExecuteFromPlan(plan.Messages, plan.Chunks, plan.DeletedFiles, plan.Instruction)
	h.jobs.Delete(jobID) // Clean up after execution
	if err != nil {
		return shared.JSONErrorResult("APPLY", err)
	}
	return mcpgo.NewToolResultText(out), nil
}

// handleAbort cancels a running or pending commit job.
func (h *Handler) handleAbort(params map[string]any) (*mcpgo.CallToolResult, error) {
	jobID := shared.GetStringParam(params, "job_id", "")
	if jobID == "" {
		return mcpgo.NewToolResultText(`{"status":"aborted","message":"No job_id provided, nothing to abort"}`), nil
	}

	v, ok := h.jobs.Load(jobID)
	if !ok {
		// Job already gone or never existed — that's fine for abort.
		return mcpgo.NewToolResultText(fmt.Sprintf(`{"status":"aborted","job_id":%q,"message":"Job not found or already completed"}`, jobID)), nil
	}
	job, ok := v.(*CommitJob)
	if !ok {
		h.jobs.Delete(jobID)
		return mcpgo.NewToolResultText(fmt.Sprintf(`{"status":"aborted","job_id":%q}`, jobID)), nil
	}

	job.mu.Lock()
	job.Status = JobAborted
	job.mu.Unlock()
	h.jobs.Delete(jobID)

	return mcpgo.NewToolResultText(fmt.Sprintf(`{"status":"aborted","job_id":%q}`, jobID)), nil
}

// handleRegenerate re-runs plan generation with optional feedback, keeping the same job_id.
// Uses the same sync-with-timeout pattern as handlePreview.
func (h *Handler) handleRegenerate(params map[string]any) (*mcpgo.CallToolResult, error) {
	jobID := shared.GetStringParam(params, "job_id", "")
	if jobID == "" {
		return shared.JSONErrorResult("REGENERATE", fmt.Errorf("job_id is required for REGENERATE"))
	}

	v, ok := h.jobs.Load(jobID)
	if !ok {
		return shared.JSONErrorResult("REGENERATE", fmt.Errorf("job not found: %s", jobID))
	}
	job, ok := v.(*CommitJob)
	if !ok {
		return shared.JSONErrorResult("REGENERATE", fmt.Errorf("invalid job type for: %s", jobID))
	}

	job.mu.Lock()
	plan := job.Plan
	if plan == nil {
		job.mu.Unlock()
		h.jobs.Delete(jobID)
		return shared.JSONErrorResult("REGENERATE", fmt.Errorf("original plan not found for job: %s", jobID))
	}
	job.Status = JobRunning
	job.Progress = "Regenerating plan with feedback..."
	job.mu.Unlock()

	feedback := shared.GetStringParam(params, "feedback", "")
	instruction := plan.Instruction

	// Try synchronous path first.
	type planResult struct {
		plan *domain.OperationPlan
		err  error
	}
	ch := make(chan planResult, 1)
	go func() {
		newPlan, err := h.commitSvc.PreparePlan(instruction, feedback)
		ch <- planResult{plan: newPlan, err: err}
	}()

	select {
	case res := <-ch:
		// Fast path: regeneration completed within the timeout.
		job.mu.Lock()
		if res.err != nil {
			job.Status = JobFailed
			job.Error = res.err.Error()
			job.mu.Unlock()
			return shared.JSONErrorResult("REGENERATE", res.err)
		}
		job.Status = JobDone
		job.Plan = res.plan
		job.Progress = "Regenerated plan ready — call APPLY to execute"
		job.mu.Unlock()
		preview := commitPlanJSON(res.plan)
		result := fmt.Sprintf(`{"status":"regenerated","job_id":%q,"plan":%s}`, jobID, preview)
		return mcpgo.NewToolResultText(result), nil

	case <-time.After(previewTimeout):
		// Slow path: fall back to async.
		go func() {
			res := <-ch
			job.mu.Lock()
			defer job.mu.Unlock()
			if res.err != nil {
				job.Status = JobFailed
				job.Error = res.err.Error()
				log.Printf("[commit] regenerate job %s failed: %v", jobID, res.err)
				return
			}
			job.Status = JobDone
			job.Plan = res.plan
			job.Progress = "Regenerated plan ready — call APPLY to execute"
			log.Printf("[commit] regenerate job %s done (async): %d messages", jobID, len(res.plan.Messages))
		}()
		result := fmt.Sprintf(`{"status":"processing","job_id":%q,"message":"Regenerating commit plan. Poll STATUS with this job_id."}`, jobID)
		return mcpgo.NewToolResultText(result), nil
	}
}

func timeNowMs() int64 { return timeNowMsFn() }

var timeNowMsFn = func() int64 { return time.Now().UnixMilli() }

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

// ─── Helpers ──────────────────────────────────────────────────────────

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
		raw = filterDiffByFile(raw, fileFilter)
	}

	res := shared.SanitizeDiffForProvider(raw, offset, limit, h.provider)
	return shared.DiffResultJSON(res), nil
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