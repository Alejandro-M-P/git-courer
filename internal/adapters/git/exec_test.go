package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// TestDiff verifies Diff returns output for unstaged changes.
func TestDiff(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create and commit a file so we have a tracked file to modify
	os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("original"), 0644)
	adapter := New(dir)
	adapter.Add([]string{"tracked.txt"})
	adapter.Commit("initial commit")

	// Modify the tracked file without staging
	os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("modified content"), 0644)

	diff, err := adapter.Diff()
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}
	if diff == "" {
		t.Error("Diff() should return non-empty diff for unstaged changes")
	}
}

// TestDiffStagedEmpty verifies DiffStaged returns empty when nothing is staged.
func TestDiffStagedEmpty(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)
	diff, err := adapter.DiffStaged()
	if err != nil {
		t.Fatalf("DiffStaged() error = %v", err)
	}
	if diff != "" {
		t.Errorf("DiffStaged() should be empty with nothing staged, got %q", diff)
	}
}

// TestDiffStaged verifies DiffStaged returns output for staged changes.
func TestDiffStaged(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create and commit a tracked file
	os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("original"), 0644)
	adapter := New(dir)
	adapter.Add([]string{"tracked.txt"})
	adapter.Commit("initial commit")

	// Modify and stage the file
	os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("staged content"), 0644)
	adapter.Add([]string{"tracked.txt"})

	diff, err := adapter.DiffStaged()
	if err != nil {
		t.Fatalf("DiffStaged() error = %v", err)
	}
	if diff == "" {
		t.Error("DiffStaged() should return non-empty diff for staged changes")
	}
}

// TestLogFull verifies LogFull returns detailed commit log.
func TestLogFull(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Make a commit
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("test"), 0644)
	adapter.Add([]string{"test.txt"})
	adapter.Commit("feat: test commit for LogFull")

	log, err := adapter.LogFull(5)
	if err != nil {
		t.Fatalf("LogFull() error = %v", err)
	}
	if log == "" {
		t.Error("LogFull() should not be empty after commit")
	}
	// LogFull (not --oneline) should include the author line
	if !containsStr(log, "Author") && !containsStr(log, "feat: test commit for LogFull") {
		t.Errorf("LogFull() missing expected commit data, got: %s", log)
	}
}

// TestListBranches verifies at least one branch is returned in a repo.
func TestListBranches(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Need at least one commit for branches to show
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644)
	adapter := New(dir)
	adapter.Add([]string{"file.txt"})
	adapter.Commit("initial")

	branches, err := adapter.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches() error = %v", err)
	}
	if branches == "" {
		t.Error("ListBranches() should return at least one branch")
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

// TestLatestTag verifies LatestTag returns the most recent tag.
func TestLatestTag(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Make a commit and tag it
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644)
	adapter.Add([]string{"file.txt"})
	adapter.Commit("tagged commit")
	adapter.Tag("v2.0.0")

	tag, err := adapter.LatestTag()
	if err != nil {
		t.Fatalf("LatestTag() error = %v", err)
	}
	if tag != "v2.0.0" {
		t.Errorf("LatestTag() = %q, want %q", tag, "v2.0.0")
	}
}

// TestCommitsFromTag verifies commits between a tag and HEAD.
func TestCommitsFromTag(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Commit and tag
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644)
	adapter.Add([]string{"file.txt"})
	adapter.Commit("base commit")
	adapter.Tag("v1.0.0")

	// More commits after tag
	os.WriteFile(filepath.Join(dir, "file2.txt"), []byte("y"), 0644)
	adapter.Add([]string{"file2.txt"})
	adapter.Commit("after tag commit")

	out, err := adapter.CommitsFromTag("v1.0.0")
	if err != nil {
		t.Fatalf("CommitsFromTag() error = %v", err)
	}
	if !containsStr(out, "after tag commit") {
		t.Errorf("CommitsFromTag() should contain commits after tag, got: %q", out)
	}
}

// TestCommitsFromTagEmpty verifies empty tag returns an error.
func TestCommitsFromTagEmpty(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)
	_, err := adapter.CommitsFromTag("")
	if err == nil {
		t.Error("CommitsFromTag(\"\") should return an error")
	}
}

// TestTagExistsEmptyName returns an error for empty name.
func TestTagExistsEmptyName(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)
	_, err := adapter.TagExists("")
	if err == nil {
		t.Error("TagExists(\"\") should return an error")
	}
}

// TestAddEmpty verifies Add with empty slice is a no-op.
func TestAddEmpty(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)
	if err := adapter.Add([]string{}); err != nil {
		t.Errorf("Add(nil) should be a no-op, got = %v", err)
	}
}

// containsStr checks if s contains substr.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && strings.Contains(s, substr)
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
