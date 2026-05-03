package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

// TestExecAdapterLog tests git log.
func TestExecAdapterLog(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Make a commit first
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("test"), 0644)
	adapter.Add([]string{"test.txt"})
	adapter.Commit("Initial commit")

	log, err := adapter.Log(10, "")
	if err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	if log == "" {
		t.Error("Log() should not be empty after commit")
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

// TestLatestTag verifies LatestTag returns the most recent tag.
func TestLatestTag(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Make a commit and tag it
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0644)
	adapter.Add([]string{"file.txt"})
	adapter.Commit("tagged commit")
	adapter.Tag("v2.0.0", "")

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
	adapter.Tag("v1.0.0", "")

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

// TestBlame verifies Blame returns per-line authorship.
func TestBlame(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create and commit a file with a few lines
	content := "line1\nline2\nline3\n"
	os.WriteFile(filepath.Join(dir, "blame.txt"), []byte(content), 0644)
	adapter := New(dir)
	adapter.Add([]string{"blame.txt"})
	adapter.Commit("add blame file")

	lines, err := adapter.Blame("blame.txt")
	if err != nil {
		t.Fatalf("Blame() error = %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("Blame() returned no lines")
	}
	if lines[0].Line == 0 {
		t.Error("Blame() line number should be >= 1")
	}
	if lines[0].Author == "" {
		t.Error("Blame() author should not be empty")
	}
	if lines[0].Hash == "" {
		t.Error("Blame() hash should not be empty")
	}
}

// TestShow verifies Show returns commit details and stats.
func TestShow(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Make a commit
	os.WriteFile(filepath.Join(dir, "show.txt"), []byte("showme"), 0644)
	adapter := New(dir)
	adapter.Add([]string{"show.txt"})
	_, commitErr := adapter.Commit("feat: show test")
	if commitErr != nil {
		t.Fatalf("Commit error: %v", commitErr)
	}

	// Get HEAD hash
	hash, _ := adapter.runGit("rev-parse", "HEAD")
	hash = strings.TrimSpace(hash)

	res, err := adapter.Show(hash)
	if err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	if res.Hash != hash {
		t.Errorf("Show() hash = %q, want %q", res.Hash, hash)
	}
	if res.Author == "" {
		t.Error("Show() author should not be empty")
	}
	if res.Message == "" {
		t.Error("Show() message should not be empty")
	}
}

// TestReflog verifies Reflog returns entries in a repo with commits.
func TestReflog(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Make a commit to create reflog entries
	os.WriteFile(filepath.Join(dir, "reflog.txt"), []byte("x"), 0644)
	adapter.Add([]string{"reflog.txt"})
	adapter.Commit("reflog test")

	entries, err := adapter.Reflog()
	if err != nil {
		t.Fatalf("Reflog() error = %v", err)
	}
	if len(entries) == 0 {
		t.Error("Reflog() should return at least one entry")
	}
}

// TestStashList verifies StashList returns stashes.
func TestStashList(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Need a tracked file with changes to stash
	os.WriteFile(filepath.Join(dir, "stash.txt"), []byte("original"), 0644)
	adapter.Add([]string{"stash.txt"})
	adapter.Commit("initial for stash")

	// Modify and stash
	os.WriteFile(filepath.Join(dir, "stash.txt"), []byte("changed"), 0644)
	adapter.Stash()

	stashes, err := adapter.StashList()
	if err != nil {
		t.Fatalf("StashList() error = %v", err)
	}
	if len(stashes) != 1 {
		t.Errorf("StashList() returned %d stashes, want 1", len(stashes))
	}
}

// TestMergeBase verifies MergeBase finds common ancestor.
func TestMergeBase(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Initial commit on default branch
	os.WriteFile(filepath.Join(dir, "base.txt"), []byte("base"), 0644)
	adapter.Add([]string{"base.txt"})
	adapter.Commit("base commit")

	// Create feature branch and commit
	adapter.Branch("feature")
	adapter.Switch("feature")
	os.WriteFile(filepath.Join(dir, "feat.txt"), []byte("feat"), 0644)
	adapter.Add([]string{"feat.txt"})
	adapter.Commit("feat commit")

	// Switch back to default branch and create another commit
	defaultBranch, _ := adapter.CurrentBranch()
	if defaultBranch == "" {
		t.Fatal("could not get current branch")
	}
	adapter.Switch(defaultBranch)
	os.WriteFile(filepath.Join(dir, "other.txt"), []byte("other"), 0644)
	adapter.Add([]string{"other.txt"})
	adapter.Commit("other commit")

	base, err := adapter.MergeBase(defaultBranch, "feature")
	if err != nil {
		t.Fatalf("MergeBase() error = %v", err)
	}
	if base == "" {
		t.Error("MergeBase() should return a non-empty hash")
	}
}

// TestLogAuthorField verifies LogFull includes author information.
func TestLogAuthorField(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Make a commit
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("test"), 0644)
	adapter.Add([]string{"test.txt"})
	adapter.Commit("feat: author test commit")

	log, err := adapter.Log(10, "")
	if err != nil {
		t.Fatalf("Log() error = %v", err)
	}
	if log == "" {
		t.Fatal("Log() should not be empty after commit")
	}

	// Verify the new format contains hash, author, date, message
	parts := strings.Split(strings.TrimSpace(log), "\n")
	for _, line := range parts {
		if line == "" {
			continue
		}
		// Format: hash|author|date|message
		segments := strings.SplitN(line, "|", 4)
		if len(segments) < 4 {
			t.Errorf("Log() line %q does not match pipe-delimited format", line)
		}
		if segments[1] == "" {
			t.Error("Log() author field should not be empty")
		}
		if segments[2] == "" {
			t.Error("Log() date field should not be empty")
		}
		if segments[3] == "" {
			t.Error("Log() message field should not be empty")
		}
	}
}

// TestDiffStatStaged verifies DiffStatStaged returns different stats from unstaged.
func TestDiffStatStaged(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Create initial commit so repo has a HEAD
	os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("initial"), 0644)
	adapter.Add([]string{"initial.txt"})
	adapter.Commit("initial commit")

	// Create a tracked file and commit it
	os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("tracked"), 0644)
	adapter.Add([]string{"tracked.txt"})
	adapter.Commit("add tracked")

	// Modify the tracked file (unstaged change)
	os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("modified tracked"), 0644)

	// Stage a new file (staged addition)
	os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged"), 0644)
	adapter.Add([]string{"staged.txt"})

	// Get unstaged stats - should show modified tracked.txt
	unstagedOut, err := adapter.DiffStat()
	if err != nil {
		t.Fatalf("DiffStat() error = %v", err)
	}
	if !strings.Contains(unstagedOut, "tracked.txt") {
		t.Error("DiffStat() should contain tracked.txt (modified unstaged)")
	}

	// Get staged stats - should show staged.txt (newly staged)
	stagedOut, err := adapter.DiffStatStaged()
	if err != nil {
		t.Fatalf("DiffStatStaged() error = %v", err)
	}
	if !strings.Contains(stagedOut, "staged.txt") {
		t.Error("DiffStatStaged() should contain staged.txt (newly staged)")
	}
	// staged.txt should not appear in unstaged output
	if strings.Contains(unstagedOut, "staged.txt") {
		t.Error("DiffStat() should NOT contain staged.txt (it's staged)")
	}

	// They should be different outputs
	if stagedOut == unstagedOut {
		t.Error("DiffStatStaged() and DiffStat() should return different results")
	}
}

// TestDiffStatStagedWithPath verifies DiffStatStaged with specific path.
func TestDiffStatStagedWithPath(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Create and stage a file in subdir
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "subdir/file.txt"), []byte("content"), 0644)
	adapter.Add([]string{"subdir/file.txt"})

	// Get stats for specific path
	out, err := adapter.DiffStatStaged("subdir")
	if err != nil {
		t.Fatalf("DiffStatStaged(subdir) error = %v", err)
	}
	if !strings.Contains(out, "subdir/file.txt") && !strings.Contains(out, "file.txt") {
		t.Error("DiffStatStaged(subdir) should contain the staged file")
	}
}
