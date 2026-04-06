package git_branches

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestStash_Basic(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	os.WriteFile(filepath.Join(dir, "stash_me.go"), []byte("package main\n"), 0644)

	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	output, _ := cmd.Output()
	if !strings.Contains(string(output), "stash_me.go") {
		t.Fatal("Change not showing")
	}

	cmd = exec.Command("git", "stash")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Stash failed: %v", err)
	}

	cmd = exec.Command("git", "stash", "list")
	cmd.Dir = dir
	output, _ = cmd.Output()
	if !strings.Contains(string(output), "stash") {
		t.Fatal("Stash not in list")
	}

	cmd = exec.Command("git", "stash", "pop")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Stash pop failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "stash_me.go")); err != nil {
		t.Error("File not restored after pop")
	}

	t.Log("✅ Stash/pop successful")
}

func TestStash_Multiple(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	for i := 0; i < 3; i++ {
		os.WriteFile(filepath.Join(dir, "stash.go"), []byte("package main\n"), 0644)
		exec.Command("git", "add", "stash.go").Run()
		exec.Command("git", "stash").Run()
	}

	cmd := exec.Command("git", "stash", "list")
	cmd.Dir = dir
	output, _ := cmd.Output()
	lines := strings.Count(string(output), "stash")

	if lines < 3 {
		t.Errorf("Expected at least 3 stashes, got %d", lines)
	}

	t.Logf("✅ Created %d stashes", lines)
}
