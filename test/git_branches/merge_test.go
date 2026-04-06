package git_branches

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestMerge_FastForward(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	exec.Command("git", "checkout", "-b", "feature/ff").Run()

	os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main\n"), 0644)
	exec.Command("git", "add", "new.go").Run()
	exec.Command("git", "commit", "-m", "new file").Run()

	exec.Command("git", "checkout", "main").Run()

	cmd := exec.Command("git", "merge", "feature/ff", "--ff-only")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("FF merge failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "new.go")); err != nil {
		t.Error("File not merged")
	}

	t.Log("✅ Fast-forward merge successful")
}

func TestMerge_NoFastForward(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	exec.Command("git", "checkout", "-b", "feature/noff").Run()
	os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package main\n"), 0644)
	exec.Command("git", "add", "feature.go").Run()
	exec.Command("git", "commit", "-m", "feature").Run()

	exec.Command("git", "checkout", "main").Run()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)
	exec.Command("git", "add", "main.go").Run()
	exec.Command("git", "commit", "-m", "main change").Run()

	cmd := exec.Command("git", "merge", "feature/noff", "--no-ff", "-m", "merge feature")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("No-ff merge failed: %v", err)
	}

	for _, f := range []string{"feature.go", "main.go"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("File %s not found after merge", f)
		}
	}

	t.Log("✅ No-fast-forward merge successful")
}

func TestMerge_ConflictDetection(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	// Create branch and modify same file
	exec.Command("git", "checkout", "-b", "feature/conflict").Run()
	os.WriteFile(filepath.Join(dir, "same.go"), []byte("package main\n// feature\n"), 0644)
	exec.Command("git", "add", "same.go").Run()
	exec.Command("git", "commit", "-m", "feature change").Run()

	// Back to main, modify same file differently
	exec.Command("git", "checkout", "main").Run()
	os.WriteFile(filepath.Join(dir, "same.go"), []byte("package main\n// main\n"), 0644)
	exec.Command("git", "add", "same.go").Run()
	exec.Command("git", "commit", "-m", "main change").Run()

	// Try to merge
	cmd := exec.Command("git", "merge", "feature/conflict")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()

	if err == nil {
		t.Error("Expected merge conflict but got none")
	} else {
		t.Log("✅ Merge conflict detected as expected")
	}

	t.Logf("Merge output: %s", string(output))
}
