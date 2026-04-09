package git

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

const defaultConfirmTTL = 10 * time.Minute

// ConfirmAdapter implements ports.ConfirmPort for any git operation requiring confirmation.
// It persists state to disk so the plan survives between MCP tool calls.
type ConfirmAdapter struct {
	planFile    string
	blockerFile string
	ttl         time.Duration
}

// NewConfirmAdapter creates a ConfirmAdapter backed by files in workDir/.gcourer/
func NewConfirmAdapter(workDir string) *ConfirmAdapter {
	return &ConfirmAdapter{
		planFile:    filepath.Join(workDir, ".gcourer", "gcourer_plan.json"),
		blockerFile: filepath.Join(workDir, ".gcourer", "gcourer_plan.lock"),
		ttl:         defaultConfirmTTL,
	}
}

// WritePlan persists the plan atomically to disk.
func (a *ConfirmAdapter) WritePlan(plan ports.OperationPlan) error {
	dir := filepath.Dir(a.planFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
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
		os.Remove(tmp)
		return fmt.Errorf("failed to rename plan: %w", err)
	}
	return nil
}

// ReadPlan reads the persisted plan. Returns nil, nil if no plan exists.
func (a *ConfirmAdapter) ReadPlan() (*ports.OperationPlan, error) {
	data, err := os.ReadFile(a.planFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read plan: %w", err)
	}
	var plan ports.OperationPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w", err)
	}
	return &plan, nil
}

// DeletePlan removes the plan file and blocker. Idempotent.
func (a *ConfirmAdapter) DeletePlan() error {
	if err := os.Remove(a.planFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete plan: %w", err)
	}
	if err := os.Remove(a.blockerFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete blocker: %w", err)
	}
	return nil
}

// CreateBlocker creates the blocker file that signals a pending approval.
func (a *ConfirmAdapter) CreateBlocker() error {
	dir := filepath.Dir(a.blockerFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create blocker directory: %w", err)
	}
	if err := os.WriteFile(a.blockerFile, []byte{}, 0644); err != nil {
		return fmt.Errorf("failed to create blocker: %w", err)
	}
	return nil
}

// HasBlocker returns true if the blocker file exists (pending approval).
func (a *ConfirmAdapter) HasBlocker() bool {
	_, err := os.Stat(a.blockerFile)
	return err == nil
}

// RemoveBlocker deletes the blocker file. Idempotent.
func (a *ConfirmAdapter) RemoveBlocker() error {
	if err := os.Remove(a.blockerFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove blocker: %w", err)
	}
	return nil
}

// IsPlanExpired returns true if the plan file is older than the TTL.
func (a *ConfirmAdapter) IsPlanExpired() bool {
	info, err := os.Stat(a.planFile)
	if err != nil {
		return true
	}
	return time.Since(info.ModTime()) > a.ttl
}

var _ ports.ConfirmPort = (*ConfirmAdapter)(nil)
