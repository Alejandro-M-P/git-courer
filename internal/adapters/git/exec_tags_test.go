package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestExecAdapterTag tests tag operations.
func TestExecAdapterTag(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	// Make initial commit first
	os.WriteFile(filepath.Join(dir, "initial.txt"), []byte("initial"), 0644)
	adapter := New(dir)
	adapter.Add([]string{"initial.txt"})
	adapter.Commit("Initial commit")

	// Create a lightweight tag (empty message)
	_, err := adapter.Tag("v1.0.0-test", "")
	if err != nil {
		t.Fatalf("Tag() error = %v", err)
	}

	// Verify tag exists
	exists, err := adapter.TagExists("v1.0.0-test")
	if err != nil {
		t.Fatalf("TagExists() error = %v", err)
	}
	if !exists {
		t.Error("TagExists() should return true for created tag")
	}

	// List tags
	tags, _ := adapter.ListTags()
	if len(tags) == 0 {
		t.Error("ListTags() should return created tag")
	}
}

// TestExecAdapterTagLightweight verifies Tag with empty message creates a lightweight tag.
func TestExecAdapterTagLightweight(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	os.WriteFile(filepath.Join(dir, "init.txt"), []byte("init"), 0644)
	adapter := New(dir)
	adapter.Add([]string{"init.txt"})
	adapter.Commit("Initial commit")

	_, err := adapter.Tag("v1.0.0-lw", "")
	if err != nil {
		t.Fatalf("Tag() error = %v", err)
	}

	// Verify lightweight tag (no 'tagger' line)
	out, err := exec.Command("git", "-C", dir, "show", "v1.0.0-lw").CombinedOutput()
	if err != nil {
		t.Fatalf("git show tag error = %v, output=%s", err, out)
	}
	if containsStr(string(out), "Tagger") {
		t.Error("lightweight tag should NOT contain 'Tagger' line")
	}
}

// TestExecAdapterTagAnnotated verifies Tag with non-empty message creates an annotated tag.
func TestExecAdapterTagAnnotated(t *testing.T) {
	dir := t.TempDir()
	initGitRepo(t, dir)

	os.WriteFile(filepath.Join(dir, "init.txt"), []byte("init"), 0644)
	adapter := New(dir)
	adapter.Add([]string{"init.txt"})
	adapter.Commit("Initial commit")

	message := "Release v2.0.0\n\nChangelog\n- feat: new stuff"
	_, err := adapter.Tag("v2.0.0-ann", message)
	if err != nil {
		t.Fatalf("Tag() error = %v", err)
	}

	out, err := exec.Command("git", "-C", dir, "show", "v2.0.0-ann").CombinedOutput()
	if err != nil {
		t.Fatalf("git show tag error = %v, output=%s", err, out)
	}
	if !containsStr(string(out), "Tagger") {
		t.Error("annotated tag should contain 'Tagger' line")
	}
	if !containsStr(string(out), "feat: new stuff") {
		t.Errorf("annotated tag should contain message body, got:\n%s", out)
	}
}
