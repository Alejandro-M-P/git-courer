package confirm

import (
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/config"
	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
)

// testConfig returns a CommitConfig with deterministic file paths for tests.
func testConfig() config.CommitConfig {
	return config.CommitConfig{
		TTL:         config.NewDurationConfig(10 * time.Minute),
		PlanFile:    "plan.json",
		LockFile:    "git-courer.lock",
		BlockerFile: "git-courer.blocker",
	}
}

// TestNew verifies the constructor wires paths correctly.
func TestNew(t *testing.T) {
	dir := t.TempDir()
	cfg := testConfig()

	adapter := New(dir, cfg)

	if adapter == nil {
		t.Fatal("New() returned nil")
	}
	if adapter.planFile == "" || adapter.lockFile == "" || adapter.blockerFile == "" {
		t.Error("New() left file paths empty")
	}
}

// TestReviewConfig verifies ReviewConfig returns paths distinct from base.
func TestReviewConfig(t *testing.T) {
	base := testConfig()
	review := ReviewConfig(base)

	if review.PlanFile == base.PlanFile {
		t.Error("ReviewConfig should return a different PlanFile")
	}
	if review.LockFile == base.LockFile {
		t.Error("ReviewConfig should return a different LockFile")
	}
	if review.BlockerFile == base.BlockerFile {
		t.Error("ReviewConfig should return a different BlockerFile")
	}
	// TTL should be preserved
	if review.TTL != base.TTL {
		t.Error("ReviewConfig should preserve TTL")
	}
}

// TestAcquireAndReleaseLock tests the full lock lifecycle.
func TestAcquireAndReleaseLock(t *testing.T) {
	dir := t.TempDir()
	adapter := New(dir, testConfig())

	// Acquire lock on clean state
	if err := adapter.AcquireLock(); err != nil {
		t.Fatalf("AcquireLock() on clean state error = %v", err)
	}

	// Second acquire should fail (lock already held)
	if err := adapter.AcquireLock(); err == nil {
		t.Error("AcquireLock() should fail when lock already held")
	}

	// Release should succeed
	if err := adapter.ReleaseLock(); err != nil {
		t.Fatalf("ReleaseLock() error = %v", err)
	}

	// Re-acquire should succeed after release
	if err := adapter.AcquireLock(); err != nil {
		t.Fatalf("AcquireLock() after release error = %v", err)
	}
	adapter.ReleaseLock()
}

// TestReleaseLockIdempotent verifies releasing a non-existent lock is not an error.
func TestReleaseLockIdempotent(t *testing.T) {
	dir := t.TempDir()
	adapter := New(dir, testConfig())

	// Release with no lock file — should be a no-op
	if err := adapter.ReleaseLock(); err != nil {
		t.Errorf("ReleaseLock() with no lock should be idempotent, got error = %v", err)
	}
}

// TestWriteAndReadPlan tests plan persistence round-trip.
func TestWriteAndReadPlan(t *testing.T) {
	dir := t.TempDir()
	adapter := New(dir, testConfig())

	plan := domain.OperationPlan{
		Operation: "commit",
		Preview:   "feat: add tests",
		Messages:  []string{"feat: add tests"},
		Files:     []string{"confirm_test.go"},
	}

	if err := adapter.WritePlan(plan); err != nil {
		t.Fatalf("WritePlan() error = %v", err)
	}

	got, err := adapter.ReadPlan()
	if err != nil {
		t.Fatalf("ReadPlan() error = %v", err)
	}
	if got == nil {
		t.Fatal("ReadPlan() returned nil — expected a plan")
	}
	if got.Operation != plan.Operation {
		t.Errorf("ReadPlan().Operation = %q, want %q", got.Operation, plan.Operation)
	}
	if got.Preview != plan.Preview {
		t.Errorf("ReadPlan().Preview = %q, want %q", got.Preview, plan.Preview)
	}
	if len(got.Messages) != len(plan.Messages) {
		t.Errorf("ReadPlan().Messages len = %d, want %d", len(got.Messages), len(plan.Messages))
	}
	// CreatedAt should have been set by WritePlan
	if got.CreatedAt == 0 {
		t.Error("WritePlan() should set CreatedAt")
	}
}

