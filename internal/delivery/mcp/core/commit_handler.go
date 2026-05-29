// Package core implements the core domain MCP handlers for git operations:
// commit pipeline (preview/apply/abort/regenerate/status), amend, and revert.
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
	Why      string        // custom explanation or justification
	Done     chan struct{} // make(chan struct{}), closed when goroutine finishes
}

// Handler holds dependencies for core domain MCP handlers (commit, amend, revert, diff, status).
type Handler struct {
	git             ports.Git
	commitSvc       *workflow.CommitService
	reviewWorkflow  *workflow.Workflow
	llm             ports.LLM
	provider        string // "ollama" for local LLM, anything else for cloud
	mcpServer       *server.MCPServer
	workDir         string // current working directory for project config access
	contentProvider ports.ContentProvider // optional: enables annotated diff output

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
	contentProvider ports.ContentProvider,
) *Handler {
	return &Handler{
		git:             git,
		commitSvc:       commitSvc,
		reviewWorkflow:  reviewWorkflow,
		llm:             llm,
		provider:        provider,
		mcpServer:       mcpServer,
		workDir:         ".",
		contentProvider: contentProvider,
	}
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
//   - PREVIEW:     Runs workflow.Run("commit", why, nil). Plan data is stored
//                  in ConfirmStore. Fast path (<45s) returns plan directly; slow path
//                  returns a job_id for polling via STATUS.
//   - STATUS:      Polls background job state. If done, reads plan from ConfirmStore.
//   - APPLY:       Executes the pending plan via workflow.Apply (no job_id needed).
//   - ABORT:       Discards the pending plan via workflow.Abort (no job_id needed).
//   - REGENERATE:  Reads why from pending plan, appends feedback, re-runs PREVIEW.

func (h *Handler) HandleCommit(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(shared.GetStringParam(params, "command", "PREVIEW"))
	why := shared.GetStringParam(params, "why", "")

	switch command {
	case "PREVIEW":
		return h.handlePreview(ctx, params, why)
	case "STATUS":
		return h.handleStatus(params)
	case "APPLY":
		return h.handleApply(ctx, params)
	case "ABORT":
		return h.handleAbort()
	case "REGENERATE":
		return h.handleRegenerate(ctx, params, why)
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
func (h *Handler) handlePreview(ctx context.Context, params map[string]any, why string) (*mcpgo.CallToolResult, error) {
	if h.commitSvc == nil {
		return shared.JSONErrorResult("PREVIEW", fmt.Errorf("commit service not available"))
	}

	// Check for new directories that need area assignments.
	// Only ask once — if area_response was provided, skip this check and save it.
	areaResponse := getStringAreaResponse(params)
	if areaResponse != nil {
		if err := h.saveAreaResponse(areaResponse); err != nil {
			log.Printf("[commit] area_response save failed on PREVIEW: %v", err)
		}
	} else {
		newDirs, projectCfg, err := h.checkNewDirectories()
		if err != nil {
			// Log but don't block — area question is optional
			log.Printf("[commit] area check failed: %v", err)
		} else if newDirs != nil && len(newDirs) > 0 {
			return mcpgo.NewToolResultText(areaRequiredResponse(newDirs, projectCfg.Areas)), nil
		}
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
		result, err := h.reviewWorkflow.Run(ctx, "commit", why, nil)
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

		var commitMsg string
		if res.result.Summary != nil && len(res.result.Summary.Messages) > 0 {
			commitMsg = composeMessage(res.result.Summary.Messages, "")
		} else {
			commitMsg = res.result.Output
		}

		// Store BgJob with TreeHash and Done for consistency with slow path.
		// Fast-path job is immediately complete: Status=BgDone, Message set, Done closed.
		jobID := fmt.Sprintf("commit-%d", time.Now().UnixMilli())
		bgJob := &BgJob{
			ID:       jobID,
			Status:   BgDone,
			TreeHash: treeHash,
			Message:  commitMsg,
			Why:      why,
			Done:     make(chan struct{}),
		}
		close(bgJob.Done) // fast path: done immediately
		h.bgJobs.Store(jobID, bgJob)

		// Read the plan from ConfirmStore to format the response
		plan, _ := h.reviewWorkflow.PlanStatus()
		var responseMsg string
		if plan != "" {
			responseMsg = plan
		} else {
			responseMsg = res.result.Output
		}

		resp := struct {
			Status  string `json:"status"`
			JobID   string `json:"job_id"`
			Message string `json:"message"`
		}{
			Status:  "success",
			JobID:   jobID,
			Message: responseMsg,
		}
		data, err := json.Marshal(resp)
		if err != nil {
			return shared.JSONErrorResult("PREVIEW", err)
		}
		return mcpgo.NewToolResultText(string(data)), nil

	case <-time.After(previewTimeout):
		// Slow path: plan generation is taking too long — fall back to async bg job.
		// Send progress: still processing
		shared.SendProgress(ctx, h.mcpServer, params, 3, shared.ProgressTotal, shared.CommitProgressMessage(shared.ProgressClassify))

		jobID := fmt.Sprintf("commit-%d", time.Now().UnixMilli())
		bgJob := &BgJob{
			ID:       jobID,
			Status:   BgRunning,
			TreeHash: treeHash,
			Why:      why,
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
			if res.result.Summary != nil && len(res.result.Summary.Messages) > 0 {
				j.Message = composeMessage(res.result.Summary.Messages, "")
			} else {
				j.Message = res.result.Output
			}
			close(j.Done)
			log.Printf("[commit] job %s done (async)", jobID)
		}()

		return mcpgo.NewToolResultText(fmt.Sprintf(
			`{"status":"processing","job_id":%q,"message":"Commit plan is taking longer than expected. Do NOT block — keep working on other tasks. Use commit-jobs to list all background jobs and their status. Poll STATUS with this job_id to get the result when ready."}`,
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
				return shared.JSONErrorResult("STATUS", fmt.Errorf("plan generation failed: %s", bgJob.Error))
			case BgDone:
				// Job persists until APPLY or ABORT removes it — do NOT delete here
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

// handleApply executes a commit. Two paths:
//  1. With job_id → plumbing path: creates a single atomic commit from the
//     PREVIEW tree snapshot via CommitTree + UpdateRef, bypassing porcelain.
//  2. Without job_id → legacy path: delegates to reviewWorkflow.Apply.
//
// If pushAfter is true, pushes to remote after successful apply.
// If area_response is provided, saves the area mappings to project config
// before proceeding with the commit (append-only — never overwrites existing mappings).
func (h *Handler) handleApply(ctx context.Context, params map[string]any) (*mcpgo.CallToolResult, error) {
	// Handle area_response if provided — save to project config before commit
	areaResponse := getStringAreaResponse(params)
	if areaResponse != nil {
		if err := h.saveAreaResponse(areaResponse); err != nil {
			log.Printf("[commit] area_response save failed: %v", err)
			// Log error but don't block the commit — area mapping is advisory
		}
	}

	jobID := shared.GetStringParam(params, "job_id", "")

	// Plumbing path: job_id present and non-empty
	if jobID != "" {
		pushAfter := false
		if v, ok := params["push_after"].(bool); ok {
			pushAfter = v
		}
		return h.applyPlumbing(ctx, jobID, pushAfter)
	}

	// Legacy path: no job_id — operate on pending plan in ConfirmStore
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

// applyPlumbing creates a single atomic commit from the PREVIEW tree snapshot
// using git plumbing commands (CommitTree + UpdateRef), bypassing the porcelain
// stage+commit cycle. This is invoked when handleApply receives a job_id.
func (h *Handler) applyPlumbing(ctx context.Context, jobID string, pushAfter bool) (*mcpgo.CallToolResult, error) {
	// Look up BgJob by job_id
	v, ok := h.bgJobs.Load(jobID)
	if !ok {
		return nil, fmt.Errorf("job not found — may have expired or been applied already")
	}
	job := v.(*BgJob)

	// Wait for job to complete if still running
	select {
	case <-job.Done:
		// Job completed — proceed
	case <-ctx.Done():
		return nil, fmt.Errorf("context cancelled while waiting for job %s", jobID)
	}

	// Re-check status after Done closes
	if job.Status == BgFailed {
		return nil, fmt.Errorf("plan generation failed: %s", job.Error)
	}

	// Compose commit message
	var message string
	if job.Message != "" {
		message = job.Message
	} else {
		chunks, err := h.commitSvc.GenerateCommitMessage(ctx, job.Why)
		if err != nil {
			message = "chore: apply changes"
		} else {
			message = composeMessage(chunks, "chore: apply changes")
		}
	}

	// Get parent commit hash
	parentHash, err := h.git.Head()
	if err != nil {
		return nil, fmt.Errorf("APPLY: failed to get HEAD: %w", err)
	}

	// Create commit from tree snapshot
	commitHash, err := h.git.CommitTree(job.TreeHash, parentHash, message)
	if err != nil {
		return nil, fmt.Errorf("APPLY: commit-tree failed: %w", err)
	}

	// Move HEAD to the new commit
	if _, err := h.git.UpdateRef("HEAD", commitHash); err != nil {
		return nil, fmt.Errorf("APPLY: update-ref failed (commit %s may need manual recovery): %w", commitHash, err)
	}

	// Release confirm lock after successful plumbing commit
	h.reviewWorkflow.CleanupAfterPlumbing()

	// Clean staging area to match new HEAD
	if _, resetErr := h.git.Reset("HEAD", "."); resetErr != nil {
		// Reset failure is not a hard error — the commit is valid
		log.Printf("[apply-plumbing] WARNING: staging cleanup failed after successful commit %s: %v", commitHash, resetErr)
	}

	// Delete completed job from bgJobs
	h.bgJobs.Delete(jobID)

	// Push if requested
	output := fmt.Sprintf(`{"commit_hash":"%s","message":"%s","status":"applied"}`, commitHash, message)
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

func composeMessage(chunks []string, fallback string) string {
	if len(chunks) == 0 {
		return fallback
	}
	var joined []string
	for _, ch := range chunks {
		ch = strings.TrimSpace(ch)
		if ch != "" {
			joined = append(joined, ch)
		}
	}
	if len(joined) == 0 {
		return fallback
	}
	return strings.Join(joined, "\n\n")
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
// It reads why from the pending plan, appends feedback,
// aborts the current plan, and re-runs handlePreview.
func (h *Handler) handleRegenerate(ctx context.Context, params map[string]any, why string) (*mcpgo.CallToolResult, error) {
	feedback := shared.GetStringParam(params, "feedback", "")

	// Read current plan to get original why if not provided
	if why == "" {
		whyResult, err := h.reviewWorkflow.ReadPendingInstruction()
		if err != nil || whyResult == "" {
			return shared.JSONErrorResult("REGENERATE", fmt.Errorf("no pending plan to regenerate. Call PREVIEW first"))
		}
		why = whyResult
	}

	// Re-run with feedback appended to why
	if feedback != "" {
		why = why + "\n\nFeedback: " + feedback
	}

	// Delete the current pending plan so workflow.Run can start fresh
	_ = h.reviewWorkflow.Abort()

	return h.handlePreview(ctx, params, why)
}

// ─── HandleCommitJobs ────────────────────────────────────────────────

// commitJobEntry is the JSON structure returned by HandleCommitJobs.
type commitJobEntry struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	TreeHash string `json:"tree_hash"`
}

// HandleCommitJobs lists all entries in bgJobs as a JSON array.
// Read-only tool for inspecting active jobs — their status, message, and tree hash.
func (h *Handler) HandleCommitJobs(_ context.Context, _ mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	var jobs []commitJobEntry
	h.bgJobs.Range(func(key, value any) bool {
		j := value.(*BgJob)
		jobs = append(jobs, commitJobEntry{
			ID:       j.ID,
			Status:   string(j.Status),
			Message:  j.Message,
			TreeHash: j.TreeHash,
		})
		return true
	})

	if jobs == nil {
		jobs = []commitJobEntry{}
	}

	data, err := json.Marshal(jobs)
	if err != nil {
		return shared.JSONErrorResult("commit-jobs", err)
	}
	return mcpgo.NewToolResultText(string(data)), nil
}