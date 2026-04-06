package git_branches

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLog_ViewHistory(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	for i := 0; i < 5; i++ {
		os.WriteFile(filepath.Join(dir, "file.go"), []byte("package main\n"), 0644)
		exec.Command("git", "add", "file.go").Run()
		exec.Command("git", "commit", "-m", "commit "+string(rune('0'+i))).Run()
	}

	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Log failed: %v", err)
	}

	t.Logf("Commit history:\n%s", string(output))
}

func TestLog_FilterByAuthor(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	exec.Command("git", "config", "user.email", "other@test.com").Run()
	exec.Command("git", "config", "user.name", "Other").Run()

	os.WriteFile(filepath.Join(dir, "other.go"), []byte("package main\n"), 0644)
	exec.Command("git", "add", "other.go").Run()
	exec.Command("git", "commit", "-m", "other author commit").Run()

	cmd := exec.Command("git", "log", "--author", "Other", "--oneline")
	cmd.Dir = dir
	output, _ := cmd.Output()

	if !strings.Contains(string(output), "other author commit") {
		t.Error("Author filter not working")
	}

	t.Log("✅ Author filter works")
}
