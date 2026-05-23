package confirm

import (
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
)

// compile-time check: InMemoryConfirm satisfies ports.Confirm (will fail until ForceRelease is in the interface)
var _ ports.Confirm = (*InMemoryConfirm)(nil)

func TestInMemoryConfirm_PortsConfirmHasForceRelease(t *testing.T) {
	t.Parallel()
	// This test confirms that the ports.Confirm interface includes ForceRelease().
	// If ForceRelease is NOT in the interface, the call below won't compile.
	var c ports.Confirm = NewInMemory(5 * time.Minute)
	if err := c.ForceRelease(); err != nil {
		t.Errorf("ports.Confirm.ForceRelease() error = %v, want nil", err)
	}
}

func TestInMemoryConfirm_ForceRelease_ClearsStuckLockAfterPlanExpiry(t *testing.T) {
	t.Parallel()
	ttl := 100 * time.Millisecond
	c := NewInMemory(ttl)

	// Acquire lock and write plan
	if err := c.AcquireLock(); err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}
	if err := c.WritePlan(domain.OperationPlan{Operation: "commit", Preview: "test", CreatedAt: time.Now().Unix()}); err != nil {
		t.Fatalf("WritePlan() error = %v", err)
	}
	if err := c.CreateBlocker(); err != nil {
		t.Fatalf("CreateBlocker() error = %v", err)
	}

	// Wait for TTL to expire
	time.Sleep(ttl + 50*time.Millisecond)

	// ForceRelease should clear everything
	if err := c.ForceRelease(); err != nil {
		t.Fatalf("ForceRelease() error = %v", err)
	}

	// After ForceRelease, AcquireLock should succeed
	if err := c.AcquireLock(); err != nil {
		t.Errorf("AcquireLock() after ForceRelease error = %v, want nil", err)
	}
}

func TestInMemoryConfirm_ForceRelease_OnAlreadyReleasedLockIsNoOp(t *testing.T) {
	t.Parallel()
	c := NewInMemory(5 * time.Minute)

	// Release on never-acquired lock — should be no-op
	if err := c.ForceRelease(); err != nil {
		t.Errorf("ForceRelease() on already-released lock error = %v, want nil", err)
	}

	// Acquire and release normally
	_ = c.AcquireLock()
	_ = c.ReleaseLock()

	// ForceRelease after normal release — should be no-op
	if err := c.ForceRelease(); err != nil {
		t.Errorf("ForceRelease() on already-released lock error = %v, want nil", err)
	}
}

func TestInMemoryConfirm_AcquireLock_AutoReleasesExpiredPlan(t *testing.T) {
	t.Parallel()
	ttl := 100 * time.Millisecond
	c := NewInMemory(ttl)

	// Acquire lock and write plan
	if err := c.AcquireLock(); err != nil {
		t.Fatalf("first AcquireLock() error = %v", err)
	}
	if err := c.WritePlan(domain.OperationPlan{Operation: "commit", Preview: "test", CreatedAt: time.Now().Unix()}); err != nil {
		t.Fatalf("WritePlan() error = %v", err)
	}

	// Wait for plan to expire
	time.Sleep(ttl + 50*time.Millisecond)

	// Second AcquireLock should auto-release the stale lock and succeed
	if err := c.AcquireLock(); err != nil {
		t.Errorf("AcquireLock() with expired plan error = %v, want nil (auto-release)", err)
	}
}

func TestInMemoryConfirm_AcquireLock_RejectsWhenPlanNotExpired(t *testing.T) {
	t.Parallel()
	c := NewInMemory(5 * time.Minute)

	// Acquire lock and write plan
	if err := c.AcquireLock(); err != nil {
		t.Fatalf("first AcquireLock() error = %v", err)
	}
	if err := c.WritePlan(domain.OperationPlan{Operation: "commit", Preview: "test", CreatedAt: time.Now().Unix()}); err != nil {
		t.Fatalf("WritePlan() error = %v", err)
	}

	// Immediate second AcquireLock should fail (plan not expired)
	err := c.AcquireLock()
	if err == nil {
		t.Error("AcquireLock() when plan is active = nil, want ErrOperationInProgress")
	}
	if err != ErrOperationInProgress {
		t.Errorf("AcquireLock() error = %v, want ErrOperationInProgress", err)
	}
}