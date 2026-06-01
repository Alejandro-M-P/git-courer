package ports

import "github.com/blak0p/git-courer/internal/core/domain"

// Confirm handles the plan/blocker/lock lifecycle for any confirmable operation.
// Three-phase protocol: START (saves plan + blocker) → APPLY (reads plan + executes) → ABORT.
type Confirm interface {
	// Lock management — prevents concurrent operations.
	AcquireLock() error
	ReleaseLock() error
	ForceRelease() error

	// Plan management — persists the pending operation to disk.
	WritePlan(plan domain.OperationPlan) error
	ReadPlan() (*domain.OperationPlan, error)
	DeletePlan() error
	IsPlanExpired() bool

	// Blocker management — signals that an operation is waiting for user approval.
	CreateBlocker() error
	HasBlocker() bool
	RemoveBlocker() error
}
