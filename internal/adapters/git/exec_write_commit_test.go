package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- WriteTree tests ---

func TestWriteTree_EmptyStaging_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	_, err := adapter.WriteTree()
	if err == nil {
		t.Fatal("WriteTree() should return error when staging area is empty")
	}
	if !strings.Contains(err.Error(), "nothing to commit") {
		t.Errorf("WriteTree() error = %q, want error containing 'nothing to commit'", err.Error())
	}
}

func TestWriteTree_WithStagedFile_ReturnsHash(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Create and stage a file
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)
	adapter := New(dir)
	adapter.Add([]string{"test.txt"})

	hash, err := adapter.WriteTree()
	if err != nil {
		t.Fatalf("WriteTree() error = %v", err)
	}
	if len(hash) != 40 {
		t.Errorf("WriteTree() hash length = %d, want 40 (got %q)", len(hash), hash)
	}
	// Verify it's a valid hex string
	for _, c := range hash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("WriteTree() hash contains non-hex char: %c in %q", c, hash)
			break
		}
	}
}

// --- CommitTree tests ---

func TestCommitTree_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Create initial commit (needed for parent)
	os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("initial content"), 0644)
	adapter.Add([]string{"initial.txt"})
	_, err := adapter.Commit("initial commit")
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	// Get HEAD as parent
	parentHash, err := adapter.Head()
	if err != nil {
		t.Fatalf("Head() error = %v", err)
	}

	// Stage a new file and get tree hash
	os.WriteFile(filepath.Join(dir, "feature.txt"), []byte("feature content"), 0644)
	adapter.Add([]string{"feature.txt"})
	treeHash, err := adapter.WriteTree()
	if err != nil {
		t.Fatalf("WriteTree() error = %v", err)
	}

	// Create a commit using CommitTree
	commitHash, err := adapter.CommitTree(treeHash, parentHash, "plumbing commit")
	if err != nil {
		t.Fatalf("CommitTree() error = %v", err)
	}
	if len(commitHash) != 40 {
		t.Errorf("CommitTree() hash length = %d, want 40 (got %q)", len(commitHash), commitHash)
	}

	// Verify the commit object exists and is valid
	cmd := exec.Command("git", "cat-file", "-t", commitHash)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cat-file -t %s failed: %v", commitHash, err)
	}
	if strings.TrimSpace(string(out)) != "commit" {
		t.Errorf("cat-file -t %s = %q, want 'commit'", commitHash, strings.TrimSpace(string(out)))
	}
}

// --- UpdateRef tests ---

func TestUpdateRef_MovesBranch(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Create initial commit
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)
	adapter.Add([]string{"file.txt"})
	_, err := adapter.Commit("initial commit")
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	// Get current HEAD hash
	headHash, err := adapter.Head()
	if err != nil {
		t.Fatalf("Head() error = %v", err)
	}

	// Create a ref pointing to the same commit
	_, err = adapter.UpdateRef("refs/heads/test-branch", headHash)
	if err != nil {
		t.Fatalf("UpdateRef() error = %v", err)
	}

	// Verify the ref points to the right commit
	cmd := exec.Command("git", "rev-parse", "refs/heads/test-branch")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse refs/heads/test-branch failed: %v", err)
	}
	resolved := strings.TrimSpace(string(out))
	if resolved != headHash {
		t.Errorf("UpdateRef: ref resolves to %q, want %q", resolved, headHash)
	}
}

// --- Head tests ---

func TestHead_ResolvesCommit(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Create initial commit
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0644)
	adapter.Add([]string{"file.txt"})
	_, err := adapter.Commit("initial commit")
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	// Get HEAD via our method
	headHash, err := adapter.Head()
	if err != nil {
		t.Fatalf("Head() error = %v", err)
	}
	if len(headHash) != 40 {
		t.Errorf("Head() hash length = %d, want 40 (got %q)", len(headHash), headHash)
	}

	// Verify it matches git rev-parse HEAD
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse HEAD failed: %v", err)
	}
	expected := strings.TrimSpace(string(out))
	if headHash != expected {
		t.Errorf("Head() = %q, want %q", headHash, expected)
	}
}

func TestHead_UnbornRepo_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	adapter := New(dir)

	// Fresh repo with no commits — HEAD is unborn
	_, err := adapter.Head()
	if err == nil {
		t.Fatal("Head() should return error on unborn repo (no commits)")
	}
}