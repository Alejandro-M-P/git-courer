package confirm

import (
	"sync"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

type InMemoryConfirm struct {
	mu           sync.Mutex
	plan         *domain.OperationPlan
	blocker      bool
	lockAcquired bool
	ttl          time.Duration
}

func NewInMemory(ttl time.Duration) *InMemoryConfirm {
	return &InMemoryConfirm{
		ttl: ttl,
	}
}

func (a *InMemoryConfirm) AcquireLock() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lockAcquired {
		return ErrOperationInProgress
	}
	a.lockAcquired = true
	return nil
}

func (a *InMemoryConfirm) ReleaseLock() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lockAcquired = false
	return nil
}

func (a *InMemoryConfirm) WritePlan(plan domain.OperationPlan) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	plan.CreatedAt = time.Now().Unix()
	a.plan = &plan
	return nil
}

func (a *InMemoryConfirm) ReadPlan() (*domain.OperationPlan, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.plan == nil {
		return nil, nil
	}
	return a.plan, nil
}

func (a *InMemoryConfirm) DeletePlan() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.plan = nil
	a.blocker = false
	return nil
}

func (a *InMemoryConfirm) IsPlanExpired() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.plan == nil {
		return true
	}
	return time.Since(time.Unix(a.plan.CreatedAt, 0)) > a.ttl
}

func (a *InMemoryConfirm) CreateBlocker() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.blocker = true
	return nil
}

func (a *InMemoryConfirm) HasBlocker() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.blocker
}

func (a *InMemoryConfirm) RemoveBlocker() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.blocker = false
	return nil
}

var _ ports.Confirm = (*InMemoryConfirm)(nil)

var ErrOperationInProgress = &confirmError{"another operation is in progress"}

type confirmError struct {
	msg string
}

func (e *confirmError) Error() string {
	return e.msg
}
