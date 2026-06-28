package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestExecAdapterCreateRef verifies atomic ref creation via git update-ref with
// empty old-oid. The ref must point at the given commit and the command must
// fail when the ref already exists (collision detection delegated to git).
func TestExecAdapterCreateRef(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create an initial commit so we have a real object SHA to point at.
	adapter := New(dir)
	writeWorktreeFile(t, dir, "file.txt", "content")
	if err := adapter.Add([]string{"file.txt"}); err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if _, err := adapter.Commit("initial"); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	commitHash, err := adapter.Head()
	if err != nil {
		t.Fatalf("Head() error = %v", err)
	}
	commitHash = strings.TrimSpace(commitHash)

	ref := "refs/heads/courer/session-test"

	// --- Happy path: ref created on empty target ---
	if err := adapter.CreateRef(ref, commitHash); err != nil {
		t.Fatalf("CreateRef() error = %v", err)
	}

	// Verify the ref exists and points at commitHash.
	showOut, err := adapter.ShowRef(ref)
	if err != nil {
		t.Fatalf("ShowRef() error = %v", err)
	}
	if !strings.Contains(showOut, commitHash) {
		t.Errorf("CreateRef() ref does not point at %s, got: %s", commitHash, showOut)
	}

	// --- Collision: same ref already exists → must fail ---
	err = adapter.CreateRef(ref, commitHash)
	if err == nil {
		t.Error("CreateRef() should fail when ref already exists (collision), got nil")
	}
}

// TestExecAdapterAddWorktree verifies that a git worktree is created at the
// requested path and bound to the given branch. The worktree directory must
// exist after the call and contain a .git file (or .git dir for older git).
func TestExecAdapterAddWorktree(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Need an initial commit and a branch to attach the worktree to.
	writeWorktreeFile(t, dir, "initial.txt", "initial")
	adapter.Add([]string{"initial.txt"})
	if _, err := adapter.Commit("initial commit"); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	commitHash, err := adapter.Head()
	if err != nil {
		t.Fatalf("Head() error = %v", err)
	}
	commitHash = strings.TrimSpace(commitHash)

	branch := "courer/session-wt-test"
	if err := adapter.CreateRef("refs/heads/"+branch, commitHash); err != nil {
		t.Fatalf("CreateRef() setup error = %v", err)
	}

	wtPath := filepath.Join(dir, "..", "git-courer-worktrees", "wt-test")
	absPath, err := filepath.Abs(wtPath)
	if err != nil {
		t.Fatalf("filepath.Abs() error = %v", err)
	}

	got, err := adapter.AddWorktree(absPath, branch)
	if err != nil {
		t.Fatalf("AddWorktree() error = %v", err)
	}

	// The returned path must match the requested worktree path.
	if got != absPath {
		t.Errorf("AddWorktree() returned path = %q, want %q", got, absPath)
	}

	// The worktree directory must exist on disk.
	info, err := os.Stat(absPath)
	if err != nil {
		t.Fatalf("worktree dir does not exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("AddWorktree() path is not a directory")
	}

	// A linked worktree has a .git file (not a directory) on git >= 2.5.
	gitEntry := filepath.Join(absPath, ".git")
	if _, err := os.Stat(gitEntry); err != nil {
		t.Errorf("worktree .git entry missing: %v", err)
	}
}

// TestExecAdapterRemoveWorktree verifies that an existing worktree is removed
// from disk when RemoveWorktree is called with --force.
func TestExecAdapterRemoveWorktree(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	writeWorktreeFile(t, dir, "initial.txt", "initial")
	adapter.Add([]string{"initial.txt"})
	if _, err := adapter.Commit("initial commit"); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	commitHash, err := adapter.Head()
	if err != nil {
		t.Fatalf("Head() error = %v", err)
	}
	commitHash = strings.TrimSpace(commitHash)

	branch := "courer/session-wt-remove-test"
	adapter.CreateRef("refs/heads/"+branch, commitHash)

	wtPath := filepath.Join(dir, "..", "git-courer-worktrees", "wt-remove")
	absPath, _ := filepath.Abs(wtPath)

	adapter.AddWorktree(absPath, branch)

	// Worktree exists now.
	if _, err := os.Stat(absPath); err != nil {
		t.Fatalf("setup: worktree dir missing: %v", err)
	}

	// --- Remove it ---
	if err := adapter.RemoveWorktree(absPath); err != nil {
		t.Fatalf("RemoveWorktree() error = %v", err)
	}

	// Worktree directory must be gone.
	if _, err := os.Stat(absPath); err == nil {
		t.Error("RemoveWorktree() dir still exists after removal")
	}
}

// writeWorktreeFile is a test helper that writes content to a file in dir.
func writeWorktreeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writeWorktreeFile(%s) error = %v", name, err)
	}
}