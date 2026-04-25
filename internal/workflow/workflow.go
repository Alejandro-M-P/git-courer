// Package workflow implements the unified git workflow engine.
//
// All operations follow four phases:
//  1. PREPARE  — gather git context
//  2. GENERATE — LLM interprets instruction → concrete args
//  3. CONFIRM  — (optional) save plan + blocker; return pending
//  4. EXECUTE  — run the git command
//
// Operations that require confirmation use a three-step MCP protocol:
//   *_START  → Run()   → returns {status: "pending_approval", preview: "..."}
//   *_APPLY  → Apply() → executes the saved plan
//   *_ABORT  → Abort() → discards the plan
package workflow

import (
	"context"
	"crypto/sha256"
	"fmt"
 fix/commit-timing-mcp

	"path/filepath"
 main
	"strings"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// Status values returned in Result.
const (
	StatusCompleted = "completed"
	StatusPending   = "pending_approval"
	StatusAborted   = "aborted"
)

// Result is the return value of Run / Apply / Abort.
type Result struct {
	Status  string            // "completed" | "pending_approval" | "aborted"
	Output  string            // Human-readable output (for completed)
	Preview string            // What will be done (for pending)
	Args    map[string]string // Resolved args from LLM (for pending)
	Summary *Summary          // Structured metadata for the user
}

// Summary provides a high-level overview of the operation's impact.
type Summary struct {
	Operation     string   `json:"operation"`
	FilesAffected []string `json:"files_affected,omitempty"`
	Impact        string   `json:"impact"` // "Low" | "Medium" | "High"
	SecurityCheck string   `json:"security_check"`
	Message       string   `json:"message"`
}

// Workflow is the main workflow engine for git review operations.
type Workflow struct {
 fix/commit-timing-mcp
	git       ports.Git
	llm       ports.LLM
	confirm   ports.Confirm
	commitSvc *CommitService
	cfg       *config.Config
}

// New creates a new Workflow with optional commit service (for commit operations).
func New(git ports.Git, llm ports.LLM, confirm ports.Confirm, commitSvc *CommitService, cfg *config.Config) *Workflow {
	return &Workflow{git: git, llm: llm, confirm: confirm, commitSvc: commitSvc, cfg: cfg}

	git      ports.Git
	llm      ports.LLM
	confirm  ports.Confirm
	cfg      *config.Config
	commit   *CommitService
	release  *ReleaseService
	security ports.SecurityService
}

// New creates a new Workflow with all its specialized services.
func New(git ports.Git, llm ports.LLM, confirm ports.Confirm, cfg *config.Config, commit *CommitService, release *ReleaseService, security ports.SecurityService) *Workflow {
	return &Workflow{
		git:      git,
		llm:      llm,
		confirm:  confirm,
		cfg:      cfg,
		commit:   commit,
		release:  release,
		security: security,
	}
main
}

// RequiresConfirm returns true if the operation needs user confirmation before executing.
func (w *Workflow) RequiresConfirm(op string) bool {
	return w.cfg.Preview.IsRequired(op)
}

// computeDiffHash calculates SHA256 hash of current diff (unstaged + staged).
func (w *Workflow) computeDiffHash() (string, error) {
	diff, err := w.git.Diff()
	if err != nil {
		return "", fmt.Errorf("failed to get diff for hash: %w", err)
	}
	diffStaged, err := w.git.DiffStaged()
	if err != nil {
		return "", fmt.Errorf("failed to get staged diff for hash: %w", err)
	}
	combined := diff + "\n--- staged ---\n" + diffStaged
	hash := sha256.Sum256([]byte(combined))
	return fmt.Sprintf("%x", hash), nil
}

// Run executes a workflow operation.
// If confirm is needed → saves plan + returns pending_approval.
// If no confirm → executes immediately and returns completed.
func (w *Workflow) Run(ctx context.Context, op, instruction string, explicitArgs map[string]string) (Result, error) {
fix/commit-timing-mcp
	// Special handling for commit operation when commit service is available
	if op == "commit" && w.commitSvc != nil {
		if w.RequiresConfirm(op) {
			// Prepare commit via commit service
			messages, chunks, deletedFiles, warnings, reasoning, err := w.commitSvc.PrepareCommit(instruction)
			if err != nil {
				return Result{}, fmt.Errorf("failed to prepare commit: %w", err)
			}
			
			chunkFiles := DiffChunksToChunkFiles(chunks)
			
			plan := domain.OperationPlan{
				Operation:      op,
				Args:          explicitArgs, // commit doesn't use args like branch/tag
				Preview:       strings.Join(messages, "\n"),
				CreatedAt:     time.Now().Unix(),
				Messages:      messages,
				Chunks:        chunkFiles,
				DeletedFiles:  deletedFiles,
				Reasoning:     reasoning,
				Instruction:   instruction,
			}
			
			// Store rejected message if there are warnings
			if len(warnings) > 0 {
				plan.RejectedMessage = warnings[0]
			}
			
			if err := w.confirm.AcquireLock(); err != nil {
				return Result{}, fmt.Errorf("could not acquire lock: %w", err)
			}
			if err := w.confirm.WritePlan(plan); err != nil {
				w.confirm.ReleaseLock()
				return Result{}, fmt.Errorf("could not save plan: %w", err)
			}
			if err := w.confirm.CreateBlocker(); err != nil {
				w.confirm.ReleaseLock()
				return Result{}, fmt.Errorf("could not create blocker: %w", err)
			}
			return Result{Status: StatusPending, Preview: strings.Join(messages, "\n"), Args: explicitArgs}, nil
		} else {
			// No confirmation required - execute directly
			result, err := w.commitSvc.Execute(instruction, false)
			if err != nil {
				return Result{}, err
			}
			return Result{Status: StatusCompleted, Output: result}, nil
		}
	}

	// 1. PREPARE

	// 1. PREPARE CONTEXT (Gather branches, tags, status)
 main
	prep, err := w.prepare(ctx, op)
	if err != nil {
		return Result{}, fmt.Errorf("prepare failed: %w", err)
	}

	// 2. GENERATE INTENT (Ask LLM what to do)
	args, preview, err := w.generate(ctx, op, instruction, prep, explicitArgs)
	if err != nil {
		return Result{}, fmt.Errorf("generate failed: %w", err)
	}

	// 3. SPECIALIZED PRE-EXECUTION (e.g., Staging for commits)
	// This is critical: we need actual staged files to run the security scan.
	if op == "commit" {
		// We delegate the complex staging logic to the CommitService's inner logic
		// but since we are in a unified flow, we ensure security runs HERE.
		// For E2E tests and real usage, we want to see the diff NOW.
		status, _ := w.git.Status()

		// Determine which files to stage based on LLM args
		includeUntracked := args["include_untracked"] == "true"
		filter := args["file_filter"]

		var filesToStage []string
		var absFilesToStage []string
		for _, f := range status.Files {
			// Basic filtering logic matching the intent
			if filter != "" && !strings.Contains(f.Path, filter) {
				continue
			}
			if f.Status == "??" && !includeUntracked {
				continue
			}
			filesToStage = append(filesToStage, f.Path)
			absFilesToStage = append(absFilesToStage, filepath.Join(w.cfg.Git.WorkDir, f.Path))
		}

		if len(filesToStage) > 0 {
			w.git.Add(filesToStage)
			diff, _ := w.git.DiffStaged(filesToStage...)

			// MANDATORY SECURITY INTERCEPTOR
			secResult := w.security.CheckFiles(absFilesToStage, diff)
			if secResult.IsBlocked() {
				w.git.Reset("HEAD", ".") // Cleanup staging on threat
				if first := secResult.FirstBlocking(); first != nil {
					return Result{
						Status:  "blocked",
						Preview: fmt.Sprintf("⚠️ SECURITY ALERT: Secret detected in %s (%s). Operation aborted for your protection.", first.File, first.Type),
						Summary: &Summary{
							Operation:     op,
							FilesAffected: filesToStage,
							Impact:        "High",
							SecurityCheck: "BLOCKED",
							Message:       fmt.Sprintf("Secret detected in %s", first.File),
						},
					}, nil
				}
			}
		}
	}

	// 4. CONFIRM (optional)
	if w.RequiresConfirm(op) {
		diff, _ := w.git.DiffStaged()
		if diff == "" {
			diff, _ = w.git.Diff()
		}

		// CREATE BACKUP BEFORE PROCEEDING
		backup, _ := w.git.CreateBackup(op, true)

		// Calculate affected files for summary
		status, _ := w.git.Status()
		var files []string
		for _, f := range status.Files {
			if f.Status != "  " && f.Status != "??" {
				files = append(files, f.Path)
			}
		}

		plan := domain.OperationPlan{
			Operation: op,
			Args:      args,
			Preview:   preview,
			CreatedAt: time.Now().Unix(),
			DiffHash:  calculateHash(diff),
			Backup:    backup,
		}
		if err := w.confirm.AcquireLock(); err != nil {
			w.git.DeleteBackup(backup) // Cleanup on lock fail
			return Result{}, fmt.Errorf("could not acquire lock: %w", err)
		}
		if err := w.confirm.WritePlan(plan); err != nil {
			w.confirm.ReleaseLock()
			w.git.DeleteBackup(backup)
			return Result{}, fmt.Errorf("could not save plan: %w", err)
		}
		if err := w.confirm.CreateBlocker(); err != nil {
			w.confirm.ReleaseLock()
			w.git.DeleteBackup(backup)
			return Result{}, fmt.Errorf("could not create blocker: %w", err)
		}
		return Result{
			Status:  StatusPending,
			Preview: preview,
			Args:    args,
			Summary: &Summary{
				Operation:     op,
				FilesAffected: files,
				Impact:        calculateImpact(op, len(files)),
				SecurityCheck: "Passed",
				Message:       "Operation planned and awaiting approval",
			},
		}, nil
	}

	// 4. EXECUTE (Direct)
	backup, _ := w.git.CreateBackup(op, true)
	output, err := w.execute(ctx, op, args)
	if err != nil {
		w.git.RestoreBackup(backup)
		return Result{}, err
	}
	w.git.DeleteBackup(backup)
	
	// Final Summary for direct execution
	return Result{
		Status: StatusCompleted, 
		Output: output, 
		Preview: preview,
		Summary: &Summary{
			Operation: op,
			Impact:    calculateImpact(op, 0),
			SecurityCheck: "Passed",
			Message: "Operation executed immediately (No confirmation required)",
		},
	}, nil
}

// executeCommitFromPlan executes a commit operation using the commit service.
func (w *Workflow) executeCommitFromPlan(plan *domain.OperationPlan) (string, error) {
	if w.commitSvc == nil {
		return "", fmt.Errorf("commit service not available")
	}
	return w.commitSvc.ExecuteFromPlan(plan.Messages, plan.Chunks, plan.DeletedFiles, plan.Instruction)
}

// Apply executes a previously planned operation (user approved via *_APPLY).
func (w *Workflow) Apply(ctx context.Context) (Result, error) {
	defer w.confirm.ReleaseLock()

	if !w.confirm.HasBlocker() {
		return Result{}, fmt.Errorf("no pending operation to apply. Run *_START first")
	}
	if w.confirm.IsPlanExpired() {
		w.confirm.DeletePlan()
		return Result{}, fmt.Errorf("plan expired. Run *_START again")
	}

	plan, err := w.confirm.ReadPlan()
	if err != nil || plan == nil {
		w.confirm.RemoveBlocker()
		return Result{}, fmt.Errorf("failed to read plan. Run *_START again")
	}

	// INTEGRITY CHECK: Compare Hash
	diff, _ := w.git.DiffStaged()
	if diff == "" {
		diff, _ = w.git.Diff()
	}
	currentHash := calculateHash(diff)
	if plan.DiffHash != "" && plan.DiffHash != currentHash {
		w.git.RestoreBackup(plan.Backup)
		w.confirm.DeletePlan()
		return Result{}, fmt.Errorf("⚖️ INTEGRITY ALERT: Your code has changed since the preview was generated. Rollback performed for your safety. Please run START again")
	}

	defer w.confirm.DeletePlan()

	// Special handling for commit operation
	if plan.Operation == "commit" && w.commitSvc != nil {
		output, err := w.executeCommitFromPlan(plan)
		if err != nil {
			return Result{}, err
		}
		return Result{Status: StatusCompleted, Output: output}, nil
	}

	output, err := w.execute(ctx, plan.Operation, plan.Args)
	if err != nil {
		w.git.RestoreBackup(plan.Backup)
		return Result{}, fmt.Errorf("execution failed: %w (Rollback performed)", err)
	}

	w.git.DeleteBackup(plan.Backup)
	return Result{
		Status:  StatusCompleted,
		Output:  output,
		Preview: plan.Preview,
		Summary: &Summary{
			Operation:     plan.Operation,
			FilesAffected: plan.Files,
			Impact:        calculateImpact(plan.Operation, len(plan.Files)),
			SecurityCheck: "Verified",
			Message:       "Operation executed successfully after user approval",
		},
	}, nil
}

func calculateImpact(op string, fileCount int) string {
	if op == "release" || op == "merge" || op == "branch_delete" {
		return "High"
	}
	if fileCount > 10 {
		return "Medium"
	}
	return "Low"
}

// Abort discards a pending operation (user cancelled via *_ABORT).
func (w *Workflow) Abort() error {
	defer w.confirm.ReleaseLock()
 fix/commit-timing-mcp
	
	// For commit operations, also reset HEAD and clean staging area
	if w.confirm.HasBlocker() {
		plan, err := w.confirm.ReadPlan()
		if err == nil && plan != nil && plan.Operation == "commit" {
			// Reset HEAD to clear any staged changes from PrepareCommit
			w.git.Reset("HEAD", ".")
		}
	}
	


	plan, err := w.confirm.ReadPlan()
	if err == nil && plan != nil {
		// Rollback to original state on abort
		w.git.RestoreBackup(plan.Backup)
		w.git.DeleteBackup(plan.Backup)
	}

main
	return w.confirm.DeletePlan()
}

func calculateHash(content string) string {
	if content == "" {
		return ""
	}
	h := sha256.New()
	h.Write([]byte(content))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// HasPendingPlan returns true if there is a pending operation waiting for approval.
func (w *Workflow) HasPendingPlan() bool {
	return w.confirm.HasBlocker()
}

// PlanStatus returns a human-readable summary of the current pending plan (if any).
func (w *Workflow) PlanStatus() (string, error) {
	if !w.confirm.HasBlocker() {
		return "No pending review operation.", nil
	}
	plan, err := w.confirm.ReadPlan()
	if err != nil {
		return "", err
	}
	if plan == nil {
		return "No pending review operation.", nil
	}
	return fmt.Sprintf("Pending: %s\nPreview: %s\nCreated: %s",
		plan.Operation,
		plan.Preview,
		fmt.Sprintf("%d", plan.CreatedAt),
	), nil
}
