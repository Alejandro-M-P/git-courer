package commit

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/core/domain"
	"github.com/Alejandro-M-P/git-courer/internal/core/ports"
	"github.com/Alejandro-M-P/git-courer/internal/infra/config"
)

// mockGit is a mock implementation of ports.Git for testing
type mockGit struct {
	resetSoftCalls int
}

func (m *mockGit) Status() (domain.Status, error) {
	return domain.Status{}, nil
}
func (m *mockGit) Diff() (string, error)                            { return "", nil }
func (m *mockGit) DiffStaged() (string, error)                      { return "", nil }
func (m *mockGit) Add(paths []string) error                         { return nil }
func (m *mockGit) Commit(message string) (string, error)            { return "", nil }
func (m *mockGit) Push() (string, error)                            { return "", nil }
func (m *mockGit) Pull() (string, error)                            { return "", nil }
func (m *mockGit) PullRebase() (string, error)                      { return "", nil }
func (m *mockGit) Fetch() (string, error)                           { return "", nil }
func (m *mockGit) Branch(name string) (string, error)               { return "", nil }
func (m *mockGit) Checkout(name string) (string, error)             { return "", nil }
func (m *mockGit) CurrentBranch() (string, error)                   { return "", nil }
func (m *mockGit) Merge(branch string) (string, error)              { return "", nil }
func (m *mockGit) Rebase(branch string) (string, error)             { return "", nil }
func (m *mockGit) Stash() (string, error)                           { return "", nil }
func (m *mockGit) StashPop() (string, error)                        { return "", nil }
func (m *mockGit) Reset(mode string, commit string) (string, error) { return "", nil }
func (m *mockGit) ResetSoft(commits int) error {
	m.resetSoftCalls = commits
	return nil
}
func (m *mockGit) Log(limit int) (string, error)                  { return "", nil }
func (m *mockGit) Show(commit string) (string, error)             { return "", nil }
func (m *mockGit) Blame(file string) (string, error)              { return "", nil }
func (m *mockGit) Clean(directories bool) (string, error)         { return "", nil }
func (m *mockGit) Revert(commit string) (string, error)           { return "", nil }
func (m *mockGit) CherryPick(commit string) (string, error)       { return "", nil }
func (m *mockGit) Tag(name string) (string, error)                { return "", nil }
func (m *mockGit) DeleteBranch(name string) (string, error)       { return "", nil }
func (m *mockGit) Reflog(limit int) (string, error)               { return "", nil }
func (m *mockGit) PushWithUpstream(branch string) (string, error) { return "", nil }
func (m *mockGit) IsRepo() bool                                   { return true }

// Ensure mockGit implements ports.Git
var _ ports.Git = (*mockGit)(nil)

func TestBlockingManager_AcquireLockAndReleaseLock(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Git: config.GitConfig{
			WorkDir: tmpDir,
		},
		Commit: config.CommitConfig{
			LockFile: "test.lock",
			PlanFile: "test_plan.json",
		},
	}

	git := &mockGit{}
	bm := NewBlockingManager(git, cfg)

	t.Run("AcquireLock succeeds first time", func(t *testing.T) {
		if err := bm.AcquireLock(); err != nil {
			t.Fatalf("AcquireLock() error = %v", err)
		}
	})

	t.Run("AcquireLock fails when already locked", func(t *testing.T) {
		// bm already holds lock from previous test
		if err := bm.AcquireLock(); err == nil {
			t.Error("AcquireLock() should fail when lock already held")
		}
	})

	t.Run("ReleaseLock releases the lock", func(t *testing.T) {
		// Release the lock
		if err := bm.ReleaseLock(); err != nil {
			t.Fatalf("ReleaseLock() error = %v", err)
		}

		// Verify we can acquire again
		if err := bm.AcquireLock(); err != nil {
			t.Fatalf("AcquireLock() after ReleaseLock() error = %v", err)
		}

		// Clean up
		bm.ReleaseLock()
	})

	t.Run("ReleaseLock is idempotent", func(t *testing.T) {
		// Release when not locked should not error
		if err := bm.ReleaseLock(); err != nil {
			t.Fatalf("ReleaseLock() when not locked error = %v", err)
		}
	})
}

