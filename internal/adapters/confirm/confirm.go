// Package confirm implements ports.Confirm using files on disk.
// Plan, lock, and blocker are all file-based for crash safety and cross-process visibility.
package confirm

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// Adapter implements ports.Confirm.
// All paths and TTL come from config — nothing hardcoded.
type Adapter struct {
	planFile    string
	lockFile    string
	blockerFile string
	ttl         time.Duration
	mu          sync.Mutex
}

// New creates a Confirm Adapter from config.
func New(workDir string, cfg config.CommitConfig) *Adapter {
	return &Adapter{
		planFile:    filepath.Join(workDir, cfg.PlanFile),
		lockFile:    filepath.Join(workDir, cfg.LockFile),
		blockerFile: filepath.Join(workDir, cfg.BlockerFile),
		ttl:         cfg.TTL.Duration,
	}
}

// ReviewConfig returns a CommitConfig suitable for review operations.
// Uses separate file paths to avoid conflicts with commit operations.
func ReviewConfig(baseCfg config.CommitConfig) config.CommitConfig {
	return config.CommitConfig{
		TTL:         baseCfg.TTL,
		LockFile:    ".gcourer/git-courer-review.lock",
		PlanFile:    ".gcourer/git-courer_review_plan.json",
		BlockerFile: ".gcourer/git-courer_review.lock",
	}
}

// AcquireLock creates the lock file. Returns error if already locked.
func (a *Adapter) AcquireLock() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, err := os.Stat(a.lockFile); err == nil {
		return fmt.Errorf("another operation is in progress")
	}
	if err := os.MkdirAll(filepath.Dir(a.lockFile), 0755); err != nil {
		return fmt.Errorf("failed to create lock directory: %w", err)
	}
	if err := os.WriteFile(a.lockFile, []byte(fmt.Sprintf("pid=%d", os.Getpid())), 0644); err != nil {
		return fmt.Errorf("failed to create lock: %w", err)
	}
	return nil
}

// ReleaseLock removes the lock file (idempotent).
func (a *Adapter) ReleaseLock() error {
	if _, err := os.Stat(a.lockFile); os.IsNotExist(err) {
		return nil
	}
	if err := os.Remove(a.lockFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lock: %w", err)
	}
	return nil
}

// WritePlan persists an operation plan to disk atomically.
func (a *Adapter) WritePlan(plan domain.OperationPlan) error {
	if err := os.MkdirAll(filepath.Dir(a.planFile), 0755); err != nil {
		return fmt.Errorf("failed to create plan directory: %w", err)
	}
	plan.CreatedAt = time.Now().Unix()
	data, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}
	tmp := a.planFile + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("failed to write plan: %w", err)
	}
	if err := os.Rename(tmp, a.planFile); err != nil {
		return fmt.Errorf("failed to rename plan: %w", err)
	}
	return nil
}

// ReadPlan reads the persisted operation plan.
func (a *Adapter) ReadPlan() (*domain.OperationPlan, error) {
	data, err := os.ReadFile(a.planFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read plan: %w", err)
	}
	var plan domain.OperationPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w", err)
	}
	return &plan, nil
}

// DeletePlan removes the plan file and the blocker.
func (a *Adapter) DeletePlan() error {
	if err := os.Remove(a.planFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete plan: %w", err)
	}
	if err := os.Remove(a.blockerFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete blocker: %w", err)
	}
	return nil
}

// IsPlanExpired returns true if the plan is older than the configured TTL.
func (a *Adapter) IsPlanExpired() bool {
	info, err := os.Stat(a.planFile)
	if err != nil {
		return true
	}
	return time.Since(info.ModTime()) > a.ttl
}

// CreateBlocker creates the blocker file to prevent execution until approval.
func (a *Adapter) CreateBlocker() error {
	if err := os.MkdirAll(filepath.Dir(a.blockerFile), 0755); err != nil {
		return fmt.Errorf("failed to create blocker directory: %w", err)
	}
	if err := os.WriteFile(a.blockerFile, []byte{}, 0644); err != nil {
		return fmt.Errorf("failed to create blocker: %w", err)
	}
	return nil
}

// HasBlocker returns true if the blocker file exists.
func (a *Adapter) HasBlocker() bool {
	_, err := os.Stat(a.blockerFile)
	return err == nil
}

// RemoveBlocker removes the blocker file (idempotent).
func (a *Adapter) RemoveBlocker() error {
	if err := os.Remove(a.blockerFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove blocker: %w", err)
	}
	return nil
}

// Compile-time interface check.
var _ ports.Confirm = (*Adapter)(nil)