// TestReadPlanNoFile returns nil when no plan file exists.
func TestReadPlanNoFile(t *testing.T) {
	dir := t.TempDir()
	adapter := New(dir, testConfig())

	got, err := adapter.ReadPlan()
	if err != nil {
		t.Fatalf("ReadPlan() with no file should not error, got = %v", err)
	}
	if got != nil {
		t.Error("ReadPlan() with no file should return nil plan")
	}
}

// TestDeletePlan verifies plan and blocker are removed.
func TestDeletePlan(t *testing.T) {
	dir := t.TempDir()
	adapter := New(dir, testConfig())

	// Write plan and create blocker first
	plan := domain.OperationPlan{Operation: "commit", Preview: "test"}
	if err := adapter.WritePlan(plan); err != nil {
		t.Fatalf("WritePlan() error = %v", err)
	}
	if err := adapter.CreateBlocker(); err != nil {
		t.Fatalf("CreateBlocker() error = %v", err)
	}

	// Verify both exist
	if _, err := adapter.ReadPlan(); err != nil {
		t.Fatalf("ReadPlan() before delete error = %v", err)
	}
	if !adapter.HasBlocker() {
		t.Fatal("HasBlocker() should be true before delete")
	}

	// Delete
	if err := adapter.DeletePlan(); err != nil {
		t.Fatalf("DeletePlan() error = %v", err)
	}

	// Plan should be gone
	got, err := adapter.ReadPlan()
	if err != nil {
		t.Fatalf("ReadPlan() after delete error = %v", err)
	}
	if got != nil {
		t.Error("ReadPlan() after delete should return nil")
	}

	// Blocker should be gone
	if adapter.HasBlocker() {
		t.Error("HasBlocker() after DeletePlan should be false")
	}
}

// TestDeletePlanIdempotent verifies deleting non-existent files is a no-op.
func TestDeletePlanIdempotent(t *testing.T) {
	dir := t.TempDir()
	adapter := New(dir, testConfig())

	if err := adapter.DeletePlan(); err != nil {
		t.Errorf("DeletePlan() with no files should be idempotent, got = %v", err)
	}
}

// TestIsPlanExpired returns true when no plan file exists.
func TestIsPlanExpiredNoFile(t *testing.T) {
	dir := t.TempDir()
	adapter := New(dir, testConfig())

	if !adapter.IsPlanExpired() {
		t.Error("IsPlanExpired() with no file should return true")
	}
}

// TestIsPlanExpiredFresh returns false for a plan just written.
func TestIsPlanExpiredFresh(t *testing.T) {
	dir := t.TempDir()
	adapter := New(dir, testConfig())

	plan := domain.OperationPlan{Operation: "commit", Preview: "test"}
	if err := adapter.WritePlan(plan); err != nil {
		t.Fatalf("WritePlan() error = %v", err)
	}

	if adapter.IsPlanExpired() {
		t.Error("IsPlanExpired() should be false for a freshly-written plan with 10m TTL")
	}
}

// TestIsPlanExpiredWithShortTTL returns true when TTL is in the past.
func TestIsPlanExpiredWithShortTTL(t *testing.T) {
	dir := t.TempDir()
	cfg := config.CommitConfig{
		TTL:         config.NewDurationConfig(0), // Expires immediately
		PlanFile:    "plan.json",
		LockFile:    "git-courer.lock",
		BlockerFile: "git-courer.blocker",
	}
	adapter := New(dir, cfg)

	plan := domain.OperationPlan{Operation: "commit", Preview: "test"}
	if err := adapter.WritePlan(plan); err != nil {
		t.Fatalf("WritePlan() error = %v", err)
	}

	// With TTL=0, the plan is expired immediately
	time.Sleep(1 * time.Millisecond)
	if !adapter.IsPlanExpired() {
		t.Error("IsPlanExpired() should be true with TTL=0")
	}
}