func TestBlockingManager_WaitForConfirmationApproveAbort(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Git: config.GitConfig{
			WorkDir: tmpDir,
		},
		Commit: config.CommitConfig{
			LockFile: "test.lock",
			PlanFile: "test_plan.json",
		},
	}

	t.Run("Approve wakes up WaitForConfirmation", func(t *testing.T) {
		git := &mockGit{}
		bm := NewBlockingManager(git, cfg)

		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()
			state := bm.WaitForConfirmation()
			if state != StateApproved {
				t.Errorf("WaitForConfirmation() = %v, want StateApproved", state)
			}
		}()

		// Give goroutine time to start waiting
		time.Sleep(10 * time.Millisecond)

		// Approve should wake up the wait
		bm.Approve()

		wg.Wait()
	})

	t.Run("Abort wakes up WaitForConfirmation", func(t *testing.T) {
		git := &mockGit{}
		bm := NewBlockingManager(git, cfg)

		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()
			state := bm.WaitForConfirmation()
			if state != StateAborted {
				t.Errorf("WaitForConfirmation() = %v, want StateAborted", state)
			}
		}()

		// Give goroutine time to start waiting
		time.Sleep(10 * time.Millisecond)

		// Abort should wake up the wait
		bm.Abort()

		wg.Wait()
	})

	t.Run("Abort cleans up persisted plan", func(t *testing.T) {
		git := &mockGit{}
		bm := NewBlockingManager(git, cfg)

		// Set a plan
		plan := &Plan{
			Message:   "test plan",
			CreatedAt: time.Now(),
		}
		if err := bm.SetPlan(plan); err != nil {
			t.Fatalf("SetPlan() error = %v", err)
		}

		// Verify plan file exists via planStore
		if !bm.planStore.PlanFileExists() {
			t.Error("Plan file should exist after SetPlan")
		}

		// Abort should delete the plan
		bm.Abort()

		if bm.planStore.PlanFileExists() {
			t.Error("Plan file should not exist after Abort")
		}
	})

	t.Run("Approve then Abort is no-op after first call", func(t *testing.T) {
		git := &mockGit{}
		bm := NewBlockingManager(git, cfg)

		bm.Approve()
		bm.Abort() // Should be no-op

		if bm.GetState() != StateApproved {
			t.Error("State should remain StateApproved after both Approve and Abort")
		}
	})

	t.Run("Abort then Approve is no-op after first call", func(t *testing.T) {
		git := &mockGit{}
		bm := NewBlockingManager(git, cfg)

		bm.Abort()
		bm.Approve() // Should be no-op

		if bm.GetState() != StateAborted {
			t.Error("State should remain StateAborted after both Abort and Approve")
		}
	})
}

func TestBlockingManager_GetState(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Git: config.GitConfig{
			WorkDir: tmpDir,
		},
		Commit: config.CommitConfig{
			LockFile: "test.lock",
			PlanFile: "test_plan.json",
		},
	}

	t.Run("initial state is pending", func(t *testing.T) {
		git := &mockGit{}
		bm := NewBlockingManager(git, cfg)

		if bm.GetState() != StatePending {
			t.Errorf("Initial state = %v, want StatePending", bm.GetState())
		}
	})

	t.Run("state after Approve is approved", func(t *testing.T) {
		git := &mockGit{}
		bm := NewBlockingManager(git, cfg)

		bm.Approve()

		if bm.GetState() != StateApproved {
			t.Errorf("State after Approve = %v, want StateApproved", bm.GetState())
		}
	})

	t.Run("state after Abort is aborted", func(t *testing.T) {
		git := &mockGit{}
		bm := NewBlockingManager(git, cfg)

		bm.Abort()

		if bm.GetState() != StateAborted {
			t.Errorf("State after Abort = %v, want StateAborted", bm.GetState())
		}
	})
}

func TestBlockingManager_GetPlan(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Git: config.GitConfig{
			WorkDir: tmpDir,
		},
		Commit: config.CommitConfig{
			LockFile: "test.lock",
			PlanFile: "test_plan.json",
		},
	}

	t.Run("GetPlan returns nil initially", func(t *testing.T) {
		git := &mockGit{}
		bm := NewBlockingManager(git, cfg)

		if bm.GetPlan() != nil {
			t.Error("GetPlan() should return nil when no plan is set")
		}
	})

	t.Run("GetPlan returns set plan", func(t *testing.T) {
		git := &mockGit{}
		bm := NewBlockingManager(git, cfg)

		expectedPlan := &Plan{
			Message:   "test message",
			CreatedAt: time.Now(),
		}

		if err := bm.SetPlan(expectedPlan); err != nil {
			t.Fatalf("SetPlan() error = %v", err)
		}

		actualPlan := bm.GetPlan()
		if actualPlan == nil {
			t.Fatal("GetPlan() should not return nil after SetPlan")
		}
		if actualPlan.Message != expectedPlan.Message {
			t.Errorf("GetPlan().Message = %v, want %v", actualPlan.Message, expectedPlan.Message)
		}
	})
}

