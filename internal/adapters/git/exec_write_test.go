package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestExecAdapterAdd tests adding files to staging.
func TestExecAdapterAdd(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create a file
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("test"), 0644)

	adapter := New(dir)
	if err := adapter.Add([]string{"test.txt"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Verify file is staged
	status, _ := adapter.Status()
	if len(status.Files) == 0 {
		t.Error("Add() should have staged file")
	}
}

// TestExecAdapterBranch tests branch operations.
func TestExecAdapterBranch(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Make initial commit first
	os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("initial"), 0644)
	adapter := New(dir)
	adapter.Add([]string{"initial.txt"})
	adapter.Commit("Initial commit")

	// Create a new branch — should NOT switch
	_, err := adapter.Branch("test-branch")
	if err != nil {
		t.Fatalf("Branch() error = %v", err)
	}

	// Current branch should still be "main" (NOT test-branch)
	current, _ := adapter.CurrentBranch()
	if current != "main" && current != "master" {
		t.Errorf("Branch() should NOT switch branch, got %q", current)
	}

	// Switch to the branch
	if err := adapter.Switch("test-branch"); err != nil {
		t.Fatalf("Switch() error = %v", err)
	}

	current, _ = adapter.CurrentBranch()
	if current != "test-branch" {
		t.Errorf("CurrentBranch() = %q, want test-branch", current)
	}

	// Clean up - switch back and delete branch
	adapter.Switch("main")
	if currentBranch, _ := adapter.CurrentBranch(); currentBranch == "" {
		adapter.Switch("master")
	}
	adapter.DeleteBranch("test-branch", false)
}

// TestExecAdapterCommit tests commit operation.
func TestExecAdapterCommit(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Create and stage a file
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("test"), 0644)
	adapter.Add([]string{"test.txt"})

	// Commit
	output, err := adapter.Commit("Test commit message")
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	if output == "" {
		t.Logf("Commit returned: %s", output)
	}
}

// TestRemove verifies a tracked file can be removed via git rm.
func TestRemove(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Add and commit a file so it is tracked
	os.WriteFile(filepath.Join(dir, "todelete.txt"), []byte("bye"), 0644)
	adapter := New(dir)
	adapter.Add([]string{"todelete.txt"})
	adapter.Commit("add file to delete")

	// git rm the tracked file
	if err := adapter.Remove([]string{"todelete.txt"}); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
}

// TestRemoveEmpty verifies Remove with empty slice is a no-op.
func TestRemoveEmpty(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)
	if err := adapter.Remove([]string{}); err != nil {
		t.Errorf("Remove(nil) should be a no-op, got = %v", err)
	}
}

// TestStashAndPop verifies stash / stash pop lifecycle.
func TestStashAndPop(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// We need a tracked file with uncommitted changes to stash
	os.WriteFile(filepath.Join(dir, "stashme.txt"), []byte("original"), 0644)
	adapter := New(dir)
	adapter.Add([]string{"stashme.txt"})
	adapter.Commit("initial")

	// Modify the file — this change will be stashed
	os.WriteFile(filepath.Join(dir, "stashme.txt"), []byte("modified"), 0644)

	out, err := adapter.Stash()
	if err != nil {
		t.Fatalf("Stash() error = %v", err)
	}
	_ = out // output may be empty or a message

	// Verify the working tree is clean after stash
	status, _ := adapter.Status()
	if !status.IsClean {
		t.Error("Stash() should leave working tree clean")
	}

	// Pop the stash
	out, err = adapter.StashPop()
	if err != nil {
		t.Fatalf("StashPop() error = %v", err)
	}
	_ = out

	// File should be back to modified
	status, _ = adapter.Status()
	if status.IsClean {
		t.Error("StashPop() should restore modified file")
	}
}

// TestReset verifies soft reset moves HEAD back one commit.
func TestReset(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// First commit
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("first"), 0644)
	adapter.Add([]string{"file1.txt"})
	adapter.Commit("first commit")

	// Second commit
	os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("second"), 0644)
	adapter.Add([]string{"file2.txt"})
	adapter.Commit("second commit")

	// Soft reset to HEAD~1
	out, err := adapter.Reset("--soft", "HEAD~1")
	if err != nil {
		t.Fatalf("Reset(--soft, HEAD~1) error = %v", err)
	}
	_ = out

	// file2.txt should be staged again after soft reset
	status, _ := adapter.Status()
	if status.Staged == 0 {
		t.Error("After soft reset, the last commit's changes should be staged")
	}
}