// TestCreateBlockerAndHasBlocker tests blocker lifecycle.
func TestCreateBlockerAndHasBlocker(t *testing.T) {
	dir := t.TempDir()
	adapter := New(dir, testConfig())

	// No blocker initially
	if adapter.HasBlocker() {
		t.Error("HasBlocker() should be false initially")
	}

	// Create blocker
	if err := adapter.CreateBlocker(); err != nil {
		t.Fatalf("CreateBlocker() error = %v", err)
	}

	// Blocker should now exist
	if !adapter.HasBlocker() {
		t.Error("HasBlocker() should be true after CreateBlocker()")
	}
}

// TestRemoveBlocker verifies removing the blocker clears the state.
func TestRemoveBlocker(t *testing.T) {
	dir := t.TempDir()
	adapter := New(dir, testConfig())

	if err := adapter.CreateBlocker(); err != nil {
		t.Fatalf("CreateBlocker() error = %v", err)
	}

	if err := adapter.RemoveBlocker(); err != nil {
		t.Fatalf("RemoveBlocker() error = %v", err)
	}

	if adapter.HasBlocker() {
		t.Error("HasBlocker() should be false after RemoveBlocker()")
	}
}

// TestRemoveBlockerIdempotent verifies removing a non-existent blocker is a no-op.
func TestRemoveBlockerIdempotent(t *testing.T) {
	dir := t.TempDir()
	adapter := New(dir, testConfig())

	if err := adapter.RemoveBlocker(); err != nil {
		t.Errorf("RemoveBlocker() with no blocker should be idempotent, got = %v", err)
	}
}

// TestBlockerAndLockIndependent verifies blocker and lock operate independently.
func TestBlockerAndLockIndependent(t *testing.T) {
	dir := t.TempDir()
	adapter := New(dir, testConfig())

	if err := adapter.AcquireLock(); err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}
	defer adapter.ReleaseLock()

	// Blocker should still be absent
	if adapter.HasBlocker() {
		t.Error("AcquireLock() should not create a blocker")
	}

	// Create blocker while lock is held
	if err := adapter.CreateBlocker(); err != nil {
		t.Fatalf("CreateBlocker() while locked error = %v", err)
	}

	// Both should be active simultaneously
	if !adapter.HasBlocker() {
		t.Error("HasBlocker() should be true while lock is held")
	}
}

// TestFullWorkflow exercises the complete commit plan lifecycle.
func TestFullWorkflow(t *testing.T) {
	dir := t.TempDir()
	adapter := New(dir, testConfig())

	// 1. Acquire lock
	if err := adapter.AcquireLock(); err != nil {
		t.Fatalf("step1 AcquireLock() error = %v", err)
	}

	// 2. Write plan
	plan := domain.OperationPlan{
		Operation: "commit",
		Preview:   "feat: full workflow",
		Messages:  []string{"feat: full workflow"},
	}
	if err := adapter.WritePlan(plan); err != nil {
		adapter.ReleaseLock()
		t.Fatalf("step2 WritePlan() error = %v", err)
	}

	// 3. Create blocker (awaiting user approval)
	if err := adapter.CreateBlocker(); err != nil {
		adapter.ReleaseLock()
		t.Fatalf("step3 CreateBlocker() error = %v", err)
	}

	// 4. Read plan back (simulating APPLY)
	got, err := adapter.ReadPlan()
	if err != nil || got == nil {
		adapter.ReleaseLock()
		t.Fatalf("step4 ReadPlan() error = %v, plan = %v", err, got)
	}

	// 5. Execute and clean up
	if err := adapter.DeletePlan(); err != nil {
		t.Errorf("step5 DeletePlan() error = %v", err)
	}
	if err := adapter.ReleaseLock(); err != nil {
		t.Errorf("step5 ReleaseLock() error = %v", err)
	}

	// State should be clean
	if adapter.HasBlocker() {
		t.Error("HasBlocker() should be false at end of workflow")
	}
	finalPlan, _ := adapter.ReadPlan()
	if finalPlan != nil {
		t.Error("ReadPlan() should be nil at end of workflow")
	}
}
