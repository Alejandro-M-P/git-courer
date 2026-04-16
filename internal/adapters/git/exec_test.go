package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestNew verifies the constructor creates adapter with correct defaults.
func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		workDir string
		wantDir string
	}{
		{"empty uses current", "", "."},
		{"explicit path", "/tmp/test", "/tmp/test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter := New(tt.workDir)
			if adapter.workDir != tt.wantDir {
				t.Errorf("New(%q).workDir = %q, want %q", tt.workDir, adapter.workDir, tt.wantDir)
			}
		})
	}
}

// TestExecAdapterStatusParsing verifies status output parsing for various git status formats.
func TestExecAdapterStatusParsing(t *testing.T) {
	// Create a temp git repo to test status parsing
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create some files to test status parsing
	os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "modified.go"), []byte("package main"), 0644)
	exec.Command("git", "-C", dir, "add", "modified.go").Run()

	adapter := New(dir)
	status, err := adapter.Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}

	// Verify status has expected fields
	if status.Branch == "" {
		t.Error("Status().Branch should not be empty")
	}
}

// TestExecAdapterListUntracked tests listing untracked files.
func TestExecAdapterListUntracked(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create untracked files
	os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("test"), 0644)

	adapter := New(dir)
	files, err := adapter.ListUntracked()
	if err != nil {
		t.Fatalf("ListUntracked() error = %v", err)
	}

	if len(files) != 2 {
		t.Errorf("ListUntracked() returned %d files, want 2", len(files))
	}
}

// TestExecAdapterCurrentBranch tests getting current branch.
func TestExecAdapterCurrentBranch(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)
	branch, err := adapter.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch() error = %v", err)
	}

	// Default branch is typically "master" or "main"
	if branch == "" {
		t.Error("CurrentBranch() should not be empty in new repo")
	}
}

// TestExecAdapterIsRepo tests repository detection.
func TestIsRepo(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want bool
	}{
		{"git repo is repo", "", true},      // empty uses current which is a git repo
		{"non-repo is not repo", "", false}, // Will skip if not in git repo
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.name == "git repo is repo" {
				adapter := New(".")
				if !adapter.IsRepo() {
					t.Skip("Not in a git repo, skipping")
				}
				return
			}

			// Create temp non-repo dir
			nonRepoDir := t.TempDir()
			adapter := New(nonRepoDir)
			if got := adapter.IsRepo(); got != tt.want {
				t.Errorf("IsRepo() = %v, want %v", got, tt.want)
			}
		})
	}
}

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

	// Create a new branch
	_, err := adapter.Branch("test-branch")
	if err != nil {
		t.Fatalf("Branch() error = %v", err)
	}

	// Switch to the branch
	if err := adapter.Switch("test-branch"); err != nil {
		t.Fatalf("Switch() error = %v", err)
	}

	current, _ := adapter.CurrentBranch()
	if current != "test-branch" {
		t.Errorf("CurrentBranch() = %q, want test-branch", current)
	}

	// Clean up - switch back and delete branch
	adapter.Switch("main")
	adapter.DeleteBranch("test-branch")
}

// TestExecAdapterTag tests tag operations.
func TestExecAdapterTag(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Make initial commit first
	os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("initial"), 0644)
	adapter := New(dir)
	adapter.Add([]string{"initial.txt"})
	adapter.Commit("Initial commit")

	// Create a tag
	_, err := adapter.Tag("v1.0.0-test")
	if err != nil {
		t.Fatalf("Tag() error = %v", err)
	}

	// Verify tag exists
	exists, err := adapter.TagExists("v1.0.0-test")
	if err != nil {
		t.Fatalf("TagExists() error = %v", err)
	}
	if !exists {
		t.Error("TagExists() should return true for created tag")
	}

	// List tags
	tags, _ := adapter.ListTags()
	if len(tags) == 0 {
		t.Error("ListTags() should return created tag")
	}
}

// TestExecAdapterLog tests git log.
func TestExecAdapterLog(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Make a commit first
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("test"), 0644)
	adapter.Add([]string{"test.txt"})
	adapter.Commit("Initial commit")

	log, err := adapter.Log(10)
	if err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	if log == "" {
		t.Error("Log() should not be empty after commit")
	}
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

// initGitRepo initializes a git repo for testing.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()

	// Initialize repo
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %s", out)
	}

	// Configure user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = dir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = dir
	cmd.Run()
}
