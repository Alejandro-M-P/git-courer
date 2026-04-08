// Package commit provides the commit use case: analyze changes,
// generate commit messages via LLM, and execute commits with rollback on failure.
// Uses a pipeline pattern for maximum speed with large diffs.
package commit

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/config"
)

const (
	// LockFileName is the filename for the lock file
	LockFileName = "git-courer.lock"
	// LockTTL is how long a lock is considered valid before being considered stale
	LockTTL = 24 * time.Hour
)

// ConfirmationState represents the state of a pending confirmation.
type ConfirmationState int

const (
	// StatePending means waiting for user confirmation
	StatePending ConfirmationState = iota
	// StateApproved means the user approved the plan
	StateApproved
	// StateAborted means the user aborted the plan
	StateAborted
)

func (s ConfirmationState) String() string {
	switch s {
	case StatePending:
		return "pending"
	case StateApproved:
		return "approved"
	case StateAborted:
		return "aborted"
	default:
		return "unknown"
	}
}

// BlockingManager handles blocking wait for user confirmation using sync.Cond.
// It coordinates between the goroutine executing the plan and the goroutine
// waiting for user confirmation.
type BlockingManager struct {
	mu         sync.Mutex
	cond       *sync.Cond
	state      ConfirmationState
	plan       *Plan
	git        ports.Git
	cfg        *config.Config
	lockFile   string
	planStore  *planStore
	commitsRef int // Reference count of commits to rollback on abort
}

// NewBlockingManager creates a new BlockingManager.
func NewBlockingManager(git ports.Git, cfg *config.Config) *BlockingManager {
	bm := &BlockingManager{
		git:       git,
		cfg:       cfg,
		state:     StatePending,
		planStore: newPlanStore(cfg),
	}
	bm.cond = sync.NewCond(&bm.mu)
	bm.lockFile = filepath.Join(cfg.Git.WorkDir, cfg.Commit.LockFile)
	return bm
}

// SetPlan sets the plan to be confirmed and stores it persistently.
func (bm *BlockingManager) SetPlan(plan *Plan) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	bm.plan = plan

	// Persist the plan
	if err := bm.planStore.WritePlan(*plan); err != nil {
		return fmt.Errorf("failed to persist plan: %w", err)
	}

	return nil
}

// WaitForConfirmation blocks until the user either approves or aborts the plan.
// Returns the final state (Approved or Aborted).
func (bm *BlockingManager) WaitForConfirmation() ConfirmationState {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Wait until state is no longer pending
	for bm.state == StatePending {
		bm.cond.Wait()
	}

	return bm.state
}

// Approve transitions the state to Approved and signals all waiting goroutines.
func (bm *BlockingManager) Approve() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.state != StatePending {
		return // Already decided
	}

	bm.state = StateApproved
	bm.cond.Broadcast()
}

// Abort transitions the state to Aborted, signals all waiting goroutines,
// and cleans up any persisted plan.
func (bm *BlockingManager) Abort() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if bm.state != StatePending {
		return // Already decided
	}

	bm.state = StateAborted
	bm.cond.Broadcast()

	// Clean up persisted plan
	bm.planStore.DeletePlan()
}

// GetState returns the current confirmation state without blocking.
func (bm *BlockingManager) GetState() ConfirmationState {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return bm.state
}

// GetPlan returns the current plan if one is set.
func (bm *BlockingManager) GetPlan() *Plan {
	bm.mu.Lock()
	defer bm.mu.Unlock()
	return bm.plan
}

// AcquireLock attempts to acquire the lock file.
// Returns nil if lock acquired, error if already locked.
func (bm *BlockingManager) AcquireLock() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(bm.lockFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create lock directory: %w", err)
	}

	// Try to create lock file (exclusive create)
	file, err := os.OpenFile(bm.lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("lock already held by another process")
		}
		return fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer file.Close()

	// Write lock metadata
	lockInfo := fmt.Sprintf("pid=%d\ntime=%s\n", os.Getpid(), time.Now().Format(time.RFC3339))
	if _, err := file.WriteString(lockInfo); err != nil {
		os.Remove(bm.lockFile)
		return fmt.Errorf("failed to write lock info: %w", err)
	}

	return nil
}

// ReleaseLock releases the lock file.
func (bm *BlockingManager) ReleaseLock() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if err := os.Remove(bm.lockFile); err != nil {
		if os.IsNotExist(err) {
			return nil // Already released
		}
		return fmt.Errorf("failed to release lock: %w", err)
	}
	return nil
}

// IsLockStale checks if the lock file exists but is old (potential stale lock from crashed process).
func (bm *BlockingManager) IsLockStale() (bool, error) {
	info, err := os.Stat(bm.lockFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil // No lock file
		}
		return false, err
	}

	// Check if lock is older than LockTTL
	return time.Since(info.ModTime()) > LockTTL, nil
}

// CleanupAndReset performs crash recovery:
// 1. Removes stale lock file
// 2. Removes any persisted plan
// 3. If there was a plan with commits, performs ResetSoft to undo them
func (bm *BlockingManager) CleanupAndReset() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	var cleanupErr error

	// Check for stale lock
	isStale, err := bm.IsLockStale()
	if err != nil {
		cleanupErr = fmt.Errorf("failed to check stale lock: %w", err)
	} else if isStale {
		if err := os.Remove(bm.lockFile); err != nil && !os.IsNotExist(err) {
			cleanupErr = fmt.Errorf("failed to remove stale lock: %w", err)
		}
	}

	// Read and delete any persisted plan
	plan, err := bm.planStore.ReadPlan()
	if err != nil {
		cleanupErr = fmt.Errorf("failed to read plan for cleanup: %w", err)
	}

	// Delete the plan file
	if err := bm.planStore.DeletePlan(); err != nil {
		cleanupErr = fmt.Errorf("failed to delete plan: %w", err)
	}

	// If there was a plan with commits, rollback
	if plan != nil && plan.Commits > 0 {
		if err := bm.git.ResetSoft(plan.Commits); err != nil {
			cleanupErr = fmt.Errorf("failed to reset %d commits: %w", plan.Commits, err)
		}
	}

	return cleanupErr
}

// CheckAndCleanupStaleResources checks for stale lock+plan on startup and cleans up if needed.
// This should be called once at application startup.
func (bm *BlockingManager) CheckAndCleanupStaleResources() error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Check for stale lock
	isStale, err := bm.IsLockStale()
	if err != nil {
		return fmt.Errorf("failed to check stale lock: %w", err)
	}

	if isStale {
		// Remove stale lock
		os.Remove(bm.lockFile)
	}

	// Check for existing plan
	plan, err := bm.planStore.ReadPlan()
	if err != nil {
		return fmt.Errorf("failed to read plan: %w", err)
	}

	if plan != nil {
		// Check if plan is expired
		if IsPlanExpired(*plan) {
			// Clean up expired plan and rollback commits
			bm.planStore.DeletePlan()
			if plan.Commits > 0 {
				if err := bm.git.ResetSoft(plan.Commits); err != nil {
					return fmt.Errorf("failed to reset %d commits from expired plan: %w", plan.Commits, err)
				}
			}
		}
	}

	return nil
}
