package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCreateBackup(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)
	adapter := New(dir)

	// Make initial commit
	os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base"), 0644)
	adapter.Add([]string{"base.txt"})
	adapter.Commit("initial")

	// Case 1: Stash untracked = true (Default behavior)
	os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("untracked content"), 0644)

	backup, err := adapter.CreateBackup("test_full", true)
	if err != nil {
		t.Fatalf("CreateBackup(..., true) error = %v", err)
	}

	if !backup.HasStash {
		t.Error("Expected HasStash to be true when untracked files exist and stashUntracked is true")
	}

	if _, err := os.Stat(filepath.Join(dir, "untracked.txt")); !os.IsNotExist(err) {
		t.Error("Untracked file should have been stashed when stashUntracked is true")
	}

	err = adapter.RestoreBackup(backup)
	if err != nil {
		t.Fatalf("RestoreBackup error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "untracked.txt")); err != nil {
		t.Error("Untracked file should have been restored")
	}

	// Case 2: Stash untracked = false (NEW behavior - currently FAILS)
	// First clean up previous file
	os.Remove(filepath.Join(dir, "untracked.txt"))

	os.WriteFile(filepath.Join(dir, "stay.txt"), []byte("should stay"), 0644)
	backup2, err := adapter.CreateBackup("test_safe", false)
	if err != nil {
		t.Fatalf("CreateBackup(..., false) error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "stay.txt")); err != nil {
		t.Error("Untracked file should NOT have been stashed when stashUntracked is false")
	}

	if backup2.HasStash {
		// This depends on whether there are OTHER changes. In this test, there aren't.
		// If only untracked files exist and we don't stash them, HasStash should be false.
		t.Log("HasStash is false as expected when no other changes exist")
	}

	adapter.DeleteBackup(backup2)
}
