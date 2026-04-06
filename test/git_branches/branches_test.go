package git_branches

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func setupGitRepo(t *testing.T) string {
	tmpDir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = tmpDir
	cmd.Run()

	return tmpDir
}

func setupGitRepoWithCommit(t *testing.T) string {
	dir := setupGitRepo(t)

	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644)
	cmd := exec.Command("git", "add", "README.md")
	cmd.Dir = dir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial commit")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to commit: %v", err)
	}

	return dir
}

func addFile(dir, filename, content string) {
	os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644)
	cmd := exec.Command("git", "add", filename)
	cmd.Dir = dir
	cmd.Run()
}

func commitFile(dir, filename, msg string) {
	addFile(dir, filename, "content")
	cmd := exec.Command("git", "commit", "-m", msg)
	cmd.Dir = dir
	cmd.Run()
}

func currentBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	output, _ := cmd.Output()
	return strings.TrimSpace(string(output))
}

func branchList(dir string) string {
	cmd := exec.Command("git", "branch")
	cmd.Dir = dir
	output, _ := cmd.Output()
	return string(output)
}

// =============================================================================
// BRANCH OPERATION TESTS
// =============================================================================

func TestBranch_CreateAndSwitch(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	cmd := exec.Command("git", "checkout", "-b", "feature/test")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to create branch: %v", err)
	}

	if !strings.Contains(branchList(dir), "feature/test") {
		t.Error("Branch not created")
	}

	if currentBranch(dir) != "feature/test" {
		t.Errorf("Not on expected branch")
	}

	t.Log("✅ Created and switched to branch: feature/test")
}

func TestBranch_ListAll(t *testing.T) {
	dir := setupGitRepoWithCommit(t)
	_ = dir // dir is used for side effects (creating the repo)

	for _, name := range []string{"feature/a", "feature/b", "fix/c"} {
		exec.Command("git", "checkout", "-b", name).Run()
		exec.Command("git", "checkout", "main").Run()
	}

	output, err := exec.Command("git", "branch", "-a").Output()
	if err != nil {
		t.Fatalf("Failed to list branches: %v", err)
	}

	t.Logf("Branches:\n%s", string(output))

	for _, name := range []string{"feature/a", "feature/b", "fix/c"} {
		if !strings.Contains(string(output), name) {
			t.Errorf("Missing branch: %s", name)
		}
	}
}

func TestBranch_Delete(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	exec.Command("git", "checkout", "-b", "to-delete").Run()
	exec.Command("git", "checkout", "main").Run()

	cmd := exec.Command("git", "branch", "-d", "to-delete")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to delete: %v", err)
	}

	if strings.TrimSpace(branchList(dir)) != "" {
		t.Error("Branch still exists")
	}

	t.Log("✅ Branch deleted")
}

func TestBranch_Rename(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	exec.Command("git", "checkout", "-b", "old-name").Run()

	cmd := exec.Command("git", "branch", "-m", "old-name", "new-name")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to rename: %v", err)
	}

	list := branchList(dir)
	if strings.Contains(list, "old-name") {
		t.Error("Old name still exists")
	}
	if !strings.Contains(list, "new-name") {
		t.Error("New name not found")
	}

	t.Log("✅ Branch renamed")
}
