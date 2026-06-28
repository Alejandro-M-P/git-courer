// Package core implements the core domain MCP handlers for git operations:
// commit pipeline (preview/apply/status), diff, and status.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/blak0p/git-courer/internal/adapters/git"
	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
	"github.com/blak0p/git-courer/internal/delivery/mcp/shared"
	"github.com/blak0p/git-courer/internal/workflow"
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
	workDir         string                // current working directory for project config access
	contentProvider ports.ContentProvider // optional: enables annotated diff output
	configProvider  func() *domain.ProjectConfig // override project config loading for tests
	llmDisabled     bool                  // true if LLM is disabled

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

// IsLLMEnabled returns true if the LLM is enabled (default).
func (h *Handler) IsLLMEnabled() bool {
	return !h.llmDisabled
}

// SetLLMEnabled enables or disables LLM logic.
func (h *Handler) SetLLMEnabled(enabled bool) {
	h.llmDisabled = !enabled
}

// loadProjectConfig returns the project configuration, using configProvider for testing
// or loading from disk via domain.LoadProjectConfig in production.
func (h *Handler) loadProjectConfig() *domain.ProjectConfig {
	if h.configProvider != nil {
		return h.configProvider()
	}
	cfg, err := domain.LoadProjectConfig(h.workDir)
	if err != nil {
		return &domain.ProjectConfig{}
	}
	return cfg
}

// ─── HandleCommit (PREVIEW/APPLY/STATUS) ──────────────────────────────
//
// The commit pipeline uses the workflow.Workflow engine + ConfirmStore:
//   - PREVIEW:     Runs workflow.Run("commit", why, nil). Plan data is stored
//                  in ConfirmStore. Fast path (<45s) returns plan directly; slow path
//                  returns a job_id for polling via STATUS.
//   - STATUS:      Polls background job state. If done, reads plan from ConfirmStore.
//   - APPLY:       Executes the pending plan via workflow.Apply (no job_id needed).
//   - ABORT:       Discards the pending plan via workflow.Abort (no job_id needed).
//   - REGENERATE:  Reads why from pending plan, appends feedback, re-runs PREVIEW.

// commandParams defines the set of allowed parameters for each commit subcommand.
// Any parameter not listed here will be rejected with an "unknown parameter" error,
// preventing LLMs from injecting irrelevant params like target_paths into APPLY.
var commandParams = map[string][]string{
	"PREVIEW": {"command", "why", "target_paths"},
	"APPLY":   {"command", "job_id", "push_after", "type"},
	"STATUS":  {"command", "job_id"},
}

// validTypes is the whitelist of allowed conventional-commit type prefixes.
var validTypes = map[string]bool{
	"feat":     true,
	"fix":      true,
	"chore":    true,
	"docs":     true,
	"refactor": true,
	"test":     true,
	"perf":     true,
	"style":    true,
}

// conventionalCommitPrefixRegex matches the type prefix at the start of a commit message.
// It captures: type, optional breaking flag (!), optional scope (...), optional colon+space.
// Example: "feat!", "feat(parser)", "feat(parser): ", "feat: ", "feat"
var conventionalCommitPrefixRegex = regexp.MustCompile(`^([a-zA-Z]+)(!?)(\([^)]*\))?[:\s]*`)

// overrideCommitType replaces the type prefix in a commit message with newType.
// It preserves scope, breaking flag (!), and body. If no recognized prefix is found,
// it prepends "newType: ". A recognized prefix means the leading word is in validTypes.
func overrideCommitType(message, newType string) string {
	if message == "" {
		return newType + ": "
	}

	loc := conventionalCommitPrefixRegex.FindStringSubmatchIndex(message)
	if loc == nil {
		return newType + ": " + message
	}

	// Extract the type word from group 1
	typeWord := message[loc[2]:loc[3]]
	if !validTypes[typeWord] {
		// The matched word is not a recognized conventional-commit type
		return newType + ": " + message
	}

	// Group indices: 3=optional !, 5=optional scope (...)
	var breakingFlag, scope string
	if loc[4] >= 0 && loc[5] >= 0 {
		breakingFlag = message[loc[4]:loc[5]] // group 2 = !
	}
	if loc[6] >= 0 && loc[7] >= 0 {
		scope = message[loc[6]:loc[7]] // group 3 = scope
	}

	_, fullEnd := loc[0], loc[1]
	newPrefix := newType + breakingFlag + scope + ": "

	return newPrefix + message[fullEnd:]
}

