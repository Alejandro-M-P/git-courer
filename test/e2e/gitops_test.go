//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestReadStatus verifies that git.Status() reflects unstaged and staged changes.
func TestReadStatus(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	llmA := requireOllama(t)
	dir, gitA := sandboxRepo(t)
	_ = llmA // status is a pure git op, LLM not involved

	writeFile(t, dir, "new.go", "package main\n")

	status, err := gitA.Status()
	if err != nil {
		t.Fatalf("Status(): %v", err)
	}

	found := false
	for _, f := range status.Files {
		if strings.Contains(f.Path, "new.go") {
			found = true
			t.Logf("file: path=%s status=%q", f.Path, f.Status)
		}
	}
	if !found {
		t.Errorf("new.go not found in status: %v", status.Files)
	}

	detail = "ok"
}

// TestReadDiff verifies that git.DiffStaged() returns the staged content of a new file.
func TestReadDiff(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	dir, gitA := sandboxRepo(t)

	writeFile(t, dir, "diff_test.go", "package main\n\nfunc DiffTest() {}\n")
	stageFile(dir, "diff_test.go")

	diff, err := gitA.DiffStaged()
	if err != nil {
		t.Fatalf("DiffStaged(): %v", err)
	}
	if diff == "" {
		t.Fatal("expected non-empty staged diff")
	}
	if !strings.Contains(diff, "diff_test.go") {
		t.Errorf("diff should mention diff_test.go, got:\n%s", diff)
	}

	detail = "ok"
	t.Logf("staged diff (%d lines)", strings.Count(diff, "\n"))
}

// TestReadLog verifies that git.Log() returns commit history.
func TestReadLog(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	_, gitA := sandboxRepo(t)

	// sandboxRepo already creates one commit
	log, err := gitA.Log(10)
	if err != nil {
		t.Fatalf("Log(): %v", err)
	}
	if log == "" {
		t.Fatal("expected non-empty log")
	}
	if !strings.Contains(log, "initial commit") {
		t.Errorf("log should contain initial commit, got:\n%s", log)
	}

	detail = "ok"
	t.Logf("log:\n%s", log)
}

// TestReadBranches verifies that git.ListBranches() returns all created branches.
func TestReadBranches(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	dir, gitA := sandboxRepo(t)

	gitExec(t, dir, "checkout", "-b", "dev")
	gitExec(t, dir, "checkout", "-b", "staging")
	gitExec(t, dir, "checkout", "-")

	branches, err := gitA.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches(): %v", err)
	}
	if !strings.Contains(branches, "dev") {
		t.Errorf("expected 'dev' in branches output:\n%s", branches)
	}
	if !strings.Contains(branches, "staging") {
		t.Errorf("expected 'staging' in branches output:\n%s", branches)
	}

	detail = "ok"
	t.Logf("branches:\n%s", branches)
}

// TestReadBranchesWithPattern verifies that ListBranches filters by pattern.
func TestReadBranchesWithPattern(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	dir, gitA := sandboxRepo(t)

	gitExec(t, dir, "checkout", "-b", "feat/login")
	gitExec(t, dir, "checkout", "-b", "feat/signup")
	gitExec(t, dir, "checkout", "-b", "fix/bug-123")
	gitExec(t, dir, "checkout", "-")

	branches, err := gitA.ListBranches("feat/*")
	if err != nil {
		t.Fatalf("ListBranches(feat/*): %v", err)
	}

	if !strings.Contains(branches, "feat/login") || !strings.Contains(branches, "feat/signup") {
		t.Errorf("expected feat/* branches in output:\n%s", branches)
	}
	if strings.Contains(branches, "fix/bug-123") {
		t.Errorf("fix/bug-123 should NOT appear in feat/* filter:\n%s", branches)
	}

	detail = "ok"
	t.Logf("feat/* branches:\n%s", branches)
}

// TestStashAndPop modifies a file, stashes the changes, verifies the working tree
// is clean, then pops and verifies the changes are restored.
func TestStashAndPop(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	dir, gitA := sandboxRepo(t)

	// Commit an initial file so stash has a base
	writeFile(t, dir, "stash_test.go", "package main\n\nfunc original() {}\n")
	stageFile(dir, "stash_test.go")
	gitExec(t, dir, "commit", "-m", "chore: add stash test file")

	// Modify it
	writeFile(t, dir, "stash_test.go", "package main\n\nfunc modified() {}\n")

	// Verify dirty state
	if gitStatusShort(dir) == "" {
		t.Fatal("expected dirty working tree before stash")
	}

	// Stash
	_, err := gitA.Stash()
	if err != nil {
		t.Fatalf("Stash(): %v", err)
	}

	// Verify clean state
	if gitStatusShort(dir) != "" {
		t.Errorf("expected clean working tree after stash, got: %q", gitStatusShort(dir))
	}

	// Verify file content reverted
	content, _ := os.ReadFile(filepath.Join(dir, "stash_test.go"))
	if strings.Contains(string(content), "modified") {
		t.Error("expected original content after stash, got modified")
	}

	// Pop
	_, err = gitA.StashPop()
	if err != nil {
		t.Fatalf("StashPop(): %v", err)
	}

	// Verify changes restored
	contentAfter, _ := os.ReadFile(filepath.Join(dir, "stash_test.go"))
	if !strings.Contains(string(contentAfter), "modified") {
		t.Errorf("expected modified content after stash pop, got: %s", contentAfter)
	}

	detail = "stash+pop ok"
	t.Logf("stash/pop OK — content restored: %s", strings.TrimSpace(string(contentAfter)))
}

