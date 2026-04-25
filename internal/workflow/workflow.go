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
}

// Workflow is the main workflow engine for git review operations.
type Workflow struct {
	git       ports.Git
	llm       ports.LLM
	confirm   ports.Confirm
	commitSvc *CommitService
	cfg       *config.Config
}

// New creates a new Workflow with optional commit service (for commit operations).
func New(git ports.Git, llm ports.LLM, confirm ports.Confirm, commitSvc *CommitService, cfg *config.Config) *Workflow {
	return &Workflow{git: git, llm: llm, confirm: confirm, commitSvc: commitSvc, cfg: cfg}
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
	prep, err := w.prepare(ctx, op)
	if err != nil {
		return Result{}, fmt.Errorf("prepare failed: %w", err)
	}

	// 2. GENERATE
	args, preview, err := w.generate(ctx, op, instruction, prep, explicitArgs)
	if err != nil {
		return Result{}, fmt.Errorf("generate failed: %w", err)
	}

	// 3. CONFIRM (optional)
	if w.RequiresConfirm(op) {
		plan := domain.OperationPlan{
			Operation: op,
			Args:      args,
			Preview:   preview,
			CreatedAt: time.Now().Unix(),
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
		return Result{Status: StatusPending, Preview: preview, Args: args}, nil
	}

	// 4. EXECUTE
	output, err := w.execute(ctx, op, args)
	if err != nil {
		return Result{}, err
	}
	return Result{Status: StatusCompleted, Output: output}, nil
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
		return Result{}, err
	}
	return Result{Status: StatusCompleted, Output: output}, nil
}

// Abort discards a pending operation (user cancelled via *_ABORT).
func (w *Workflow) Abort() error {
	defer w.confirm.ReleaseLock()
	
	// For commit operations, also reset HEAD and clean staging area
	if w.confirm.HasBlocker() {
		plan, err := w.confirm.ReadPlan()
		if err == nil && plan != nil && plan.Operation == "commit" {
			// Reset HEAD to clear any staged changes from PrepareCommit
			w.git.Reset("HEAD", ".")
		}
	}
	
	return w.confirm.DeletePlan()
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
