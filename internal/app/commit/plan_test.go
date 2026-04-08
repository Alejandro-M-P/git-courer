package commit

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Alejandro-M-P/git-courer/internal/infra/config"
)

func TestPlanStore_WritePlanAndReadPlan(t *testing.T) {
	// Create temp dir for test
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Git: config.GitConfig{
			WorkDir: tmpDir,
		},
		Commit: config.CommitConfig{
			PlanFile: "test_plan.json",
		},
	}

	store := newPlanStore(cfg)

	t.Run("WritePlan and ReadPlan roundtrip", func(t *testing.T) {
		plan := Plan{
			Message:    "test commit message",
			SubCommand: "commit",
			Preview:    false,
			TTL:        600,
			CreatedAt:  time.Now(),
			Commits:    1,
		}

		// Write plan
		if err := store.WritePlan(plan); err != nil {
			t.Fatalf("WritePlan() error = %v", err)
		}

		// Read plan
		readPlan, err := store.ReadPlan()
		if err != nil {
			t.Fatalf("ReadPlan() error = %v", err)
		}

		if readPlan == nil {
			t.Fatal("ReadPlan() returned nil")
		}

		if readPlan.Message != plan.Message {
			t.Errorf("Message = %v, want %v", readPlan.Message, plan.Message)
		}
		if readPlan.SubCommand != plan.SubCommand {
			t.Errorf("SubCommand = %v, want %v", readPlan.SubCommand, plan.SubCommand)
		}
		if readPlan.Preview != plan.Preview {
			t.Errorf("Preview = %v, want %v", readPlan.Preview, plan.Preview)
		}
		if readPlan.TTL != plan.TTL {
			t.Errorf("TTL = %v, want %v", readPlan.TTL, plan.TTL)
		}
		if readPlan.Commits != plan.Commits {
			t.Errorf("Commits = %v, want %v", readPlan.Commits, plan.Commits)
		}
	})

	t.Run("ReadPlan returns nil when no plan exists", func(t *testing.T) {
		// Use a different path that doesn't have a plan
		cfg2 := &config.Config{
			Git: config.GitConfig{
				WorkDir: tmpDir,
			},
			Commit: config.CommitConfig{
				PlanFile: "nonexistent_plan.json",
			},
		}
		store2 := newPlanStore(cfg2)

		plan, err := store2.ReadPlan()
		if err != nil {
			t.Fatalf("ReadPlan() error = %v", err)
		}
		if plan != nil {
			t.Error("ReadPlan() should return nil when no plan exists")
		}
	})
}

func TestIsPlanExpired(t *testing.T) {
	t.Run("plan with no TTL uses default", func(t *testing.T) {
		plan := Plan{
			Message:    "test",
			SubCommand: "commit",
			Preview:    false,
			TTL:        0,                                 // No TTL set
			CreatedAt:  time.Now().Add(-11 * time.Minute), // Older than default 10 min
		}

		if !IsPlanExpired(plan) {
			t.Error("IsPlanExpired() should return true for plan older than DefaultTTL")
		}
	})

	t.Run("plan within TTL is not expired", func(t *testing.T) {
		plan := Plan{
			Message:    "test",
			SubCommand: "commit",
			Preview:    false,
			TTL:        0,                                // No TTL set, use default
			CreatedAt:  time.Now().Add(-5 * time.Minute), // Within default 10 min
		}

		if IsPlanExpired(plan) {
			t.Error("IsPlanExpired() should return false for plan within DefaultTTL")
		}
	})

	t.Run("plan with custom TTL", func(t *testing.T) {
		plan := Plan{
			Message:    "test",
			SubCommand: "commit",
			Preview:    false,
			TTL:        60,                               // 60 seconds
			CreatedAt:  time.Now().Add(-2 * time.Minute), // Older than 60 seconds
		}

		if !IsPlanExpired(plan) {
			t.Error("IsPlanExpired() should return true for plan older than custom TTL")
		}
	})

	t.Run("plan within custom TTL is not expired", func(t *testing.T) {
		plan := Plan{
			Message:    "test",
			SubCommand: "commit",
			Preview:    false,
			TTL:        600,                              // 600 seconds (10 minutes)
			CreatedAt:  time.Now().Add(-5 * time.Minute), // Within TTL
		}

		if IsPlanExpired(plan) {
			t.Error("IsPlanExpired() should return false for plan within custom TTL")
		}
	})
}

