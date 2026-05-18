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
	"github.com/mark3labs/mcp-go/server"
)

// ─── BgJob tracks background goroutine completion for slow PREVIEW paths ───

// BgJobStatus represents the status of a background job.
type BgJobStatus string

const (
	BgRunning BgJobStatus = "running"
	BgDone    BgJobStatus = "done"
	BgFailed  BgJobStatus = "failed"
)

// BgJob tracks a background goroutine's completion state.
// Plan data lives in ConfirmStore; this also carries the tree snapshot and done signal.
type BgJob struct {
	ID       string
	Status   BgJobStatus
	Error    string
	TreeHash string        // write-once before goroutine
	Message  string        // write-once in goroutine, read after Done
	Done     chan struct{} // make(chan struct{}), closed when goroutine finishes
}

// Handler holds dependencies for core domain MCP handlers (status, diff, commit, amend, revert).
type Handler struct {
	git            ports.Git
	commitSvc      *workflow.CommitService
	reviewWorkflow *workflow.Workflow
	llm            ports.LLM
	provider       string // "ollama" for local, anything else for cloud
	mcpServer      *server.MCPServer

	bgJobs sync.Map // job_id → *BgJob (lightweight: only running/done/failed)
}

// NewHandler creates a new core domain Handler.
func NewHandler(
	git ports.Git,
	commitSvc *workflow.CommitService,
	reviewWorkflow *workflow.Workflow,
	llm ports.LLM,
	provider string,
	mcpServer *server.MCPServer,
) *Handler {
	return &Handler{
		git:            git,
		commitSvc:      commitSvc,
		reviewWorkflow: reviewWorkflow,
		llm:            llm,
		provider:       provider,
		mcpServer:      mcpServer,
	}
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
	if result, err := shared.CheckSafetyGate("amend", dryRun, confirmed); result != nil || err != nil {
		return result, err
	}

	// Auto-create backup before amend for undo safety
	_, _ = h.git.CreateBackup("AMEND", domain.StashNone)

	out, err := h.git.Amend(shared.GetStringParam(params, "commit_message", ""), git.SplitPaths(shared.GetStringParam(params, "target_paths", "")))
	if err != nil {
		return shared.JSONErrorResult("AMEND", err)
	}
	return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("AMEND", true, out, "use backup RESTORE or undo to undo if needed")), nil
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
	if result, err := shared.CheckSafetyGate("revert", dryRun, confirmed); result != nil || err != nil {
		return result, err
	}

	// Auto-create backup before revert for undo safety
	_, _ = h.git.CreateBackup("REVERT", domain.StashNone)

	out, err := h.git.Revert(shared.GetStringParam(params, "target_commit", ""))
	if err != nil {
		return shared.JSONErrorResult("REVERT", err)
	}
	return mcpgo.NewToolResultText(shared.WriteHintedResultJSON("REVERT", true, out, "use backup RESTORE or undo to undo if needed")), nil
}

// ─── HandleCommit (PREVIEW/APPLY/ABORT/REGENERATE/STATUS) ──────────────
//
// The commit pipeline uses the workflow.Workflow engine + ConfirmStore:
//   - PREVIEW:     Runs workflow.Run("commit", instruction, nil). Plan data is stored
//                  in ConfirmStore. Fast path (<45s) returns plan directly; slow path
//                  returns a job_id for polling via STATUS.
//   - STATUS:      Polls background job state. If done, reads plan from ConfirmStore.
//   - APPLY:       Executes the pending plan via workflow.Apply (no job_id needed).
//   - ABORT:       Discards the pending plan via workflow.Abort (no job_id needed).
//   - REGENERATE:  Reads instruction from pending plan, appends feedback, re-runs PREVIEW.

