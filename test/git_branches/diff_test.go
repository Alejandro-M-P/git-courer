package git_branches

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiff_WorkingTree(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Modified\n"), 0644)

	cmd := exec.Command("git", "diff")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to diff: %v", err)
	}

	if !strings.Contains(string(output), "Modified") {
		t.Error("Diff doesn't show modification")
	}

	t.Log("✅ Working tree diff works")
}

func TestDiff_Staged(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Staged change\n"), 0644)
	cmd := exec.Command("git", "add", "README.md")
	cmd.Dir = dir
	cmd.Run()

	cmd = exec.Command("git", "diff", "--cached")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to diff staged: %v", err)
	}

	if !strings.Contains(string(output), "Staged change") {
		t.Error("Staged diff doesn't show change")
	}

	t.Log("✅ Staged diff works")
}

func TestDiff_BetweenBranches(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	exec.Command("git", "checkout", "-b", "feature/diff-test").Run()

	os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package main\n"), 0644)
	exec.Command("git", "add", "feature.go").Run()
	exec.Command("git", "commit", "-m", "add feature").Run()

	exec.Command("git", "checkout", "main").Run()

	cmd := exec.Command("git", "diff", "main..feature/diff-test", "--name-only")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to diff branches: %v", err)
	}

	if !strings.Contains(string(output), "feature.go") {
		t.Error("Branch diff doesn't show new file")
	}

	t.Log("✅ Branch diff works")
}
