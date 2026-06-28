package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
)

// FinishStatus is the high-level outcome of a session finish operation.
type FinishStatus string

const (
	FinishSuccess       FinishStatus = "success"
	FinishPreviewFailed FinishStatus = "preview_failed"
	FinishCleanupFailed FinishStatus = "cleanup_failed"
	FinishNotFound      FinishStatus = "not_found"
)

// FinishResult is the structured result of SessionFinishWorkflow.Finish.
type FinishResult struct {
	Status      FinishStatus    `json:"status"`
	Message     string          `json:"message"`
	Preview     *PreviewResult  `json:"preview,omitempty"`
	BranchAlive bool            `json:"branch_alive"`
	Session     *domain.Session `json:"session,omitempty"`
}

// SessionFinishWorkflow orchestrates the finish lifecycle: load session,
// preview-validate (uncommitted-changes guard + diff stats only), cleanup the
// worktree, and persist state. The session branch is left ALIVE for the user
// to integrate manually (merge, PR, or discard). No merge is attempted and no
// branch is deleted.
type SessionFinishWorkflow struct {
	git     ports.Git // worktree adapter (preview, cleanup)
	store   ports.SessionStore
	preview *PreviewEngine
	workDir string
}

// NewSessionFinishWorkflow builds a finish workflow backed by git, store, and a
// PreviewEngine rooted at workDir. The git port must address the session
// worktree (or the main repo when no session is active) so the uncommitted
// check and worktree removal land in the right place.
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
//  2. Run PreviewLight against the session's base branch: the only abort gate
//     is uncommitted/untracked changes (data-loss guard). Test status and
//     merge readiness are NOT checked — the user controls integration.
//  3. Cleanup: RemoveWorktree only. The session branch is NOT deleted so the
//     user can merge, open a PR, or `session discard` it later.
//  4. If cleanup fails, transition the session to cleanup_failed and persist
//     it. The branch is still alive; the user can retry cleanup or discard.
//  5. If cleanup succeeds, delete the session metadata file.
func (w *SessionFinishWorkflow) Finish(ctx context.Context, sessionID string) (*FinishResult, error) {
	sess, err := w.store.Get(sessionID)
	if err != nil {
		return &FinishResult{Status: FinishNotFound, Message: err.Error()}, nil
	}

	baseBranch := sess.BaseBranch
	if baseBranch == "" {
		baseBranch = "main"
	}

	// 2. Preview validation (light: uncommitted guard + diff stats only).
	preview, perr := w.preview.PreviewLight(ctx, baseBranch)
	if perr != nil {
		return &FinishResult{
			Status:  FinishPreviewFailed,
			Message: fmt.Sprintf("preview error: %v", perr),
			Session: sess,
		}, nil
	}

	// 3. Abort ONLY on uncommitted/untracked changes (data-loss risk).
	if preview.HasUncommitted {
		return &FinishResult{
			Status:  FinishPreviewFailed,
			Message: "session has uncommitted or untracked changes; commit or stash before finishing",
			Preview: preview,
			Session: sess,
		}, nil
	}

	// 4. Cleanup: remove the worktree only. The branch stays alive.
	cleanupErr := w.cleanup(sess)
	if cleanupErr != nil {
		sess.Status = domain.SessionCleanupFailed
		sess.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		_ = w.store.Save(sess)
		return &FinishResult{
			Status:      FinishCleanupFailed,
			Message:     fmt.Sprintf("cleanup failed: %v", cleanupErr),
			Preview:     preview,
			BranchAlive: true,
			Session:     sess,
		}, nil
	}

	// 5. Cleanup succeeded: delete the session metadata file.
	_ = w.store.Delete(sess.ID)

	return &FinishResult{
		Status:      FinishSuccess,
		Message:     fmt.Sprintf("session %q finished: worktree removed, branch %q alive for manual integration", sess.ID, sess.Branch),
		Preview:     preview,
		BranchAlive: true,
		Session:     sess,
	}, nil
}

// cleanup removes the session worktree. The session branch is intentionally
// NOT deleted — finish leaves the branch alive so the user can integrate
// (merge, PR) or discard it explicitly via `session discard`.
func (w *SessionFinishWorkflow) cleanup(sess *domain.Session) error {
	if sess.Worktree == "" {
		return nil
	}
	return w.git.RemoveWorktree(sess.Worktree)
}

