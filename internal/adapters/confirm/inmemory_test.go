package confirm

import (
	"testing"
	"time"

	"github.com/blak0p/git-courer/internal/core/domain"
	"github.com/blak0p/git-courer/internal/core/ports"
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

// ---------------------------------------------------------------------------
// Pre-existing method coverage (not part of bugfix-batch-1 but uncovered)
// ---------------------------------------------------------------------------

func TestInMemoryConfirm_ReadPlan_NilReturnsNil(t *testing.T) {
	t.Parallel()
	c := NewInMemory(5 * time.Minute)
	plan, err := c.ReadPlan()
	if err != nil {
		t.Fatalf("ReadPlan() error = %v", err)
	}
	if plan != nil {
		t.Errorf("ReadPlan() on empty = %v, want nil", plan)
	}
}

func TestInMemoryConfirm_ReadPlan_ReturnsWrittenPlan(t *testing.T) {
	t.Parallel()
	c := NewInMemory(5 * time.Minute)
	wantPlan := domain.OperationPlan{Operation: "commit", Preview: "test preview"}
	if err := c.WritePlan(wantPlan); err != nil {
		t.Fatalf("WritePlan() error = %v", err)
	}
	got, err := c.ReadPlan()
	if err != nil {
		t.Fatalf("ReadPlan() error = %v", err)
	}
	if got.Operation != wantPlan.Operation {
		t.Errorf("ReadPlan().Operation = %q, want %q", got.Operation, wantPlan.Operation)
	}
	if got.Preview != wantPlan.Preview {
		t.Errorf("ReadPlan().Preview = %q, want %q", got.Preview, wantPlan.Preview)
	}
}

func TestInMemoryConfirm_DeletePlan_ClearsPlanAndBlocker(t *testing.T) {
	t.Parallel()
	c := NewInMemory(5 * time.Minute)
	_ = c.WritePlan(domain.OperationPlan{Operation: "commit"})
	_ = c.CreateBlocker()

	_ = c.DeletePlan()

	plan, _ := c.ReadPlan()
	if plan != nil {
		t.Error("DeletePlan(): plan still exists, want nil")
	}
	if c.HasBlocker() {
		t.Error("DeletePlan(): blocker still set, want false")
	}
}

func TestInMemoryConfirm_IsPlanExpired_NoPlanReturnsTrue(t *testing.T) {
	t.Parallel()
	c := NewInMemory(5 * time.Minute)
	if !c.IsPlanExpired() {
		t.Error("IsPlanExpired() with no plan = false, want true")
	}
}

func TestInMemoryConfirm_IsPlanExpired_FreshPlanReturnsFalse(t *testing.T) {
	t.Parallel()
	c := NewInMemory(5 * time.Minute)
	_ = c.WritePlan(domain.OperationPlan{Operation: "commit", CreatedAt: time.Now().Unix()})
	if c.IsPlanExpired() {
		t.Error("IsPlanExpired() with fresh plan = true, want false")
	}
}

func TestInMemoryConfirm_IsPlanExpired_OldPlanReturnsTrue(t *testing.T) {
	t.Parallel()
	ttl := 100 * time.Millisecond
	c := NewInMemory(ttl)
	_ = c.WritePlan(domain.OperationPlan{Operation: "commit", CreatedAt: time.Now().Unix()})
	time.Sleep(ttl + 50*time.Millisecond)
	if !c.IsPlanExpired() {
		t.Error("IsPlanExpired() with expired plan = false, want true")
	}
}

func TestInMemoryConfirm_HasBlocker_DefaultFalse(t *testing.T) {
	t.Parallel()
	c := NewInMemory(5 * time.Minute)
	if c.HasBlocker() {
		t.Error("HasBlocker() default = true, want false")
	}
}

func TestInMemoryConfirm_CreateAndRemoveBlocker(t *testing.T) {
	t.Parallel()
	c := NewInMemory(5 * time.Minute)
	_ = c.CreateBlocker()
	if !c.HasBlocker() {
		t.Error("HasBlocker() after CreateBlocker = false, want true")
	}
	_ = c.RemoveBlocker()
	if c.HasBlocker() {
		t.Error("HasBlocker() after RemoveBlocker = true, want false")
	}
}

func TestInMemoryConfirm_ReleaseLock_Idempotent(t *testing.T) {
	t.Parallel()
	c := NewInMemory(5 * time.Minute)
	_ = c.AcquireLock()
	_ = c.ReleaseLock()
	// Release again — should not panic or error
	if err := c.ReleaseLock(); err != nil {
		t.Errorf("ReleaseLock() second call error = %v, want nil", err)
	}
}

func TestInMemoryConfirm_WritePlanSetsCreatedAt(t *testing.T) {
	t.Parallel()
	c := NewInMemory(5 * time.Minute)
	before := time.Now().Unix()
	_ = c.WritePlan(domain.OperationPlan{Operation: "commit"})
	after := time.Now().Unix()

	plan, _ := c.ReadPlan()
	if plan.CreatedAt < before || plan.CreatedAt > after {
		t.Errorf("WritePlan() CreatedAt = %d, want between %d and %d", plan.CreatedAt, before, after)
	}
}