func TestBlockingManager_IsLockStale(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Git: config.GitConfig{
			WorkDir: tmpDir,
		},
		Commit: config.CommitConfig{
			LockFile: "test.lock",
			PlanFile: "test_plan.json",
		},
	}

	t.Run("returns false when no lock exists", func(t *testing.T) {
		git := &mockGit{}
		bm := NewBlockingManager(git, cfg)

		isStale, err := bm.IsLockStale()
		if err != nil {
			t.Fatalf("IsLockStale() error = %v", err)
		}
		if isStale {
			t.Error("IsLockStale() should return false when no lock exists")
		}
	})

	t.Run("returns false for fresh lock", func(t *testing.T) {
		git := &mockGit{}
		bm := NewBlockingManager(git, cfg)

		if err := bm.AcquireLock(); err != nil {
			t.Fatalf("AcquireLock() error = %v", err)
		}
		defer bm.ReleaseLock()

		isStale, err := bm.IsLockStale()
		if err != nil {
			t.Fatalf("IsLockStale() error = %v", err)
		}
		if isStale {
			t.Error("IsLockStale() should return false for fresh lock")
		}
	})

	t.Run("returns true for old lock", func(t *testing.T) {
		git := &mockGit{}
		bm := NewBlockingManager(git, cfg)

		// Create a lock file with old timestamp
		dir := filepath.Dir(bm.lockFile)
		os.MkdirAll(dir, 0755)
		os.WriteFile(bm.lockFile, []byte("old lock"), 0644)

		// Manually set the mod time to the past
		oldTime := time.Now().Add(-25 * time.Hour)
		os.Chtimes(bm.lockFile, oldTime, oldTime)

		isStale, err := bm.IsLockStale()
		if err != nil {
			t.Fatalf("IsLockStale() error = %v", err)
		}
		if !isStale {
			t.Error("IsLockStale() should return true for lock older than LockTTL")
		}
	})
}

func TestBlockingManager_CleanupAndReset(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Git: config.GitConfig{
			WorkDir: tmpDir,
		},
		Commit: config.CommitConfig{
			LockFile: "test.lock",
			PlanFile: "test_plan.json",
		},
	}

	t.Run("cleanup removes stale lock", func(t *testing.T) {
		git := &mockGit{}
		bm := NewBlockingManager(git, cfg)

		// Create a stale lock
		dir := filepath.Dir(bm.lockFile)
		os.MkdirAll(dir, 0755)
		os.WriteFile(bm.lockFile, []byte("stale lock"), 0644)
		oldTime := time.Now().Add(-25 * time.Hour)
		os.Chtimes(bm.lockFile, oldTime, oldTime)

		if err := bm.CleanupAndReset(); err != nil {
			t.Fatalf("CleanupAndReset() error = %v", err)
		}

		if bm.planStore.PlanFileExists() {
			t.Error("Plan file should not exist after CleanupAndReset")
		}
	})

	t.Run("cleanup handles plan with commits", func(t *testing.T) {
		git := &mockGit{}
		bm := NewBlockingManager(git, cfg)

		// Create a plan with commits
		plan := &Plan{
			Message:   "test plan",
			Commits:   2,
			CreatedAt: time.Now().Add(-15 * time.Minute), // Expired
		}
		if err := bm.SetPlan(plan); err != nil {
			t.Fatalf("SetPlan() error = %v", err)
		}

		if err := bm.CleanupAndReset(); err != nil {
			t.Fatalf("CleanupAndReset() error = %v", err)
		}

		if git.resetSoftCalls != 2 {
			t.Errorf("ResetSoft called with %d, want 2", git.resetSoftCalls)
		}
	})
}

func TestConfirmationState_String(t *testing.T) {
	tests := []struct {
		state    ConfirmationState
		expected string
	}{
		{StatePending, "pending"},
		{StateApproved, "approved"},
		{StateAborted, "aborted"},
		{ConfirmationState(999), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("ConfirmationState(%d).String() = %v, want %v", tt.state, got, tt.expected)
		}
	}
}
