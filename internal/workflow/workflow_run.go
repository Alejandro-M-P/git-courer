package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/blak0p/git-courer/internal/core/domain"
)

// Run executes a workflow operation.
// If confirm is needed → saves plan + returns pending_approval.
// If no confirm → executes immediately and returns completed.
func (w *Workflow) Run(ctx context.Context, op, instruction string, explicitArgs map[string]string) (Result, error) {
	// Propagate progress callback to commitSvc if available
	if w.commitSvc != nil && w.progress != nil {
		w.commitSvc.SetProgressCallback(w.progress)
	}

	// Special handling for commit operation when commit service is available
	if op == "commit" && w.commitSvc != nil {
		if w.RequiresConfirm(op, explicitArgs) {
			// Prepare commit via commit service
			messages, chunks, deletedFiles, warnings, reasoning, err := w.commitSvc.PrepareCommit(instruction)
			if err != nil {
				return Result{}, fmt.Errorf("failed to prepare commit: %w", err)
			}

			// Calculate files for commit
			var files []string
			for _, chunk := range chunks {
				files = append(files, chunk.Files...)
			}
			files = append(files, deletedFiles...)

			chunkFiles := DiffChunksToChunkFiles(chunks)

			plan := domain.OperationPlan{
				Operation:    op,
				Args:         explicitArgs, // commit doesn't use args like branch/tag
				Preview:      strings.Join(messages, "\n"),
				CreatedAt:    time.Now().Unix(),
				Messages:     messages,
				Files:        files,
				Chunks:       chunkFiles,
				DeletedFiles: deletedFiles,
				Reasoning:    reasoning,
				Instruction:  instruction,
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
			return Result{
				Status:  StatusPending,
				Preview: strings.Join(messages, "\n"),
				Args:    explicitArgs,
				Summary: &Summary{
					Operation:     op,
					FilesAffected: files,
					Impact:        calculateImpact(op, len(files)),
					SecurityCheck: "Passed",
					Message:       "Commit planned and awaiting approval",
					Reasoning:     reasoning,
					Messages:      messages,
				},
			}, nil
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
			absFilesToStage = append(absFilesToStage, filepath.Join(".", f.Path))
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
	if w.RequiresConfirm(op, explicitArgs) {
		diff, _ := w.git.DiffStaged()
		if diff == "" {
			diff, _ = w.git.Diff()
		}

		// CREATE BACKUP BEFORE PROCEEDING (don't stash untracked files during preview)
		backup, _ := w.git.CreateBackup(op, domain.StashNone)

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
			Files:     files,
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
	backup, _ := w.git.CreateBackup(op, domain.StashAll)
	output, err := w.execute(ctx, op, args)
	if err != nil {
		w.git.RestoreBackup(backup)
		return Result{}, err
	}
	w.git.DeleteBackup(backup)

	// Final Summary for direct execution
	return Result{
		Status:  StatusCompleted,
		Output:  output,
		Preview: preview,
		Summary: &Summary{
			Operation:     op,
			Impact:        calculateImpact(op, 0),
			SecurityCheck: "Passed",
			Message:       "Operation executed immediately (No confirmation required)",
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
		return Result{
			Status: StatusCompleted,
			Output: output,
			Summary: &Summary{
				Operation:     plan.Operation,
				FilesAffected: plan.Files,
				Impact:        calculateImpact(plan.Operation, len(plan.Files)),
				SecurityCheck: "Verified",
				Message:       "Commit executed successfully",
				Reasoning:     plan.Reasoning,
				Messages:      plan.Messages,
			},
		}, nil
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
			Reasoning:     plan.Reasoning,
			Messages:      plan.Messages,
		},
	}, nil
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

	plan, err := w.confirm.ReadPlan()
	if err == nil && plan != nil {
		// Rollback to original state on abort
		w.git.RestoreBackup(plan.Backup)
		w.git.DeleteBackup(plan.Backup)
	}

	return w.confirm.DeletePlan()
}
