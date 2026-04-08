// Package commit provides the commit use case: analyze changes,
// generate commit messages via LLM, and execute commits with rollback on failure.
// Uses a pipeline pattern for maximum speed with large diffs.
package commit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/infra/config"
)

const (
	// PlanFileName is the filename for the persisted plan
	PlanFileName = "git-courer_plan.json"
	// BlockerFileName is the filename that blocks execution until user approves
	BlockerFileName = "git-courer_commit.lock"
	// DefaultTTL is the default time-to-live for a plan if not configured
	DefaultTTL = 10 * time.Minute
)

// Plan represents a commit plan that can be persisted and reviewed before execution.
// It tracks the proposed commit message, the subcommand to execute, a preview flag,
// and timing information for TTL enforcement.
type Plan struct {
	Message         string    `json:"message"`          // The generated commit message
	RejectedMessage string    `json:"rejected_message"` // Previous message rejected by user (for retry context)
	SubCommand      string    `json:"sub_command"`      // The git subcommand (commit, branch, etc.)
	Preview         bool      `json:"preview"`          // Whether this is a preview (no execution)
	TTL             int64     `json:"ttl"`              // Time-to-live in seconds (for comparison with CreatedAt)
	CreatedAt       time.Time `json:"created_at"`       // When the plan was created
	// Commits is the number of commits that would be created (for ResetSoft on cleanup)
	Commits int `json:"commits"`
}

// planStore handles persistence of plans to disk using atomic writes.
type planStore struct {
	planFilePath    string
	blockerFilePath string
}

// newPlanStore creates a planStore with the default path from config.
func newPlanStore(cfg *config.Config) *planStore {
	return &planStore{
		planFilePath:    filepath.Join(cfg.Git.WorkDir, cfg.Commit.PlanFile),
		blockerFilePath: filepath.Join(cfg.Git.WorkDir, ".gcourer", BlockerFileName),
	}
}

// CreateBlocker creates the blocker file to prevent execution until approval.
func (s *planStore) CreateBlocker() error {
	dir := filepath.Dir(s.blockerFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create blocker directory: %w", err)
	}
	if err := os.WriteFile(s.blockerFilePath, []byte{}, 0644); err != nil {
		return fmt.Errorf("failed to create blocker file: %w", err)
	}
	return nil
}

// HasBlocker checks if the blocker file exists.
func (s *planStore) HasBlocker() bool {
	_, err := os.Stat(s.blockerFilePath)
	return err == nil
}

// RemoveBlocker deletes the blocker file.
func (s *planStore) RemoveBlocker() error {
	if err := os.Remove(s.blockerFilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove blocker: %w", err)
	}
	return nil
}

// WritePlan writes the plan to disk using atomic write (temp file + rename).
// This ensures the plan file is either fully written or not present at all.
func (s *planStore) WritePlan(plan Plan) error {
	// Ensure directory exists
	dir := filepath.Dir(s.planFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create plan directory: %w", err)
	}

	// Serialize plan to JSON
	data, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	// Write to temp file first (atomic write)
	tmpPath := s.planFilePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp plan file: %w", err)
	}

	// Rename temp file to actual plan file (atomic on POSIX)
	if err := os.Rename(tmpPath, s.planFilePath); err != nil {
		// Clean up temp file on failure
		os.Remove(tmpPath)
		return fmt.Errorf("failed to atomically rename plan file: %w", err)
	}

	return nil
}

// ReadPlan reads and parses the persisted plan from disk.
// Returns nil, nil if no plan exists.
func (s *planStore) ReadPlan() (*Plan, error) {
	data, err := os.ReadFile(s.planFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No plan exists
		}
		return nil, fmt.Errorf("failed to read plan file: %w", err)
	}

	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan file: %w", err)
	}

	return &plan, nil
}

// IsPlanExpired checks if the plan's TTL has been exceeded.
// TTL is compared against CreatedAt + TTL duration.
func IsPlanExpired(plan Plan) bool {
	if plan.TTL <= 0 {
		// No TTL set, use default
		return time.Since(plan.CreatedAt) > DefaultTTL
	}
	ttl := time.Duration(plan.TTL) * time.Second
	return time.Since(plan.CreatedAt) > ttl
}

// DeletePlan removes the plan file and blocker from disk.
// Returns nil if files don't exist (idempotent).
func (s *planStore) DeletePlan() error {
	// Remove plan file
	if err := os.Remove(s.planFilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete plan file: %w", err)
	}
	// Remove blocker file
	if err := os.Remove(s.blockerFilePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete blocker file: %w", err)
	}
	return nil
}

// PlanFileExists checks if the plan file exists on disk.
func (s *planStore) PlanFileExists() bool {
	_, err := os.Stat(s.planFilePath)
	return err == nil
}
