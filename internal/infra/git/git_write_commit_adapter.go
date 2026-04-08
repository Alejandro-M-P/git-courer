package git

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// GitWriteCommitAdapter implements ports.GitWriteCommitPort for commit operations
type GitWriteCommitAdapter struct {
	exec      *ExecAdapter
	planFile  string
	lockFile  string
	ttl       time.Duration
	mu        sync.Mutex
	cond      *sync.Cond
	confirmed bool
	aborted   bool
	state     string // "pending", "approved", "aborted"
}

// NewGitWriteCommitAdapter creates a new GitWriteCommitAdapter
func NewGitWriteCommitAdapter(workDir string) *GitWriteCommitAdapter {
	planFile := filepath.Join(workDir, ".gcourer", "git-courer_plan.json")
	lockFile := filepath.Join(workDir, ".gcourer", "git-courer.lock")

	a := &GitWriteCommitAdapter{
		exec:     NewExecAdapter(workDir),
		planFile: planFile,
		lockFile: lockFile,
		ttl:      10 * time.Minute,
		state:    "none",
	}
	a.cond = sync.NewCond(&a.mu)
	return a
}

// ConfigureForCommit sets configuration for commit operations
func (a *GitWriteCommitAdapter) ConfigureForCommit(ttl time.Duration) {
	a.ttl = ttl
}

// WritePlan writes the commit plan to disk
func (a *GitWriteCommitAdapter) WritePlan(plan ports.CommitPlan) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Ensure directory exists
	dir := filepath.Dir(a.planFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Use the port's CommitPlan which has proper JSON tags
	plan.CreatedAt = time.Now().Unix()

	// Write atomically using proper JSON marshaling
	tmpFile := a.planFile + ".tmp"
	data, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write plan: %w", err)
	}

	if err := os.Rename(tmpFile, a.planFile); err != nil {
		return fmt.Errorf("failed to rename plan: %w", err)
	}

	return nil
}

// ReadPlan reads the commit plan from disk
func (a *GitWriteCommitAdapter) ReadPlan() (*ports.CommitPlan, error) {
	data, err := os.ReadFile(a.planFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read plan: %w", err)
	}

	var plan ports.CommitPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w", err)
	}
	return &plan, nil
}

// IsPlanExpired checks if the plan has expired
func (a *GitWriteCommitAdapter) IsPlanExpired() bool {
	info, err := os.Stat(a.planFile)
	if err != nil {
		return true // If file doesn't exist, consider it expired
	}

	age := time.Since(info.ModTime())
	return age > a.ttl
}

// DeletePlan removes the plan file
func (a *GitWriteCommitAdapter) DeletePlan() error {
	if err := os.Remove(a.planFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete plan: %w", err)
	}
	return nil
}

// AcquireLock acquires the lock file
func (a *GitWriteCommitAdapter) AcquireLock() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if lock exists
	if _, err := os.Stat(a.lockFile); err == nil {
		return fmt.Errorf("operation in progress")
	}

	// Create lock file
	dir := filepath.Dir(a.lockFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create lock directory: %w", err)
	}

	lockData := fmt.Sprintf("pid=%d", os.Getpid())
	if err := os.WriteFile(a.lockFile, []byte(lockData), 0644); err != nil {
		return fmt.Errorf("failed to create lock: %w", err)
	}

	return nil
}

// ReleaseLock releases the lock file
func (a *GitWriteCommitAdapter) ReleaseLock() error {
	if err := os.Remove(a.lockFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lock: %w", err)
	}
	return nil
}

// IsLocked checks if the lock is held
func (a *GitWriteCommitAdapter) IsLocked() bool {
	_, err := os.Stat(a.lockFile)
	return err == nil
}

// WaitForConfirmation blocks until user confirms or aborts
func (a *GitWriteCommitAdapter) WaitForConfirmation() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.state = "pending"
	for !a.confirmed && !a.aborted {
		a.cond.Wait()
	}

	if a.confirmed {
		a.state = "approved"
		return true
	}
	a.state = "aborted"
	return false
}

// Approve signals that the user approved the operation
func (a *GitWriteCommitAdapter) Approve() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.confirmed = true
	a.cond.Signal()
}

// Abort signals that the user aborted the operation
func (a *GitWriteCommitAdapter) Abort() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.aborted = true
	a.cond.Signal()
}

// GetState returns the current confirmation state: 0=pending, 1=approved, 2=aborted
func (a *GitWriteCommitAdapter) GetState() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.aborted {
		return 2 // StateAborted
	}
	if a.confirmed {
		return 1 // StateApproved
	}
	return 0 // StatePending
}

// Execute is a placeholder - the actual commit execution uses the existing commit service
func (a *GitWriteCommitAdapter) Execute(instruction string, preview bool) (string, error) {
	return "", fmt.Errorf("Execute should not be called directly - use commit service")
}

// Ensure GitWriteCommitAdapter implements ports.GitWriteCommitPort
var _ ports.GitWriteCommitPort = (*GitWriteCommitAdapter)(nil)
