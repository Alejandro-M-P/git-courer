package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestGitRepo creates a temporary git repo and returns its path.
func setupTestGitRepo(t *testing.T) string {
	t.Helper()
	repoDir := t.TempDir()

	for _, cmd := range []string{
		"git init",
		"git config user.email test@test",
		"git config user.name test",
	} {
		parts := strings.Fields(cmd)
		c := exec.Command(parts[0], parts[1:]...)
		c.Dir = repoDir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("%s: %v\n%s", cmd, err, out)
		}
	}
	return repoDir
}

// runGit runs a git command in the given directory and returns output.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return strings.TrimSpace(string(out))
}

// writeFile writes content to a file in the given directory.
func writeFile(t *testing.T, dir, filename, content string) {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestGitContentProvider_New(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	provider := NewGitContentProvider(repoDir)
	assert.NotNil(t, provider)
	assert.Equal(t, repoDir, provider.repoRoot)
}

func TestGitContentProvider_GetContents_FileInHEADAndIndex(t *testing.T) {
	repoDir := setupTestGitRepo(t)

	// Create file and commit
	writeFile(t, repoDir, "handler.go", "package main\nfunc Existing() {}\n")
	runGit(t, repoDir, "add", "handler.go")
	runGit(t, repoDir, "commit", "-m", "initial")

	// Modify and stage
	writeFile(t, repoDir, "handler.go", "package main\nfunc Existing() {}\nfunc NewFunc() {}\n")
	runGit(t, repoDir, "add", "handler.go")

	provider := NewGitContentProvider(repoDir)
	contents, err := provider.GetContents([]string{"handler.go"})
	require.NoError(t, err)
	require.Len(t, contents, 1)

	assert.NotNil(t, contents[0].Before, "Before should contain the committed version")
	assert.NotNil(t, contents[0].After, "After should contain the staged version")
	assert.Equal(t, "handler.go", contents[0].Filename)
	assert.Contains(t, string(contents[0].Before), "Existing")
	assert.Contains(t, string(contents[0].After), "NewFunc")
}

func TestGitContentProvider_GetContents_NewFile(t *testing.T) {
	repoDir := setupTestGitRepo(t)

	// Create file and stage it (no commit)
	writeFile(t, repoDir, "new.go", "package main\n")
	runGit(t, repoDir, "add", "new.go")

	provider := NewGitContentProvider(repoDir)
	contents, err := provider.GetContents([]string{"new.go"})
	require.NoError(t, err)
	require.Len(t, contents, 1)

	assert.Nil(t, contents[0].Before, "Before should be nil for a new file (not in HEAD)")
	assert.NotNil(t, contents[0].After, "After should contain the staged content")
	assert.Equal(t, "new.go", contents[0].Filename)
}

func TestGitContentProvider_GetContents_DeletedFile(t *testing.T) {
	repoDir := setupTestGitRepo(t)

	// Create file and commit it
	writeFile(t, repoDir, "toremove.go", "package main\n")
	runGit(t, repoDir, "add", "toremove.go")
	runGit(t, repoDir, "commit", "-m", "initial")

	// Delete and stage
	os.Remove(filepath.Join(repoDir, "toremove.go"))
	runGit(t, repoDir, "add", "toremove.go")

	provider := NewGitContentProvider(repoDir)
	contents, err := provider.GetContents([]string{"toremove.go"})
	require.NoError(t, err)
	require.Len(t, contents, 1)

	assert.NotNil(t, contents[0].Before, "Before should contain the committed version")
	assert.Nil(t, contents[0].After, "After should be nil for a deleted file")
	assert.Equal(t, "toremove.go", contents[0].Filename)
}

func TestGitContentProvider_GetContents_NonExistentFile(t *testing.T) {
	repoDir := setupTestGitRepo(t)

	provider := NewGitContentProvider(repoDir)
	_, err := provider.GetContents([]string{"nonexistent.go"})
	assert.Error(t, err, "should return error for non-existent file")
}

func TestGitContentProvider_GetContents_EmptyFileList(t *testing.T) {
	repoDir := setupTestGitRepo(t)
	provider := NewGitContentProvider(repoDir)

	contents, err := provider.GetContents([]string{})
	assert.NoError(t, err, "empty file list should not error")
	assert.Empty(t, contents, "should return empty slice for empty input")
}