func (h *Handler) HandleCommit(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(shared.GetStringParam(params, "command", "PREVIEW"))
	instruction := shared.GetStringParam(params, "instruction", "")

	switch command {
	case "PREVIEW":
		return h.handlePreview(ctx, params, instruction)
	case "STATUS":
		return h.handleStatus(params)
	case "APPLY":
		return h.handleApply(ctx, params)
	case "ABORT":
		return h.handleAbort()
	case "REGENERATE":
		return h.handleRegenerate(ctx, params, instruction)
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
// Progress notifications are sent when a ProgressToken is present in params.
// Plan data is stored in ConfirmStore (via workflow.Run), not in the job struct.
func (h *Handler) handlePreview(ctx context.Context, params map[string]any, instruction string) (*mcpgo.CallToolResult, error) {
	if h.commitSvc == nil {
		return shared.JSONErrorResult("PREVIEW", fmt.Errorf("commit service not available"))
	}

	// Synchronous WriteTree: capture the current staging area snapshot atomically.
	// If this fails, no BgJob is created — we return immediately.
	treeHash, err := h.git.WriteTree()
	if err != nil {
		return shared.JSONErrorResult("PREVIEW", err)
	}

	// Configure progress callback in workflow
	h.reviewWorkflow.SetProgressCallback(func(step, total int, message string) {
		shared.SendProgress(ctx, h.mcpServer, params, float64(step), float64(total), message)
	})

	type runResult struct {
		result workflow.Result
		err     error
	}
	ch := make(chan runResult, 1)
	go func() {
		result, err := h.reviewWorkflow.Run(ctx, "commit", instruction, nil)
		ch <- runResult{result: result, err: err}
	}()

	select {
	case res := <-ch:
		// Fast path: plan generation completed within the timeout.
		if res.err != nil {
			return shared.JSONErrorResult("PREVIEW", res.err)
		}
		// Send progress: plan ready
		shared.SendProgress(ctx, h.mcpServer, params, 4, shared.ProgressTotal, shared.CommitProgressMessage(shared.ProgressPlan))

		// Store BgJob with TreeHash and Done for consistency with slow path.
		// Fast-path job is immediately complete: Status=BgDone, Message set, Done closed.
		jobID := fmt.Sprintf("commit-%d", time.Now().UnixMilli())
		bgJob := &BgJob{
			ID:       jobID,
			Status:   BgDone,
			TreeHash: treeHash,
			Message:  res.result.Output,
			Done:     make(chan struct{}),
		}
		close(bgJob.Done) // fast path: done immediately
		h.bgJobs.Store(jobID, bgJob)

		// Read the plan from ConfirmStore to format the response
		plan, _ := h.reviewWorkflow.PlanStatus()
		if plan != "" {
			return mcpgo.NewToolResultText(plan), nil
		}
		// Fallback: return the result directly
		return mcpgo.NewToolResultText(res.result.Output), nil

	case <-time.After(previewTimeout):
		// Slow path: plan generation is taking too long — fall back to async bg job.
		// Send progress: still processing
		shared.SendProgress(ctx, h.mcpServer, params, 3, shared.ProgressTotal, shared.CommitProgressMessage(shared.ProgressClassify))

		jobID := fmt.Sprintf("commit-%d", time.Now().UnixMilli())
		bgJob := &BgJob{
			ID:       jobID,
			Status:   BgRunning,
			TreeHash: treeHash,
			Done:     make(chan struct{}),
		}
		h.bgJobs.Store(jobID, bgJob)

		go func() {
			res := <-ch
			j := bgJob
			if res.err != nil {
				j.Status = BgFailed
				j.Error = res.err.Error()
				close(j.Done)
				log.Printf("[commit] job %s failed: %v", jobID, res.err)
				return
			}
			j.Status = BgDone
			j.Message = res.result.Output
			close(j.Done)
			log.Printf("[commit] job %s done (async)", jobID)
		}()

		return mcpgo.NewToolResultText(fmt.Sprintf(
			`{"status":"processing","job_id":%q,"message":"Commit plan is taking longer than expected. Poll STATUS with this job_id to get the result."}`,
			jobID,
		)), nil
	}
}

// handleStatus returns the current state of a commit operation.
// Two paths:
// 1. With job_id → check BgJob status. If running, return processing.
//    If done, read plan from ConfirmStore. If failed, return error.
// 2. Without job_id → read plan status from ConfirmStore via PlanStatus().
func (h *Handler) handleStatus(params map[string]any) (*mcpgo.CallToolResult, error) {
	jobID := shared.GetStringParam(params, "job_id", "")

	// Path 1: Check background job by ID
	if jobID != "" {
		v, ok := h.bgJobs.Load(jobID)
		if ok {
			bgJob := v.(*BgJob)
			switch bgJob.Status {
			case BgRunning:
				return mcpgo.NewToolResultText(fmt.Sprintf(
					`{"status":"processing","job_id":%q,"message":"Plan still generating. Poll again."}`,
					jobID,
				)), nil
			case BgFailed:
				h.bgJobs.Delete(jobID)
				return shared.JSONErrorResult("STATUS", fmt.Errorf("plan generation failed: %s", bgJob.Error))
			case BgDone:
				h.bgJobs.Delete(jobID)
				// Fall through to plan status check below
			}
		}
		// Job not found in bgJobs — might have finished and been cleaned up.
		// Fall through to plan status check.
	}

	// Path 2: Read plan from ConfirmStore
	status, err := h.reviewWorkflow.PlanStatus()
	if err != nil {
		return shared.JSONErrorResult("STATUS", err)
	}
	return mcpgo.NewToolResultText(status), nil
}

// handleApply executes the pending commit plan via workflow.Apply.
// No job_id needed — operates on the pending plan in ConfirmStore.
// If pushAfter is true (from params), it also pushes to remote after successful apply.
func (h *Handler) handleApply(ctx context.Context, params map[string]any) (*mcpgo.CallToolResult, error) {
	pushAfter := false
	if v, ok := params["push_after"].(bool); ok {
		pushAfter = v
	}

	result, err := h.reviewWorkflow.Apply(ctx)
	if err != nil {
		return shared.JSONErrorResult("APPLY", err)
	}

	output := result.Output
	if pushAfter {
		pushOut, pErr := h.git.Push()
		if pErr != nil {
			output = fmt.Sprintf("%s\n\n[WARNING] Push failed: %v", output, pErr)
		} else {
			output = fmt.Sprintf("%s\n\n[SUCCESS] Changes pushed to remote:\n%s", output, pushOut)
		}
	}

	return mcpgo.NewToolResultText(output), nil
}

// handleAbort discards the pending plan via workflow.Abort.
// No job_id needed — operates on the pending plan in ConfirmStore.
func (h *Handler) handleAbort() (*mcpgo.CallToolResult, error) {
	err := h.reviewWorkflow.Abort()
	if err != nil {
		// Abort returns error if nothing to abort — that's fine for the client
		return mcpgo.NewToolResultText(fmt.Sprintf(`{"status":"aborted","message":"%s"}`, err.Error())), nil
	}
	return mcpgo.NewToolResultText(`{"status":"aborted","message":"Operation cancelled and rolled back"}`), nil
}

// handleRegenerate re-runs plan generation with feedback.
// It reads the instruction from the pending plan, appends feedback,
// aborts the current plan, and re-runs handlePreview.
func (h *Handler) handleRegenerate(ctx context.Context, params map[string]any, instruction string) (*mcpgo.CallToolResult, error) {
	feedback := shared.GetStringParam(params, "feedback", "")

	// Read current plan to get original instruction if not provided
	if instruction == "" {
		instr, err := h.reviewWorkflow.ReadPendingInstruction()
		if err != nil || instr == "" {
			return shared.JSONErrorResult("REGENERATE", fmt.Errorf("no pending plan to regenerate. Call PREVIEW first"))
		}
		instruction = instr
	}

	// Re-run with feedback appended to instruction
	if feedback != "" {
		instruction = instruction + "\n\nFeedback: " + feedback
	}

	// Delete the current pending plan so workflow.Run can start fresh
	_ = h.reviewWorkflow.Abort()

	return h.handlePreview(ctx, params, instruction)
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