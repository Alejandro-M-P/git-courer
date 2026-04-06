package git_branches

import (
	"os/exec"
	"strings"
	"testing"
)

func TestTag_Create(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	cmd := exec.Command("git", "tag", "v1.0.0")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Tag create failed: %v", err)
	}

	cmd = exec.Command("git", "tag", "-l")
	cmd.Dir = dir
	output, _ := cmd.Output()
	if !strings.Contains(string(output), "v1.0.0") {
		t.Error("Tag not found")
	}

	t.Log("✅ Tag created")
}

func TestTag_Annotated(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	cmd := exec.Command("git", "tag", "-a", "v2.0.0", "-m", "Release version 2.0.0")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Annotated tag failed: %v", err)
	}

	cmd = exec.Command("git", "show", "v2.0.0")
	cmd.Dir = dir
	output, _ := cmd.Output()
	if !strings.Contains(string(output), "Release version 2.0.0") {
		t.Error("Annotated tag message not found")
	}

	t.Log("✅ Annotated tag created")
}

func TestTag_Delete(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	exec.Command("git", "tag", "to-delete").Run()

	cmd := exec.Command("git", "tag", "-d", "to-delete")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Tag delete failed: %v", err)
	}

	cmd = exec.Command("git", "tag", "-l")
	cmd.Dir = dir
	output, _ := cmd.Output()
	if strings.Contains(string(output), "to-delete") {
		t.Error("Tag still exists after delete")
	}

	t.Log("✅ Tag deleted")
}

func TestReflog_View(t *testing.T) {
	dir := setupGitRepoWithCommit(t)

	exec.Command("git", "checkout", "-b", "temp").Run()
	exec.Command("git", "checkout", "main").Run()

	cmd := exec.Command("git", "reflog")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Reflog failed: %v", err)
	}

	t.Logf("Reflog:\n%s", string(output))
}