// TestResetSoft performs a soft reset: HEAD moves back but staged changes are preserved.
func TestResetSoft(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	dir, gitA := sandboxRepo(t)

	// Create a commit to reset
	writeFile(t, dir, "reset_test.go", "package main\n\nfunc Resettable() {}\n")
	stageFile(dir, "reset_test.go")
	gitExec(t, dir, "commit", "-m", "feat: add resettable function")

	logsBefore := gitLog(dir)
	if len(logsBefore) < 2 {
		t.Fatalf("expected ≥2 commits before reset, got %d", len(logsBefore))
	}

	// Soft reset to HEAD~1
	_, err := gitA.Reset("--soft", "HEAD~1")
	if err != nil {
		t.Fatalf("Reset(soft, HEAD~1): %v", err)
	}

	// HEAD should have moved back
	logsAfter := gitLog(dir)
	if len(logsAfter) >= len(logsBefore) {
		t.Errorf("expected fewer commits after soft reset: before=%d after=%d", len(logsBefore), len(logsAfter))
	}

	// Changes should still be staged (soft reset preserves index)
	staged, _ := gitA.DiffStaged()
	if staged == "" {
		t.Error("expected staged changes after soft reset — soft keeps index")
	}

	detail = "soft reset ok"
	t.Logf("commits: before=%d after=%d staged=%v", len(logsBefore), len(logsAfter), staged != "")
}

// TestResetHard performs a hard reset: HEAD moves and working tree is cleaned.
func TestResetHard(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	dir, gitA := sandboxRepo(t)

	writeFile(t, dir, "hard_reset.go", "package main\n\nfunc HardReset() {}\n")
	stageFile(dir, "hard_reset.go")
	gitExec(t, dir, "commit", "-m", "feat: add hard reset target")

	logsBefore := gitLog(dir)

	_, err := gitA.Reset("--hard", "HEAD~1")
	if err != nil {
		t.Fatalf("Reset(hard, HEAD~1): %v", err)
	}

	logsAfter := gitLog(dir)
	if len(logsAfter) >= len(logsBefore) {
		t.Errorf("expected fewer commits after hard reset: before=%d after=%d", len(logsBefore), len(logsAfter))
	}

	// Working tree should be clean
	if gitStatusShort(dir) != "" {
		t.Errorf("expected clean working tree after hard reset, got: %q", gitStatusShort(dir))
	}

	detail = "hard reset ok"
	t.Logf("commits: before=%d after=%d status=%q", len(logsBefore), len(logsAfter), gitStatusShort(dir))
}

// TestCheckoutFile verifies that checking out a file reverts unstaged changes.
func TestCheckoutFile(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	dir, gitA := sandboxRepo(t)

	// Commit a file
	writeFile(t, dir, "revertable.go", "package main\n\nfunc Original() {}\n")
	stageFile(dir, "revertable.go")
	gitExec(t, dir, "commit", "-m", "chore: add revertable file")

	// Modify it without staging
	writeFile(t, dir, "revertable.go", "package main\n\nfunc Modified() {}\n")

	if gitStatusShort(dir) == "" {
		t.Fatal("expected dirty file before checkout")
	}

	// Checkout to revert
	_, err := gitA.Checkout("revertable.go")
	if err != nil {
		t.Fatalf("Checkout(revertable.go): %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "revertable.go"))
	if !strings.Contains(string(content), "Original") {
		t.Errorf("expected original content after checkout, got: %s", content)
	}

	detail = "checkout ok"
	t.Logf("file content after checkout:\n%s", strings.TrimSpace(string(content)))
}

// TestCurrentBranch verifies CurrentBranch() returns the right name after switching.
func TestCurrentBranch(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	dir, gitA := sandboxRepo(t)

	gitExec(t, dir, "checkout", "-b", "my-feature")

	branch, err := gitA.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch(): %v", err)
	}
	if branch != "my-feature" {
		t.Errorf("expected my-feature, got %q", branch)
	}

	detail = branch
	t.Logf("current branch: %s", branch)
}

// TestListTags verifies that ListTags() returns all created tags.
func TestListTags(t *testing.T) {
	start := time.Now()
	var detail string
	defer func() { recordResult(t.Name(), !t.Failed(), time.Since(start), detail) }()

	dir, gitA := sandboxRepo(t)

	gitExec(t, dir, "tag", "v1.0.0")
	gitExec(t, dir, "tag", "v1.1.0")
	gitExec(t, dir, "tag", "v2.0.0")

	tags, err := gitA.ListTags()
	if err != nil {
		t.Fatalf("ListTags(): %v", err)
	}

	for _, want := range []string{"v1.0.0", "v1.1.0", "v2.0.0"} {
		found := false
		for _, got := range tags {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected tag %s in %v", want, tags)
		}
	}

	detail = "ok"
	t.Logf("tags: %v", tags)
}