func (h *Handler) HandleCommit(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	params, _ := req.Params.Arguments.(map[string]any)
	command := strings.ToUpper(shared.GetStringParam(params, "command", "PREVIEW"))

	// Validate that only known parameters are present for this subcommand.
	// This prevents LLMs from passing irrelevant params (e.g. target_paths) to APPLY.
	var allowed []string
	if command == "PREVIEW" {
		if h.IsLLMEnabled() {
			allowed = []string{"command", "why", "target_paths"}
		} else {
			allowed = []string{"command", "target_paths", "message"}
		}
	} else if commandParams[command] != nil {
		allowed = commandParams[command]
	}

	if len(allowed) > 0 {
		if result, err := shared.ValidateKnownParams(params, allowed); result != nil || err != nil {
			return result, err
		}
	}

	why := shared.GetStringParam(params, "why", "")

	switch command {
	case "PREVIEW":
		return h.handlePreview(ctx, params, why)
	case "STATUS":
		return h.handleStatus(params)
	case "APPLY":
		return h.handleApply(ctx, params)
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
	if !h.IsLLMEnabled() {
		msg := shared.GetStringParam(params, "message", "")
		if msg == "" {
			return shared.JSONErrorResult("PREVIEW", fmt.Errorf("message parameter is required"))
		}

		targetPaths := shared.GetStringParam(params, "target_paths", "")
		if targetPaths != "" {
			if err := h.git.Add(git.SplitPaths(targetPaths)); err != nil {
				return shared.JSONErrorResult("PREVIEW", err)
			}
		}

		// Perform conventional commit type prefix check and inference when LLM is disabled
		hasValidPrefix := false
		loc := conventionalCommitPrefixRegex.FindStringSubmatchIndex(msg)
		if loc != nil {
			typeWord := msg[loc[2]:loc[3]]
			if validTypes[typeWord] {
				hasValidPrefix = true
			}
		}

		if !hasValidPrefix {
			var stagedFiles []string
			status, err := h.git.Status()
			if err == nil {
				for _, f := range status.Files {
					if f.Staged {
						stagedFiles = append(stagedFiles, f.Path)
					}
				}
			}
			stagedDiff, _ := h.git.DiffStaged("")
			chunk := domain.DiffChunk{
				Files: stagedFiles,
				Diff:  stagedDiff,
			}
			inferredType := domain.InferCommitType(chunk)
			if inferredType == "" {
				inferredType = "chore"
			}
			msg = inferredType + ": " + msg
		}

		treeHash, err := h.git.WriteTree()
		if err != nil {
			return shared.JSONErrorResult("PREVIEW", err)
		}

		jobID := fmt.Sprintf("commit-%d", time.Now().UnixMilli())
		bgJob := &BgJob{
			ID:       jobID,
			Status:   BgDone,
			TreeHash: treeHash,
			Message:  msg,
			Why:      why,
			Done:     make(chan struct{}),
		}
		close(bgJob.Done)
		h.bgJobs.Store(jobID, bgJob)

		resp := struct {
			Status  string `json:"status"`
			JobID   string `json:"job_id"`
			Message string `json:"message"`
		}{
			Status:  "success",
			JobID:   jobID,
			Message: fmt.Sprintf("Commit plan created with message: %s", msg),
		}
		data, err := json.Marshal(resp)
		if err != nil {
			return shared.JSONErrorResult("PREVIEW", err)
		}
		return mcpgo.NewToolResultText(string(data)), nil
	}

	if h.commitSvc == nil {
		return shared.JSONErrorResult("PREVIEW", fmt.Errorf("commit service not available"))
	}

	// 1. Validate and stage user-selected paths before touching metadata.
	targetPaths := shared.GetStringParam(params, "target_paths", "")
	if targetPaths != "" {
		if err := h.git.Add(git.SplitPaths(targetPaths)); err != nil {
			return shared.JSONErrorResult("PREVIEW", err)
		}
	}

	// 2. Resolve branch and reconcile commit store
	if store := h.commitSvc.CommitStore(); store != nil {
		currentBranch, err := h.git.CurrentBranch()
		if err == nil && currentBranch != "" {
			if err := store.SetBranch(currentBranch); err != nil {
				log.Printf("[WARN] Failed to set branch store for %q: %v", currentBranch, err)
			}
		}

		// Determine the log range: merge-base..HEAD gives only commits unique to this branch.
		// Use BaseBranch from ProjectConfig if set, otherwise try common trunk names.
		var logOutput string
		logErr := error(nil)
		skipReconcile := false
		if currentBranch != "" {
			projectCfg := h.loadProjectConfig()
			baseBranch := projectCfg.BaseBranch
			resolved := false

			if baseBranch == currentBranch {
				// On trunk branch — skip reconciliation entirely.
				// LogRange(HEAD, HEAD) would produce empty output, so we skip.
				skipReconcile = true
			} else if baseBranch != "" {
				// BaseBranch configured — single merge-base call.
				mergeBase, mbErr := h.git.MergeBase(baseBranch, currentBranch)
				if mbErr != nil {
					// Bug 1: When BaseBranch is configured and MergeBase fails, return error
					// Do NOT fall back to full log — this pollutes CommitStore with cross-branch history
					log.Printf("[WARN] MergeBase(%q, %q) failed: %v — returning error instead of falling back", baseBranch, currentBranch, mbErr)
					return shared.JSONErrorResult("PREVIEW", fmt.Errorf("cannot resolve merge base for configured BaseBranch %q: %w", baseBranch, mbErr))
				}
				if mergeBase != "" {
					logOutput, logErr = h.git.LogRange(mergeBase, "HEAD")
					resolved = true
				}
				// If MergeBase succeeds but returns empty, fall through to hardcoded list
			}

			if !skipReconcile && !resolved && baseBranch == "" {
				// No BaseBranch configured — try common trunk branch names in order.
				for _, base := range []string{"main", "master", "develop"} {
					if base == currentBranch {
						continue
					}
					mergeBase, mbErr := h.git.MergeBase(base, currentBranch)
					if mbErr == nil && mergeBase != "" {
						logOutput, logErr = h.git.LogRange(mergeBase, "HEAD")
						resolved = true
						break
					}
				}
			}
			if !skipReconcile && !resolved {
				// No common ancestor found — take all reachable commits.
				logOutput, logErr = h.git.Log(0, "")
			}
		} else {
			logOutput, logErr = h.git.Log(0, "")
		}

		if logErr != nil {
			log.Printf("[WARN] Failed to get git log for reconcile: %v", logErr)
		} else if !skipReconcile {
			var gitEntries []domain.CommitEntry
			if logOutput != "" {
				lines := strings.Split(logOutput, "\n")
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					parts := strings.SplitN(line, "|", 4)
					if len(parts) < 4 {
						continue
					}
				opts := []domain.CommitEntryOption{
					domain.WithAuthor(parts[1]),
					domain.WithDate(parts[2]),
				}
				entry, err := domain.NewCommitEntry(parts[0], parts[3], opts...)
					if err != nil {
						log.Printf("[WARN] Failed to parse commit entry from log: %v", err)
						continue
					}
					gitEntries = append(gitEntries, entry)
				}
			}
			if err := store.Reconcile(gitEntries); err != nil {
				log.Printf("[WARN] Failed to reconcile commit store: %v", err)
			}
		}
	}

	// 3. Stage metadata before WriteTree
	if err := h.git.Add([]string{domain.MetadataDir}); err != nil {
		// Log but do not block — if metadata dir doesn't exist, we don't fail
		log.Printf("[DEBUG] Failed to stage metadata directory: %v", err)
	}

	// Synchronous WriteTree: capture the current staging area snapshot atomically.
	// If this fails, no BgJob is created — we return immediately.
	treeHash, err := h.git.WriteTree()
	if err != nil {
		return shared.JSONErrorResult("PREVIEW", err)
	}

	// Conditionally unstage the metadata directory to keep the user's staging area clean
	if _, headErr := h.git.Head(); headErr == nil {
		if _, resetErr := h.git.Reset("HEAD", domain.MetadataDir); resetErr != nil {
			log.Printf("[WARN] Failed to unstage metadata directory: %v", resetErr)
		}
	} else {
		if _, resetErr := h.git.Reset("--", domain.MetadataDir); resetErr != nil {
			log.Printf("[WARN] Failed to unstage metadata directory: %v", resetErr)
		}
	}

	// Configure progress callback in workflow
	h.reviewWorkflow.SetProgressCallback(func(step, total int, message string) {
		shared.SendProgress(ctx, h.mcpServer, params, float64(step), float64(total), message)
	})

	type runResult struct {
		result workflow.Result
		err    error
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
//  1. With job_id → check BgJob status. If running, return processing.
//     If done, read plan from ConfirmStore. If failed, return error.
//  2. Without job_id → read plan status from ConfirmStore via PlanStatus().
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
func (h *Handler) handleApply(ctx context.Context, params map[string]any) (*mcpgo.CallToolResult, error) {
	jobID := shared.GetStringParam(params, "job_id", "")

	// Plumbing path: job_id present and non-empty
	if jobID != "" {
		pushAfter := false
		if v, ok := params["push_after"].(bool); ok {
			pushAfter = v
		}
		typeOverride := shared.GetStringParam(params, "type", "")
		return h.applyPlumbing(ctx, jobID, pushAfter, typeOverride)
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
func (h *Handler) applyPlumbing(ctx context.Context, jobID string, pushAfter bool, typeOverride string) (*mcpgo.CallToolResult, error) {
	// Look up BgJob by job_id first (so "job not found" surfaces before type validation)
	v, ok := h.bgJobs.Load(jobID)
	if !ok {
		return nil, fmt.Errorf("job not found — may have expired or been applied already")
	}
	job := v.(*BgJob)

	// Validate type override if provided (after job lookup, before any git mutation)
	if typeOverride != "" {
		if !validTypes[typeOverride] {
			return nil, fmt.Errorf("invalid type override %q: valid values are feat, fix, chore, docs, refactor, test, perf, style", typeOverride)
		}
	}

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

	// Apply type override if provided
	if typeOverride != "" {
		message = overrideCommitType(message, typeOverride)
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

	// Capture the commit metadata in the commit store for release logs/changelogs
	if h.commitSvc != nil {
		h.commitSvc.CaptureCommit(message)
	}

	// Plumbing amend — stage metadata dir, write new tree, create replacement commit, update HEAD
	// This ensures the pushed commit includes the latest metadata entry
	if err := h.git.Add([]string{domain.MetadataDir}); err != nil {
		log.Printf("[WARN] applyPlumbing: failed to stage metadata dir: %v", err)
	} else {
		// Write a new tree that includes the staged metadata
		newTreeHash, treeErr := h.git.WriteTree()
		if treeErr != nil {
			log.Printf("[WARN] applyPlumbing: WriteTree failed: %v", treeErr)
		} else {
			// Create a replacement commit with the same parent (original commit) but new tree
			replacementCommit, commitErr := h.git.CommitTree(newTreeHash, parentHash, message)
			if commitErr != nil {
				log.Printf("[WARN] applyPlumbing: CommitTree (amend) failed: %v", commitErr)
			} else {
				// Update HEAD to point to the replacement commit
				_, refErr := h.git.UpdateRef("HEAD", replacementCommit)
				if refErr != nil {
					log.Printf("[WARN] applyPlumbing: UpdateRef failed: %v", refErr)
				} else {
					// Success — update commitHash to the replacement for the response
					oldCommitHash := commitHash
					commitHash = replacementCommit
					log.Printf("[DEBUG] applyPlumbing: metadata amend successful — commit %s → %s", oldCommitHash, replacementCommit)
				}
			}
		}
	}

	// Release confirm lock after successful plumbing commit
	if h.reviewWorkflow != nil {
		h.reviewWorkflow.CleanupAfterPlumbing()
	}

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
