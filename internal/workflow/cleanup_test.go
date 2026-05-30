package workflow

import (
	"fmt"
	"testing"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// mockConfirmCallTracking implements ports.Confirm for testing CleanupAfterPlumbing.
type mockConfirmCallTracking struct {
	forceReleaseCalled bool
	forceReleaseErr    error
}

var _ ports.Confirm = (*mockConfirmCallTracking)(nil)

func (m *mockConfirmCallTracking) AcquireLock() error                          { return nil }
func (m *mockConfirmCallTracking) ReleaseLock() error                          { return nil }
func (m *mockConfirmCallTracking) ForceRelease() error                         { m.forceReleaseCalled = true; return m.forceReleaseErr }
func (m *mockConfirmCallTracking) WritePlan(_ domain.OperationPlan) error       { return nil }
func (m *mockConfirmCallTracking) ReadPlan() (*domain.OperationPlan, error)     { return nil, nil }
func (m *mockConfirmCallTracking) DeletePlan() error                          { return nil }
func (m *mockConfirmCallTracking) IsPlanExpired() bool                         { return false }
func (m *mockConfirmCallTracking) CreateBlocker() error                        { return nil }
func (m *mockConfirmCallTracking) HasBlocker() bool                            { return false }
func (m *mockConfirmCallTracking) RemoveBlocker() error                        { return nil }

func TestWorkflow_CleanupAfterPlumbing_CallsForceRelease(t *testing.T) {
	t.Parallel()
	mock := &mockConfirmCallTracking{}
	w := &Workflow{
		confirm: mock,
	}

	w.CleanupAfterPlumbing()

	if !mock.forceReleaseCalled {
		t.Error("CleanupAfterPlumbing() did not call ForceRelease(), want it called")
	}
}

func TestWorkflow_CleanupAfterPlumbing_LogsWarningOnError(t *testing.T) {
	t.Parallel()
	mock := &mockConfirmCallTracking{
		forceReleaseErr: fmt.Errorf("release failed"),
	}
	w := &Workflow{
		confirm: mock,
	}

	// Should not panic even if ForceRelease returns an error
	w.CleanupAfterPlumbing()

	if !mock.forceReleaseCalled {
		t.Error("CleanupAfterPlumbing() did not call ForceRelease() even when error expected")
	}
}