// TestMerge verifies merging a branch with a commit into the current branch.
func TestMerge(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Initial commit on the default branch
	os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base"), 0644)
	adapter.Add([]string{"base.txt"})
	adapter.Commit("base commit")

	// Create and switch to feature branch
	adapter.Branch("feature-branch")
	adapter.Switch("feature-branch")

	// Commit on feature branch
	os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature"), 0644)
	adapter.Add([]string{"feature.txt"})
	adapter.Commit("feature commit")

	// Switch back to default branch
	defaultBranch, _ := adapter.runGit("rev-parse", "--abbrev-ref", "HEAD")
	// We need to go back to main/master
	adapter.Switch("main")
	if defaultBranch == "" {
		adapter.Switch("master")
	}

	// Merge feature branch
	out, err := adapter.Merge("feature-branch")
	if err != nil {
		t.Fatalf("Merge() error = %v", err)
	}
	_ = out
}

// TestStashPopUntrackedError verifies StashPop returns friendly error when untracked conflict.
func TestStashPopUntrackedError(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Commit a file
	os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("original"), 0644)
	adapter.Add([]string{"tracked.txt"})
	adapter.Commit("initial")

	// Stash with untracked (use -u flag via git command directly)
	os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("stashme"), 0644)
	os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("modified"), 0644)
	cmd := exec.Command("git", "stash", "push", "-u", "-m", "test")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git stash push failed: %v, %s", err, out)
	}

	// Create a file with the same name as untracked to cause conflict
	os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("conflict"), 0644)

	// StashPop should fail with untracked error
	_, err := adapter.StashPop()
	if err == nil {
		t.Fatal("StashPop() should error when untracked files conflict")
	}
	if !containsStr(err.Error(), "STASH_POP_UNTRACKED") {
		t.Errorf("StashPop() error = %q, want STASH_POP_UNTRACKED", err.Error())
	}
}

// TestPullNoUpstreamError verifies Pull returns NO_UPSTREAM when no remote configured.
func TestPullNoUpstreamError(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create a file and commit
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("test"), 0644)
	adapter := New(dir)
	adapter.Add([]string{"file.txt"})
	adapter.Commit("initial")

	_, err := adapter.Pull()
	if err == nil {
		t.Fatal("Pull() should error when no upstream configured")
	}
	if !containsStr(err.Error(), "NO_UPSTREAM") {
		t.Errorf("Pull() error = %q, want NO_UPSTREAM", err.Error())
	}
}

// TestDeleteBranchForce verifies force delete uses -D flag.
func TestDeleteBranchForce(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Make initial commit
	os.WriteFile(filepath.Join(dir, "init.txt"), []byte("init"), 0644)
	adapter := New(dir)
	adapter.Add([]string{"init.txt"})
	adapter.Commit("initial")

	// Create a branch
	adapter.Branch("feature")
	adapter.Switch("feature")

	// Commit on feature
	os.WriteFile(filepath.Join(dir, "feat.txt"), []byte("feat"), 0644)
	adapter.Add([]string{"feat.txt"})
	adapter.Commit("feat commit")

	// Switch back to default branch (master or main)
	current, _ := adapter.CurrentBranch()
	if current == "feature" {
		if err := adapter.Switch("master"); err != nil {
			adapter.Switch("main")
		}
	}

	// Try non-force delete — should fail because branch has unmerged commits
	_, err := adapter.DeleteBranch("feature", false)
	if err == nil {
		t.Fatal("DeleteBranch(feature, false) should error for unmerged branch")
	}

	// Force delete should succeed
	_, err = adapter.DeleteBranch("feature", true)
	if err != nil {
		t.Fatalf("DeleteBranch(feature, true) error = %v", err)
	}

	// Verify branch is gone
	branches, _ := adapter.ListBranches()
	if containsStr(branches, "feature") {
		t.Error("DeleteBranch with force should remove unmerged branch")
	}
}
