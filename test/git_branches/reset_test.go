package git_branches

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReset_Soft(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	os.WriteFile(filepath.Join(dir, "to-reset.go"), []byte("package main\n"), 0644)
	exec.Command("git", "add", "to-reset.go").Run()
	exec.Command("git", "commit", "-m", "will reset").Run()

	cmd := exec.Command("git", "reset", "--soft", "HEAD~1")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Reset soft failed: %v", err)
	}

	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	output, _ := cmd.Output()
	if !strings.Contains(string(output), "to-reset.go") {
		t.Error("Changes not staged after soft reset")
	}

	t.Log("✅ Soft reset successful")
}

func TestReset_Hard(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	os.WriteFile(filepath.Join(dir, "deleted.go"), []byte("package main\n"), 0644)
	exec.Command("git", "add", "deleted.go").Run()
	exec.Command("git", "commit", "-m", "will hard reset").Run()

	cmd := exec.Command("git", "reset", "--hard", "HEAD~1")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Reset hard failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "deleted.go")); err == nil {
		t.Error("File still exists after hard reset")
	}

	t.Log("✅ Hard reset successful")
}

func TestRevert_Commit(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	os.WriteFile(filepath.Join(dir, "to-revert.go"), []byte("package main\n"), 0644)
	exec.Command("git", "add", "to-revert.go").Run()
	exec.Command("git", "commit", "-m", "will revert").Run()

	cmd := exec.Command("git", "revert", "HEAD", "--no-edit")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Revert failed: %v", err)
	}

	cmd = exec.Command("git", "log", "--oneline", "-2")
	cmd.Dir = dir
	output, _ := cmd.Output()
	if !strings.Contains(string(output), "Revert") {
		t.Error("Revert didn't create commit")
	}

	t.Log("✅ Revert successful")
}