func TestPlanStore_DeletePlan(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Git: config.GitConfig{
			WorkDir: tmpDir,
		},
		Commit: config.CommitConfig{
			PlanFile: "test_plan.json",
		},
	}

	store := newPlanStore(cfg)

	t.Run("DeletePlan removes file", func(t *testing.T) {
		// First create a plan
		plan := Plan{
			Message:   "test",
			CreatedAt: time.Now(),
		}
		if err := store.WritePlan(plan); err != nil {
			t.Fatalf("WritePlan() error = %v", err)
		}

		// Verify file exists
		if !store.PlanFileExists() {
			t.Error("PlanFileExists() should return true after WritePlan")
		}

		// Delete plan
		if err := store.DeletePlan(); err != nil {
			t.Fatalf("DeletePlan() error = %v", err)
		}

		// Verify file no longer exists
		if store.PlanFileExists() {
			t.Error("PlanFileExists() should return false after DeletePlan")
		}
	})

	t.Run("DeletePlan is idempotent", func(t *testing.T) {
		// Deleting non-existent plan should not error
		if err := store.DeletePlan(); err != nil {
			t.Errorf("DeletePlan() on non-existent file error = %v", err)
		}
	})
}

func TestPlanStore_PlanFileExists(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Git: config.GitConfig{
			WorkDir: tmpDir,
		},
		Commit: config.CommitConfig{
			PlanFile: "test_plan.json",
		},
	}

	store := newPlanStore(cfg)

	t.Run("returns false when no plan exists", func(t *testing.T) {
		if store.PlanFileExists() {
			t.Error("PlanFileExists() should return false when no plan exists")
		}
	})

	t.Run("returns true after WritePlan", func(t *testing.T) {
		plan := Plan{
			Message:   "test",
			CreatedAt: time.Now(),
		}
		if err := store.WritePlan(plan); err != nil {
			t.Fatalf("WritePlan() error = %v", err)
		}

		if !store.PlanFileExists() {
			t.Error("PlanFileExists() should return true after WritePlan")
		}
	})
}

func TestPlanStore_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Git: config.GitConfig{
			WorkDir: tmpDir,
		},
		Commit: config.CommitConfig{
			PlanFile: "atomic_plan.json",
		},
	}

	store := newPlanStore(cfg)

	t.Run("write creates temp file then renames", func(t *testing.T) {
		plan := Plan{
			Message:    "atomic test",
			SubCommand: "commit",
			Preview:    true,
			CreatedAt:  time.Now(),
		}

		if err := store.WritePlan(plan); err != nil {
			t.Fatalf("WritePlan() error = %v", err)
		}

		// Verify final file exists
		if _, err := os.Stat(store.planFilePath); os.IsNotExist(err) {
			t.Error("Final plan file should exist")
		}

		// Verify temp file does not exist
		tmpPath := store.planFilePath + ".tmp"
		if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
			t.Error("Temp file should not exist after successful write")
		}
	})

	t.Run("WritePlan creates directory if missing", func(t *testing.T) {
		nestedCfg := &config.Config{
			Git: config.GitConfig{
				WorkDir: tmpDir,
			},
			Commit: config.CommitConfig{
				PlanFile: filepath.Join("subdir", "nested", "plan.json"),
			},
		}
		nestedStore := newPlanStore(nestedCfg)

		plan := Plan{
			Message:   "nested test",
			CreatedAt: time.Now(),
		}

		if err := nestedStore.WritePlan(plan); err != nil {
			t.Fatalf("WritePlan() should create nested directories, error = %v", err)
		}

		if !nestedStore.PlanFileExists() {
			t.Error("PlanFileExists() should return true for nested path")
		}
	})
}
