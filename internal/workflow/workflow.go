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
	"fmt"
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
	git     ports.Git
	llm     ports.LLM
	confirm ports.Confirm
	cfg     *config.Config
}

// New creates a new Workflow.
func New(git ports.Git, llm ports.LLM, confirm ports.Confirm, cfg *config.Config) *Workflow {
	return &Workflow{git: git, llm: llm, confirm: confirm, cfg: cfg}
}

// RequiresConfirm returns true if the operation needs user confirmation before executing.
func (w *Workflow) RequiresConfirm(op string) bool {
	return w.cfg.Preview.IsRequired(op)
}

// Run executes a workflow operation.
// If confirm is needed → saves plan + returns pending_approval.
// If no confirm → executes immediately and returns completed.
func (w *Workflow) Run(ctx context.Context, op, instruction string, explicitArgs map[string]string) (Result, error) {
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

	output, err := w.execute(ctx, plan.Operation, plan.Args)
	if err != nil {
		return Result{}, err
	}
	return Result{Status: StatusCompleted, Output: output}, nil
}

// Abort discards a pending operation (user cancelled via *_ABORT).
func (w *Workflow) Abort() error {
	defer w.confirm.ReleaseLock()
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
