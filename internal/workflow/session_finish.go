package workflow

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
)

// FinishStatus is the high-level outcome of a session finish operation.
type FinishStatus string

const (
	FinishSuccess       FinishStatus = "success"
	FinishConflict      FinishStatus = "conflict"
	FinishPreviewFailed FinishStatus = "preview_failed"
	FinishCleanupFailed FinishStatus = "cleanup_failed"
	FinishNotFound      FinishStatus = "not_found"
)

// FinishResult is the structured result of SessionFinishWorkflow.Finish.
type FinishResult struct {
	Status  FinishStatus    `json:"status"`
	Message string          `json:"message"`
	Preview *PreviewResult  `json:"preview,omitempty"`
	Session *domain.Session `json:"session,omitempty"`
}

// SessionFinishWorkflow orchestrates the finish lifecycle: load session,
// preview-validate, merge into base, cleanup worktree+branch, persist state.
type SessionFinishWorkflow struct {
	git     ports.Git
	store   ports.SessionStore
	preview *PreviewEngine
	workDir string
}

// NewSessionFinishWorkflow builds a finish workflow backed by git, store, and
// a PreviewEngine rooted at workDir.
func NewSessionFinishWorkflow(git ports.Git, store ports.SessionStore, workDir string) *SessionFinishWorkflow {
	return &SessionFinishWorkflow{
		git:     git,
		store:   store,
		preview: NewPreviewEngine(git, workDir),
		workDir: workDir,
	}
}

// Finish executes the session finish lifecycle for sessionID.
//
// Steps:
//  1. Load the session from the store. Missing session -> not_found.
//  2. Run the PreviewEngine against the session's base branch.
//  3. If preview detects uncommitted changes, test failure, or a merge
//     conflict, abort and keep the session active (preview_failed or conflict).
//  4. Resolve the main repo root from GitCommonDir, switch to the base branch
//     there, and merge the session branch.
//  5. On merge conflict, abort the merge and return conflict status.
//  6. On success, cleanup: RemoveWorktree + DeleteBranch.
//  7. If cleanup fails, transition the session to cleanup_failed (the merge is
//     NOT reverted) and persist it.
//  8. If cleanup succeeds, delete the session metadata file.
func (w *SessionFinishWorkflow) Finish(ctx context.Context, sessionID string) (*FinishResult, error) {
	sess, err := w.store.Get(sessionID)
	if err != nil {
		return &FinishResult{Status: FinishNotFound, Message: err.Error()}, nil
	}

	baseBranch := sess.BaseBranch
	if baseBranch == "" {
		baseBranch = "main"
	}

	// 2. Preview validation.
	preview, perr := w.preview.Preview(ctx, baseBranch)
	if perr != nil {
		return &FinishResult{
			Status:  FinishPreviewFailed,
			Message: fmt.Sprintf("preview error: %v", perr),
			Session: sess,
		}, nil
	}

	// 3. Abort on uncommitted changes, test failure, or conflict.
	if preview.HasUncommitted {
		return &FinishResult{
			Status:  FinishPreviewFailed,
			Message: "session has uncommitted or untracked changes; commit or stash before finishing",
			Preview: preview,
			Session: sess,
		}, nil
	}
	if preview.TestResult != nil && (preview.TestResult.Status == "fail" || preview.TestResult.Status == "timeout") {
		return &FinishResult{
			Status:  FinishPreviewFailed,
			Message: fmt.Sprintf("tests %s: %d failing", preview.TestResult.Status, preview.TestResult.Failed),
			Preview: preview,
			Session: sess,
		}, nil
	}
	if preview.HasConflict {
		return &FinishResult{
			Status:  FinishConflict,
			Message: "merge conflict detected during dry-run; resolve before finishing",
			Preview: preview,
			Session: sess,
		}, nil
	}

	// 4. Resolve the main repo root from GitCommonDir and switch to base branch.
	commonDir, cerr := w.git.GitCommonDir()
	if cerr != nil {
		return &FinishResult{
			Status:  FinishPreviewFailed,
			Message: fmt.Sprintf("resolve git common dir: %v", cerr),
			Preview: preview,
			Session: sess,
		}, nil
	}
	mainRepoRoot := filepath.Dir(commonDir)
	if serr := switchInDir(w.git, mainRepoRoot, baseBranch); serr != nil {
		return &FinishResult{
			Status:  FinishPreviewFailed,
			Message: fmt.Sprintf("switch to base branch %q in %s: %v", baseBranch, mainRepoRoot, serr),
			Preview: preview,
			Session: sess,
		}, nil
	}

	// 5. Merge the session branch into the base branch.
	if _, merr := w.git.Merge(sess.Branch); merr != nil {
		// Detect conflict via status; abort merge regardless.
		status, _ := w.git.Status()
		if status.Conflicted > 0 {
			_, _ = w.git.MergeAbort()
			return &FinishResult{
				Status:  FinishConflict,
				Message: fmt.Sprintf("merge conflict while merging %q into %q", sess.Branch, baseBranch),
				Preview: preview,
				Session: sess,
			}, nil
		}
		return &FinishResult{
			Status:  FinishPreviewFailed,
			Message: fmt.Sprintf("merge %q into %q: %v", sess.Branch, baseBranch, merr),
			Preview: preview,
			Session: sess,
		}, nil
	}

	// 6. Cleanup: remove worktree then delete the session branch.
	cleanupErr := w.cleanup(sess)
	if cleanupErr != nil {
		// 7. Tolerant cleanup: do NOT revert the merge. Mark the session as
		// cleanup_failed and persist so the caller can retry cleanup.
		sess.Status = domain.SessionCleanupFailed
		sess.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		_ = w.store.Save(sess)
		return &FinishResult{
			Status:  FinishCleanupFailed,
			Message: fmt.Sprintf("merge succeeded but cleanup failed: %v", cleanupErr),
			Preview: preview,
			Session: sess,
		}, nil
	}

	// 8. Cleanup succeeded: delete the session metadata file.
	_ = w.store.Delete(sess.ID)

	return &FinishResult{
		Status:  FinishSuccess,
		Message: fmt.Sprintf("session %q merged into %q and cleaned up", sess.ID, baseBranch),
		Preview: preview,
		Session: sess,
	}, nil
}

// cleanup removes the session worktree and deletes the session branch.
// Worktree removal is attempted first; if it fails we still try to delete the
// branch and report the first error.
func (w *SessionFinishWorkflow) cleanup(sess *domain.Session) error {
	var firstErr error
	if sess.Worktree != "" {
		if err := w.git.RemoveWorktree(sess.Worktree); err != nil {
			firstErr = err
		}
	}
	if _, derr := w.git.DeleteBranch(sess.Branch, true); derr != nil && firstErr == nil {
		firstErr = derr
	}
	return firstErr
}

// switchInDir is a placeholder for switching branches in a specific directory.
// The current ExecAdapter operates on its configured workDir, so the caller
// must ensure the adapter is bound to mainRepoRoot. This helper exists so the
// finish workflow can be made directory-aware without changing the Git port.
func switchInDir(git ports.Git, dir, branch string) error {
	// The ExecAdapter resolves its workDir at construction time. When the
	// session workflow runs from a worktree, the adapter must already point
	// at the main repo root (resolved from GitCommonDir). We rely on the
	// caller constructing the workflow with the main repo root as workDir.
	return git.Switch(branch)
}